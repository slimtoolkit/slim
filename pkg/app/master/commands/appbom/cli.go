package appbom

import (
	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli/v2"
)

const (
	Name  = "appbom"
	Usage = "Show application BOM"
	Alias = "a"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		xc := app.NewExecutionContext(Name, ctx.String(commands.FlagConsoleFormat))

		OnCommand(
			xc,
			gcvalues)

		return nil
	},
}
