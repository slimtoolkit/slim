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
