package probe

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Probe command flag names and usage descriptions
const (
	FlagTarget      = "target"
	FlagTargetUsage = "Target endpoint to probe"

	//for now just TCP ports (so no FlagProto for now)
	FlagPort      = "port"
	FlagPortUsage = "Endpoint port to probe"
)

var Flags = map[string]cli.Flag{
	FlagTarget: &cli.StringFlag{
		Name:    FlagTarget,
		Value:   "",
		Usage:   FlagTargetUsage,
		EnvVars: []string{"DSLIM_PRB_TARGET"},
	},
	FlagPort: &cli.UintSliceFlag{
		Name:    FlagPort,
		Value:   cli.NewUintSlice(),
		Usage:   FlagPortUsage,
		EnvVars: []string{"DSLIM_PRB_PORT"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
