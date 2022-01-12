package registry

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Registry command flag names
const (
	FlagSaveToDocker = "save-to-docker"
)

// Registry command flag usage info
const (
	FlagSaveToDockerUsage = "Save pulled image to docker"
)

var Flags = map[string]cli.Flag{
	FlagSaveToDocker: &cli.BoolFlag{
		Name:    FlagSaveToDocker,
		Value:   true, //defaults to true
		Usage:   FlagSaveToDockerUsage,
		EnvVars: []string{"DSLIM_REG_PULL_SAVE_TO_DOCKER"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
