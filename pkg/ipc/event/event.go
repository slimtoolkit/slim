package event

import (
	"encoding/json"
	goerr "errors"

	"github.com/slimtoolkit/slim/pkg/errors"
)

// Event errors
var (
	ErrUnknownEvent    = goerr.New("unknown event type")
	ErrUnexpectedEvent = goerr.New("unexpected event type")
)

// Type is an event ID type
type Type string

// Supported events
const (
	StartMonitorDone   Type = "event.monitor.start.done"
	StartMonitorFailed Type = "event.monitor.start.failed"
	StopMonitorDone    Type = "event.monitor.stop.done"
	ShutdownSensorDone Type = "event.sensor.shutdown.done"
	Error              Type = "event.error"
)

type Message struct {
	Name Type        `json:"name"`
	Data interface{} `json:"data,omitempty"`
}

// StartMonitorFailed contains additional context for the failure
type StartMonitorFailedData struct {
	Component string            `json:"component,omitempty"`
	State     string            `json:"state,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
	Errors    []string          `json:"errors,omitempty"`
}

const (
	StateSensorTypeCreating    = "sensor.type.creating"
	StateCmdStartMonCmdWaiting = "cmd.monitor.start.waiting"
	StateEnvPreparing          = "env.preparing"
	StateMonCreating           = "monitor.creating"
	StateMonStarting           = "monitor.starting"
)

const (
	ComSensorConstructor = "sensor.constructor"
	ComSensorCmdServer   = "sensor.cmd.server"
	ComMonitorRunner     = "monitor.runner"
)

const (
	CtxSensorType = "sensor.type"
	CtxCmdType    = "cmd.type"
)

func (m *Message) UnmarshalJSON(data []byte) error {
	var tmp struct {
		Name Type            `json:"name"`
		Data json.RawMessage `json:"data,omitempty"`
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	m.Name = tmp.Name
	switch tmp.Name {
	case Error:
		var data errors.SensorError
		if err := json.Unmarshal(tmp.Data, &data); err != nil {
			return err
		}

		m.Data = &data
	case StartMonitorFailed:
		var data StartMonitorFailedData
		if err := json.Unmarshal(tmp.Data, &data); err != nil {
			return err
		}

		m.Data = &data
	default:
		if len(tmp.Data) > 0 {
			return json.Unmarshal(tmp.Data, &m.Data)
		}
	}

	return nil
}
