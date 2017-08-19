package commands

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"

	"github.com/docker-slim/docker-slim/pkg/version"
)

// OnVersion implements the 'version' docker-slim command
func OnVersion(clientConfig *config.DockerClient) {
	fmt.Println("docker-slim:")
	fmt.Println(version.Current())

	fmt.Println("docker:")

	client := dockerclient.New(clientConfig)

	info, err := client.Info()
	if err != nil {
		fmt.Println("error getting docker info")
		return
	}

	fmt.Printf("Name=%v\n", info.Name)
	fmt.Printf("KernelVersion=%v\n", info.KernelVersion)
	fmt.Printf("OperatingSystem=%v\n", info.OperatingSystem)
	fmt.Printf("OSType=%v\n", info.OSType)
	fmt.Printf("ServerVersion=%v\n", info.ServerVersion)
	fmt.Printf("Architecture=%v\n", info.Architecture)

	version, err := client.Version()
	if err != nil {
		fmt.Println("error getting docker version")
		return
	}

	fmt.Printf("ApiVersion=%v\n", version.Get("ApiVersion"))
	fmt.Printf("MinAPIVersion=%v\n", version.Get("MinAPIVersion"))
	fmt.Printf("BuildTime=%v\n", version.Get("BuildTime"))
	fmt.Printf("GitCommit=%v\n", version.Get("GitCommit"))
}
