package commands

import (
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
)

// OnVersion implements the 'version' docker-slim command
func OnVersion(doDebug, inContainer, isDSImage bool, clientConfig *config.DockerClient) {
	client := dockerclient.New(clientConfig)
	version.Print(client, true, inContainer, isDSImage)
}
