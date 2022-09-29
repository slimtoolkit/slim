package sensor

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
)

type startMonitorOpt func(*command.StartMonitor)

func WithSaneDefaults() startMonitorOpt {
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

func WithAppNameArgs(name string, arg ...string) startMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.AppName = name
		cmd.AppArgs = arg
	}
}

func WithPreserves(path ...string) startMonitorOpt {
	return func(cmd *command.StartMonitor) {
		cmd.Preserves = commands.ParsePaths(path)
	}
}

func NewMonitorStartCommand(opts ...startMonitorOpt) command.StartMonitor {
	cmd := command.StartMonitor{}

	for _, opt := range opts {
		opt(&cmd)
	}

	return cmd
}
