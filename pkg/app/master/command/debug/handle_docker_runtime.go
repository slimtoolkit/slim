package debug

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/container"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
)

// HandleDockerRuntime implements support for the docker runtime
func HandleDockerRuntime(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	commandParams *CommandParams,
	client *dockerapi.Client,
	sid string,
	debugContainerName string) {

	if commandParams.ActionListDebuggableContainers {
		xc.Out.State("action.list_debuggable_containers")

		result, err := listDockerDebuggableContainers(client)
		if err != nil {
			logger.WithError(err).Error("listDockerDebuggableContainers")
			xc.FailOn(err)
		}

		xc.Out.Info("debuggable.containers", ovars{"count": len(result)})
		for cname, iname := range result {
			xc.Out.Info("debuggable.container", ovars{"name": cname, "image": iname})
		}

		return
	}

	//todo: need to check that if targetRef is not empty it is valid

	if commandParams.ActionListSessions {
		xc.Out.State("action.list_sessions", ovars{"target": commandParams.TargetRef})

		//later will track/show additional debug session info
		result, err := listDockerDebugContainers(client, commandParams.TargetRef, false)
		if err != nil {
			logger.WithError(err).Error("listDockerDebugContainers")
			xc.FailOn(err)
		}

		var waitingCount int
		var runningCount int
		var terminatedCount int
		for _, info := range result {
			switch info.State {
			case CSWaiting:
				waitingCount++
			case CSRunning:
				runningCount++
			case CSTerminated:
				terminatedCount++
			}
		}

		xc.Out.Info("debug.session.count",
			ovars{
				"total":      len(result),
				"running":    runningCount,
				"waiting":    waitingCount,
				"terminated": terminatedCount,
			})

		for name, info := range result {
			outParams := ovars{
				"target":     info.TargetContainerName,
				"name":       name,
				"image":      info.SpecImage,
				"state":      info.State,
				"start.time": info.StartTime,
			}

			/*
				if info.State == CSTerminated {
					outParams["exit.code"] = info.ExitCode
					outParams["finish.time"] = info.FinishTime
					if info.ExitReason != "" {
						outParams["exit.reason"] = info.ExitReason
					}
					if info.ExitMessage != "" {
						outParams["exit.message"] = info.ExitMessage
					}
				}
			*/

			xc.Out.Info("debug.session", outParams)
		}

		return
	}

	if commandParams.ActionShowSessionLogs {
		xc.Out.State("action.show_session_logs",
			ovars{
				"target":  commandParams.TargetRef,
				"session": commandParams.Session})

		result, err := listDockerDebugContainers(client, commandParams.TargetRef, false)
		if err != nil {
			logger.WithError(err).Error("listDockerDebugContainers")
			xc.FailOn(err)
		}

		if len(result) < 1 {
			xc.Out.Info("no.debug.session")
			return
		}

		//todo: need to pick the last session if commandParams.Session is empty
		var containerID string
		for _, info := range result {
			if commandParams.Session == "" {
				commandParams.Session = info.Name
			}

			if commandParams.Session == info.Name {
				containerID = info.ContainerID
			}
			break
		}

		xc.Out.Info("container.logs.target", ovars{
			"container.name": commandParams.Session,
			"container.id":   containerID})

		if err := dumpDockerContainerLogs(logger, xc, client, containerID); err != nil {
			logger.WithError(err).Error("dumpDockerContainerLogs")
		}

		return
	}

	if commandParams.ActionConnectSession {
		xc.Out.State("action.connect_session",
			ovars{
				"target":  commandParams.TargetRef,
				"session": commandParams.Session})

		result, err := listDockerDebugContainers(client, commandParams.TargetRef, true)
		if err != nil {
			logger.WithError(err).Error("listDockerDebugContainers")
			xc.FailOn(err)
		}

		if len(result) < 1 {
			xc.Out.Info("no.debug.session")
			return
		}

		//todo: need to pick the last session if commandParams.Session is empty
		var containerID string
		for _, info := range result {
			if commandParams.Session == "" {
				commandParams.Session = info.Name
			}

			if commandParams.Session == info.Name {
				containerID = info.ContainerID
			}
			break
		}

		//todo: need to validate that the session container exists and it's running

		r, w := io.Pipe()
		go io.Copy(w, os.Stdin)

		options := dockerapi.AttachToContainerOptions{
			Container:    containerID,
			InputStream:  r,
			OutputStream: os.Stdout,
			ErrorStream:  os.Stderr,
			Stdin:        true,
			Stdout:       true,
			Stderr:       true,
			Stream:       true,
			RawTerminal:  true,
			Logs:         true,
		}

		err = client.AttachToContainer(options)
		xc.FailOn(err)
		return
	}

	imageInspector, err := image.NewInspector(client, commandParams.DebugContainerImage)
	errutil.FailOn(err)
	noImage, err := imageInspector.NoImage()
	errutil.FailOn(err)
	if noImage {
		err := imageInspector.Pull(true, "", "", "")
		xc.FailOn(err)
	}

	targetContainerInfo, err := client.InspectContainer(commandParams.TargetRef)
	if err != nil {
		xc.Out.Error("target.container.inspect", err.Error())
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
	}

	if commandParams.DoRunAsTargetShell {
		logger.Trace("doRunAsTargetShell")
		commandParams.Entrypoint = ShellCommandPrefix(commandParams.DebugContainerImage)
		shellConfig := configShell(sid, false)
		if CgrSlimToolkitDebugImage == commandParams.DebugContainerImage {
			shellConfig = configShellAlt(sid, false)
		}

		commandParams.Cmd = []string{shellConfig}
	} else {
		if len(commandParams.Cmd) == 0 &&
			CgrSlimToolkitDebugImage == commandParams.DebugContainerImage {
			commandParams.Cmd = []string{bashShellName}
		}
	}

	options := container.ExecutionOptions{
		ContainerName: debugContainerName,
		Entrypoint:    commandParams.Entrypoint,
		Cmd:           commandParams.Cmd,
		Terminal:      commandParams.DoTerminal,
	}

	exe, err := container.NewExecution(
		xc,
		logger,
		client,
		commandParams.DebugContainerImage,
		&options,
		nil,
		true,
		true)

	// attach network, IPC & PIDs, essentially this is run --network container:golang_service --pid container:golang_service --ipc container:golang_service
	mode := fmt.Sprintf("container:%s", commandParams.TargetRef)

	if targetContainerInfo.HostConfig.IpcMode == "shareable" {
		exe.IpcMode = mode
	}

	exe.NetworkMode = mode
	exe.PidMode = mode

	xc.FailOn(err)

	err = exe.Start()
	xc.FailOn(err)

	_, err = exe.Wait()
	xc.FailOn(err)

	defer func() {
		err = exe.Cleanup()
		errutil.WarnOn(err)
	}()

	if !commandParams.DoTerminal {
		exe.ShowContainerLogs()
	}
}

func listDockerDebuggableContainers(client *dockerapi.Client) (map[string]string, error) {
	const op = "debug.listDockerDebuggableContainers"

	containers, err := dockerutil.ListContainers(client, "", false)
	if err != nil {
		log.WithFields(log.Fields{
			"op":               op,
			"containers.count": len(containers),
		}).Error("dockerutil.ListContainers")
		return nil, err
	}

	activeContainers := map[string]string{}
	for name, info := range containers {
		if info.State != dockerutil.CSRunning {
			log.WithFields(log.Fields{
				"op":        "listDockerDebuggableContainers",
				"container": name,
				"state":     info.State,
			}).Trace("ignoring.nonrunning.container")
			continue
		}

		if strings.HasPrefix(name, containerNamePrefix) {
			log.WithFields(log.Fields{
				"op":        "listDockerDebuggableContainers",
				"container": name,
			}).Trace("ignoring.debug.container")
			continue
		}

		activeContainers[name] = info.Image
	}

	return activeContainers, nil
}

func listDebuggableDockerContainersWithConfig(client *dockerapi.Client) (map[string]string, error) {
	//todo: pass the docker client config params instead of the existing client
	return listDockerDebuggableContainers(client)
}

func listDockerDebugContainers(
	client *dockerapi.Client,
	targetContainer string,
	onlyActive bool) (map[string]*DebugContainerInfo, error) {
	containers, err := dockerutil.ListContainers(client, "", true)
	if err != nil {
		return nil, err
	}

	result := map[string]*DebugContainerInfo{}
	for name, container := range containers {
		if !strings.HasPrefix(name, containerNamePrefix) {
			log.WithFields(log.Fields{
				"op":        "listDockerDebugContainers",
				"container": name,
			}).Trace("ignoring.nondebug.container")
			continue
		}

		//todo: filter by targetContainer (when info.TargetContainerName is populated)

		t := time.Unix(container.Created, 0) //UnixMilli, UnixMicro
		info := &DebugContainerInfo{
			//TargetContainerName: info.TargetContainerName,
			Name:        container.Name,
			SpecImage:   container.Image,
			ContainerID: container.ID,
			//Command:             info.Command,
			//Args:                info.Args,
			//WorkingDir:          info.WorkingDir,
			//TTY:                 info.TTY,
			StartTime: fmt.Sprintf("%v", t),
		}

		switch container.State {
		case dockerutil.CSCreated, dockerutil.CSRestarting, dockerutil.CSPaused:
			info.State = CSWaiting
		case dockerutil.CSRunning:
			info.State = CSRunning
		case dockerutil.CSRemoving, dockerutil.CSExited, dockerutil.CSDead:
			info.State = CSTerminated
		}

		if onlyActive {
			if info.State == CSRunning {
				result[info.Name] = info
			}
		} else {
			result[info.Name] = info
		}
	}

	return result, nil
}

func listDockerDebugContainersWithConfig(
	client *dockerapi.Client,
	targetContainer string,
	onlyActive bool) (map[string]*DebugContainerInfo, error) {
	//todo: pass the docker client config params instead of the existing client
	return listDockerDebugContainers(client, targetContainer, onlyActive)
}

func dumpDockerContainerLogs(
	logger *log.Entry,
	xc *app.ExecutionContext,
	client *dockerapi.Client,
	containerID string) error {
	logger.Tracef("dumpDockerContainerLogs(%s)", containerID)

	outData, errData, err := dockerutil.GetContainerLogs(client, containerID, true)
	if err != nil {
		logger.WithError(err).Error("error reading container logs")
		return err
	}

	xc.Out.Info("container.logs.start")
	xc.Out.LogDump("debug.container.logs.stdout", string(outData))
	xc.Out.LogDump("debug.container.logs.stderr", string(errData))
	xc.Out.Info("container.logs.end")
	return nil
}
