package sensor

import (
	mastercommand "github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
)

type StartMonitorOpt func(*command.StartMonitor)

func WithSaneDefaults() StartMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.RTASourcePT = true
		cmd.KeepPerms = true
		cmd.IncludeCertAll = true
		cmd.IncludeCertBundles = true
		cmd.IncludeCertDirs = true
		cmd.IncludeCertPKAll = true
		cmd.IncludeCertPKDirs = true
		cmd.IncludeNew = true
	}
}

func WithAppNameArgs(name string, arg ...string) StartMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.AppName = name
		cmd.AppArgs = arg
	}
}

func WithAppUser(user string) StartMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.AppUser = user
		cmd.RunTargetAsUser = true
	}
}

func WithAppStdoutToFile() StartMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.AppStdoutToFile = true
	}
}

func WithAppStderrToFile() StartMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.AppStderrToFile = true
	}
}

func WithPreserves(path ...string) StartMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.Preserves = mastercommand.ParsePaths(path)
	}
}

func NewMonitorStartCommand(opts ...StartMonitorOpt) command.StartMonitor {
	cmd := command.StartMonitor{}

	for _, opt := range opts {
		opt(&cmd)
	}

	return cmd
}
