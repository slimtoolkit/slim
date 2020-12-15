package errutil

import (
	"fmt"
	"runtime/debug"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/consts"
	"github.com/docker-slim/docker-slim/pkg/version"
)

// FailOn logs the error information and terminates the application if there's an error
func FailOn(err error) {
	if err != nil {
		stackData := debug.Stack()
		log.WithError(err).WithField("version", version.Current()).WithField("stack", string(stackData)).Fatal("docker-slim: failure")

		showCommunityInfo()
	}
}

// WarnOn logs the error information as a warning
func WarnOn(err error) {
	if err != nil {
		stackData := debug.Stack()
		log.WithError(err).WithField("version", version.Current()).WithField("stack", string(stackData)).Warn("docker-slim: warning")
	}
}

// FailWhen logs the given message if the condition is true (terminates the application)
func FailWhen(cond bool, msg string) {
	if cond {
		stackData := debug.Stack()
		log.WithFields(log.Fields{
			"version": version.Current(),
			"error":   msg,
			"stack":   string(stackData),
		}).Fatal("docker-slim: failure")

		showCommunityInfo()
	}
}

// Fail logs the given messages and terminates the application
func Fail(msg string) {
	stackData := debug.Stack()
	log.WithFields(log.Fields{
		"version": version.Current(),
		"error":   msg,
		"stack":   string(stackData),
	}).Fatal("docker-slim: failure")

	showCommunityInfo()
}

func showCommunityInfo() {
	fmt.Printf("docker-slim: message='join the Gitter channel to get help with this failure' info='%s'\n", consts.CommunityGitter)
	fmt.Printf("docker-slim: message='join the Discord server to get help with this failure' info='%s'\n", consts.CommunityDiscord)
	fmt.Printf("docker-slim: message='Github discussions' info='%s'\n", consts.CommunityDiscussions)
}
