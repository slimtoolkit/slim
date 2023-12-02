package server

import (
	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"

	"github.com/urfave/cli/v2"
)

const (
	Name  = "server"
	Usage = "Run as an HTTP server"
	Alias = "s"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		gcvalues, err := command.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		xc := app.NewExecutionContext(Name, ctx.String(command.FlagConsoleFormat))

		OnCommand(
			xc,
			gcvalues)

		return nil
	},
}
