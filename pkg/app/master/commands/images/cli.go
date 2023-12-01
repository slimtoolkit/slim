package images

import (
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/commands"
	"github.com/slimtoolkit/slim/pkg/command"
)

const (
	Name  = string(command.Images)
	Usage = "Get information about container images"
	Alias = "i"
)

//todo soon: add a lot of useful filtering flags
// (to show new images from last hour, to show images in use, by size, with details, etc)

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
