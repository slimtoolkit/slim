package version

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"
)

// OnCommand implements the 'version' docker-slim command
func OnCommand(doDebug, inContainer, isDSImage bool, clientConfig *config.DockerClient) {
	logger := log.WithFields(log.Fields{"app": "docker-slim", "command": command.Version})

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("cmd=%s info=docker.connect.error message='%s'\n", Name, exitMsg)
		fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", Name, v.Current(), fsutil.ExeDir())
		commands.Exit(-777)
	}
	errutil.FailOn(err)

	version.Print(fmt.Sprintf("cmd=%s", Name), logger, client, true, inContainer, isDSImage)
}
