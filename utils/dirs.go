package utils

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/pdiscover"
)

func ExeDir() string {
	exePath, err := pdiscover.GetOwnProcPath()
	FailOn(err)
	return filepath.Dir(exePath)
}

func FileDir(fileName string) string {
	dirName, err := filepath.Abs(filepath.Dir(fileName))
	FailOn(err)
	return dirName
}

func PrepareSlimDirs(imageId string) (string, string) {
	//images IDs in Docker 1.9+ are prefixed with a hash type...
	if strings.Contains(imageId, ":") {
		parts := strings.Split(imageId, ":")
		imageId = parts[1]
	}

	localVolumePath := filepath.Join(ExeDir(), ".images", imageId)
	artifactLocation := filepath.Join(localVolumePath, "artifacts")
	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
		artifactDir, err = os.Stat(artifactLocation)
		FailOn(err)
		log.Debug("created artifact directory: ", artifactDir)
	}
	FailWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	return localVolumePath, artifactLocation
}
