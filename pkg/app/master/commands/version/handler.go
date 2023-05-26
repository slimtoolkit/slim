package version

import (
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	//"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"
)

type ovars = app.OutVars

// OnCommand implements the 'version' command
func OnCommand(
	xc *app.ExecutionContext,
	doDebug, inContainer, isDSImage bool,
	clientConfig *config.DockerClient) {
	logger := log.WithFields(log.Fields{"app": "slim", "command": command.Version})

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
