package debug

import (
	"fmt"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/container"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
)

// HandleDockerRuntime implements support for the docker runtime
func HandleDockerRuntime(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	commandParams *CommandParams,
	client *dockerapi.Client) {
	imageInspector, err := image.NewInspector(client, commandParams.DebugContainerImage)
	if imageInspector.NoImage() {
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

	options := container.ExecutionOptions{
		Entrypoint: commandParams.Entrypoint,
		Cmd:        commandParams.Cmd,
		Terminal:   commandParams.DoTerminal,
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
