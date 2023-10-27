package control

type Command string

const (
	StopTargetAppCommand Command = "stop-target-app"
	WaitForEventCommand  Command = "wait-for-event"
)
