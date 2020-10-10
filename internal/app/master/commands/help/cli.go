package help

import (
	"github.com/urfave/cli"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
)

var (
	Name  = "help"
	Usage = "Show help info"
	Alias = "h"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		cli.ShowAppHelp(ctx)
		return nil
	},
}

func init() {
	commands.CLI = append(commands.CLI, CLI)
}
