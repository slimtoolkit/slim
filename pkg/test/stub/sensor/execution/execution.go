package execution

import (
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/sensor/execution"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
)

type executionStub struct {
	commands chan command.Message
}

var _ execution.Interface = &executionStub{}

func NewExecution() *executionStub {
	return &executionStub{
		commands: make(chan command.Message),
	}
}

func (e *executionStub) State() string {
	return ""
}

func (e *executionStub) Commands() <-chan command.Message {
	return e.commands
}

func (e *executionStub) SendCommand(cmd command.Message) {
	e.commands <- cmd
}

func (e *executionStub) PubEvent(etype event.Type, data ...interface{}) {
	log.
		WithField("type", etype).
		WithField("data", data).
		Debug("execution stub - new event")
}

func (e *executionStub) Close() {
	close(e.commands)
}

func (e *executionStub) HookSensorPostStart() {
	// noop
}

func (e *executionStub) HookSensorPreShutdown() {
	// noop
}

func (e *executionStub) HookMonitorPreStart() {
	// noop
}

func (e *executionStub) HookTargetAppRunning() {
	// noop
}

func (e *executionStub) HookMonitorPostShutdown() {
	// noop
}

func (e *executionStub) HookMonitorFailed() {
	// noop
}
