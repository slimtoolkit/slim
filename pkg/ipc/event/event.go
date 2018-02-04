package event

import (
	"errors"
)

// Event errors
var (
	ErrUnknownEvent = errors.New("unknown event type")
)

// Name is an event ID type
type Name string

// Supported events
const (
	StopMonitorDoneName    Name = "event.monitor.stop.done"
	ShutdownSensorDoneName Name = "event.sensor.shutdown.done"
)
