package utils

import (
	"os"
)

func RemoveArtifacts(artifactLocation string) error {
	return os.RemoveAll(artifactLocation)
}
