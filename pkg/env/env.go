package env

import (
	"os"
)

const (
	dockerEnvPath = "/.dockerenv"
)

func HasDockerEnvPath() bool {
	_, err := os.Stat(dockerEnvPath)
	return err == nil
}

func HasContainerCgroups() bool {
	return false
}

func InContainer() bool {
	if HasDockerEnvPath() {
		return true
	}

	if HasContainerCgroups() {
		return true
	}

	return false
}
