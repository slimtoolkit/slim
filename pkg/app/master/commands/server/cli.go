package server

import (
	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

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
		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		xc := app.NewExecutionContext(Name)

		OnCommand(
			xc,
			gcvalues)

		return nil
	},
}
