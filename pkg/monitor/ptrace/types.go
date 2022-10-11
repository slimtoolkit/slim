package ptrace

import (
	"io"
)

type AppRunOpt struct {
	Cmd                 string
	Args                []string
	AppStdout           io.Writer
	AppStderr           io.Writer
	WorkDir             string
	User                string
	RunAsUser           bool
	RTASourcePT         bool
	ReportOnMainPidExit bool
}
