package commands

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"
)

// OnVersion implements the 'version' docker-slim command
func OnVersion(doDebug, inContainer, isDSImage bool, clientConfig *config.DockerClient) {
	logger := log.WithFields(log.Fields{"app": "docker-slim", "command": command.Version})

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("docker-slim[version]: info=docker.connect.error message='%s'\n", exitMsg)
		fmt.Printf("docker-slim[version]: state=exited version=%s location='%s'\n", v.Current(), fsutil.ExeDir())
		os.Exit(-777)
	}
	errutil.FailOn(err)

	version.Print("docker-slim[version]:", logger, client, true, inContainer, isDSImage)
}
