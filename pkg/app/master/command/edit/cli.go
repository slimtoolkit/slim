package edit

import (
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

const (
	Name  = "edit"
	Usage = "Edit container image"
	Alias = "e"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		targetRef := ctx.String(command.FlagTarget)
		if targetRef == "" {
			if ctx.Args().Len() < 1 {
				xc.Out.Error("param.target", "missing target image ID/name")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		OnCommand(
			xc,
			gcvalues,
			targetRef)

		return nil
	},
}
