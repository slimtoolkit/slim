package ptrace

type AppRunOpt struct {
	Cmd                 string
	Args                []string
	WorkDir             string
	User                string
	RunAsUser           bool
	RTASourcePT         bool
	ReportOnMainPidExit bool
}
