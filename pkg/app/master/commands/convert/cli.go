package convert

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/commands"
)

const (
	Name  = "convert"
	Usage = "Convert container image"
	Alias = "k"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		if ctx.Args().Len() < 1 {
			fmt.Printf("slim[%s]: missing target info...\n\n", Name)
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		xc := app.NewExecutionContext(Name, ctx.String(commands.FlagConsoleFormat))

		OnCommand(
			xc,
			gcvalues,
			targetRef)

		return nil
	},
}
