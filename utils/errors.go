package utils

import (
	log "github.com/Sirupsen/logrus"
)

func FailOn(err error) {
	if err != nil {
		log.WithError(err).Fatal("docker-slim: failure")
	}
}

func WarnOn(err error) {
	if err != nil {
		log.WithError(err).Warn("docker-slim: warning")
	}
}

func FailWhen(cond bool, msg string) {
	if cond {
		log.WithField("error", msg).Fatal("docker-slim: failure")
	}
}
