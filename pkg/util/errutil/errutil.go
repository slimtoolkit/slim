package errutil

import (
	"fmt"
	"runtime/debug"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/consts"
	"github.com/docker-slim/docker-slim/pkg/version"
)

// FailOnWithInfo logs the error information with additional context info and terminates the application if there's an error
func FailOnWithInfo(err error, info map[string]string) {
	if err != nil {
		showInfo(info)

		stackData := debug.Stack()
		log.WithError(err).WithField("version", version.Current()).WithField("stack", string(stackData)).Fatal("docker-slim: failure")

		showCommunityInfo()
	}
}

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

func showInfo(info map[string]string) {
	if len(info) > 0 {
		fmt.Println("Error Context Info:")
		for k, v := range info {
			fmt.Printf("'%s': '%s'\n", k, v)
		}
	}
}

func showCommunityInfo() {
	fmt.Printf("docker-slim: message='join the Gitter channel to get help with this failure' info='%s'\n", consts.CommunityGitter)
	fmt.Printf("docker-slim: message='join the Discord server to get help with this failure' info='%s'\n", consts.CommunityDiscord)
	fmt.Printf("docker-slim: message='Github discussions' info='%s'\n", consts.CommunityDiscussions)
}

// exec.Command().Run() and its derivatives sometimes return
// "wait: no child processes" or "waitid: no child processes"
// even for successful runs. It's a race condition between the
// Start() + Wait() calls and the actual underlying command
// execution. The shorter the execution time, the higher are
// the chances to get this error.
//
// Some examples from the wild:
//  - https://github.com/gitpod-io/gitpod/blob/405d44b74b5ac1dffe20e076d59c2b5986f18960/components/common-go/process/process.go#L18.
func IsNoChildProcesses(err error) bool {
	if err == nil {
		return false
	}

	return strings.HasSuffix(err.Error(), ": no child processes")
}
