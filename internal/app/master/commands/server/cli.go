package server

import (
	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

const (
	Name  = "server"
	Usage = "Run as an HTTP server"
	Alias = "s"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()

		gcvalues, err := commands.GlobalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		ec := &commands.ExecutionContext{}

		OnCommand(
			gcvalues,
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
