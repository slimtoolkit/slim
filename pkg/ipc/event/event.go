package event

import (
	"errors"
)

// Event errors
var (
	ErrUnknownEvent    = errors.New("unknown event type")
	ErrUnexpectedEvent = errors.New("unexpected event type")
)

// Name is an event ID type
type Name string

// Supported events
const (
	StartMonitorDoneName   Name = "event.monitor.start.done"
	StartMonitorFailedName Name = "event.monitor.start.failed"
	StopMonitorDoneName    Name = "event.monitor.stop.done"
	ShutdownSensorDoneName Name = "event.sensor.shutdown.done"
)
