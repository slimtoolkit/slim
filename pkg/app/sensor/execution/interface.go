package execution

import (
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
)

type Interface interface {
	State() string
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
