package utils

import (
	"os"
)

// RemoveArtifacts removes the artifacts generated during the current application execution
func RemoveArtifacts(artifactLocation string) error {
	return os.RemoveAll(artifactLocation)
}
