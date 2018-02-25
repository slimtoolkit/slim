package version

import (
	"fmt"

	"github.com/cloudimmunity/go-dockerclientx"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/cloudimmunity/system"
)

// Print shows the master app version information
func Print(client *docker.Client) {
	fmt.Println("docker-slim:")
	fmt.Println(v.Current())

	fmt.Println("host:")
	fmt.Printf("%#v\n", system.GetSystemInfo())

	fmt.Println("docker:")
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

	ver, err := client.Version()
	if err != nil {
		fmt.Println("error getting docker version")
		return
	}

	fmt.Printf("ApiVersion=%v\n", ver.Get("ApiVersion"))
	fmt.Printf("MinAPIVersion=%v\n", ver.Get("MinAPIVersion"))
	fmt.Printf("BuildTime=%v\n", ver.Get("BuildTime"))
	fmt.Printf("GitCommit=%v\n", ver.Get("GitCommit"))
}
