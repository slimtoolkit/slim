package command

// Command type constants
const (
	Build        Type = "build"
	Profile      Type = "profile"
	Xray         Type = "xray"
	Lint         Type = "lint"
	Containerize Type = "containerize"
	Convert      Type = "convert"
	Merge        Type = "merge"
	Edit         Type = "edit"
	Debug        Type = "debug"
	Probe        Type = "probe"
	Run          Type = "run"
	Server       Type = "server"
	Registry     Type = "registry"
	Version      Type = "version"
	Update       Type = "update"
)

// Type is the command type name
type Type string

// Command state constants
const (
	StateUnknown   = "unknown"
	StateError     = "error"
	StateStarted   = "started"
	StateCompleted = "completed"
	StateExited    = "exited"
	StateDone      = "done"
)

// State is the command state type
type State string
