package execution

import (
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
)

type Interface interface {
	Commands() <-chan command.Message
	PubEvent(etype event.Type, data ...interface{})
	Close()

	// Lifecycle hooks (extension points)
	HookSensorPostStart()
	HookSensorPreShutdown()
	HookMonitorPreStart()
	HookTargetAppRunning()
	HookMonitorPostShutdown()
	HookMonitorFailed()
}
