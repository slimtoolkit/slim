package utils

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
)

func ExeDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	FailOn(err)
	return dirName
}

func FileDir(fileName string) string {
	dirName, err := filepath.Abs(filepath.Dir(fileName))
	FailOn(err)
	return dirName
}

func PrepareSlimDirs() (string, string) {
	localVolumePath := filepath.Join(ExeDir(), "container")
	artifactLocation := filepath.Join(localVolumePath, "artifacts")
	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
		artifactDir, err = os.Stat(artifactLocation)
		FailOn(err)
		log.Debug("created artifact directory: ",artifactDir)
	}
	FailWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	return localVolumePath, artifactLocation
}
