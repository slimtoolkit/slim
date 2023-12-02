package version

import (
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	//"github.com/slimtoolkit/slim/pkg/app/master/commands"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

type ovars = app.OutVars

// OnCommand implements the 'version' command
func OnCommand(
	xc *app.ExecutionContext,
	doDebug, inContainer, isDSImage bool,
	clientConfig *config.DockerClient) {
	logger := log.WithFields(log.Fields{"app": "slim", "cmd": cmd.Version})

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := -777
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	version.Print(xc, Name, logger, client, true, inContainer, isDSImage)
}
