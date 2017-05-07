package utils

import (
	"runtime/debug"

	log "github.com/Sirupsen/logrus"
)

func FailOn(err error) {
	if err != nil {
		stackData := debug.Stack()
		log.WithError(err).WithField("version", CurrentVersion()).WithField("stack", string(stackData)).Fatal("docker-slim: failure")
	}
}

func WarnOn(err error) {
	if err != nil {
		stackData := debug.Stack()
		log.WithError(err).WithField("version", CurrentVersion()).WithField("stack", string(stackData)).Warn("docker-slim: warning")
	}
}

func FailWhen(cond bool, msg string) {
	if cond {
		stackData := debug.Stack()
		log.WithFields(log.Fields{
			"version": CurrentVersion(),
			"error":   msg,
			"stack":   string(stackData),
		}).Fatal("docker-slim: failure")
	}
}

func Fail(msg string) {
	stackData := debug.Stack()
	log.WithFields(log.Fields{
		"version": CurrentVersion(),
		"error":   msg,
		"stack":   string(stackData),
	}).Fatal("docker-slim: failure")
}
