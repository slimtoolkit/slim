package debug

import (
	"fmt"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/container"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

type ovars = app.OutVars

// OnCommand implements the 'debug' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	commandParams *CommandParams) {
	logger := log.WithFields(log.Fields{"app": appName, "command": Name})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewDebugCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted

	xc.Out.State("started")
	xc.Out.Info("params",
		ovars{
			"target":          commandParams.TargetRef,
			"debug-image":     commandParams.DebugContainerImage,
			"debug-image-cmd": commandParams.DebugContainerImageCmd,
			"attach-tty":      commandParams.AttachTty,
		})

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := commands.ECTCommon | commands.ECNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(xc, Name, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	imageInspector, err := image.NewInspector(client, commandParams.DebugContainerImage)
	options := container.ExecutionOptions{
		Cmd:      commandParams.DebugContainerImageCmd,
		Terminal: commandParams.AttachTty,
	}
	if imageInspector.NoImage() {
		err := imageInspector.Pull(true, "", "", "")
		errutil.FailOn(err)
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
	exe.IpcMode = mode
	exe.NetworkMode = mode
	exe.PidMode = mode

	errutil.FailOn(err)

	err = exe.Start()
	errutil.FailOn(err)

	_, err = exe.Wait()
	errutil.FailOn(err)

	defer func() {
		err = exe.Cleanup()
		errutil.WarnOn(err)
	}()

	if !commandParams.AttachTty {
		exe.ShowContainerLogs()
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted
	xc.Out.State("done")

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}
