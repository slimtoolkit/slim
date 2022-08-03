package run

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Run command flag names
const (
	FlagLiveLogs    = "live-logs"
	FlagTerminal    = "terminal"
	FlagPublishPort = "publish"
	FlagRemove      = "rm"
	FlagDetach      = "detach"
)

// Run command flag usage info
const (
	FlagLiveLogsUsage    = "Show live logs for the container (cant use with --terminal)"
	FlagTerminalUsage    = "Attach interactive terminal to the container"
	FlagPublishPortUsage = "Map container port to host port (format => port | hostPort:containerPort | hostIP:hostPort:containerPort | hostIP::containerPort )"
	FlagRemoveUsage      = "Remove the container when it exits"
	FlagDetachUsage      = "Start the container and do not wait for it to exit"
)

var Flags = map[string]cli.Flag{
	FlagLiveLogs: &cli.BoolFlag{
		Name:    FlagLiveLogs,
		Usage:   FlagLiveLogsUsage,
		EnvVars: []string{"DSLIM_RUN_LIVE_LOGS"},
	},
	FlagTerminal: &cli.BoolFlag{
		Name:    FlagTerminal,
		Usage:   FlagTerminalUsage,
		EnvVars: []string{"DSLIM_RUN_TERMINAL"},
	},
	FlagPublishPort: &cli.StringSliceFlag{
		Name:    FlagPublishPort,
		Value:   &cli.StringSlice{},
		Usage:   FlagPublishPortUsage,
		EnvVars: []string{"DSLIM_RUN_PUBLISH_PORT"},
	},
	FlagRemove: &cli.BoolFlag{
		Name:    FlagRemove,
		Usage:   FlagRemoveUsage,
		EnvVars: []string{"DSLIM_RUN_RM"},
	},
	FlagDetach: &cli.BoolFlag{
		Name:    FlagDetach,
		Usage:   FlagDetachUsage,
		EnvVars: []string{"DSLIM_RUN_DETACH"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
