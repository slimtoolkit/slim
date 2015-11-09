package main

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
)

func failOnError(err error) {
	if err != nil {
		log.WithError(err).Fatal("docker-slim: failure")
	}
}

func warnOnError(err error) {
	if err != nil {
		log.WithError(err).Warn("docker-slim: warning")
	}
}

func failWhen(cond bool, msg string) {
	if cond {
		log.WithField("error", msg).Fatal("docker-slim: failure")
	}
}

func myFileDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	failOnError(err)
	return dirName
}

func myAppDirs() (string, string) {
	localVolumePath := filepath.Join(myFileDir(), "container")
	artifactLocation := filepath.Join(localVolumePath, "artifacts")
	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
		artifactDir, err = os.Stat(artifactLocation)
		failOnError(err)
	}
	failWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	return localVolumePath, artifactLocation
}

func removeArtifacts(artifactLocation string) error {
	return os.RemoveAll(artifactLocation)
}
