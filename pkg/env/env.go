package env

import (
	"os"
)

const (
	dockerEnvPath   = "/.dockerenv"
	dsImageFlagPath = "/.ds.container.d3e2c84f976743bdb92a7044ef12e381"
)

func HasDSImageFlag() bool {
	_, err := os.Stat(dsImageFlagPath)
	return err == nil
}

func HasDockerEnvPath() bool {
	_, err := os.Stat(dockerEnvPath)
	return err == nil
}

func HasContainerCgroups() bool {
	return false
}

func InContainer() (bool, bool) {
	isDSImage := HasDSImageFlag()
	if HasDockerEnvPath() {
		return true, isDSImage
	}

	if HasContainerCgroups() {
		return true, isDSImage
	}

	return false, isDSImage
}
