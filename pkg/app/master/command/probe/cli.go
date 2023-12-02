package probe

import (
	"fmt"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"

	"github.com/urfave/cli/v2"
)

//Standalone probing

const (
	Name  = "probe"
	Usage = "Probe target endpoint"
	Alias = "prb"
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

		gcvalues, err := command.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		xc := app.NewExecutionContext(Name, ctx.String(command.FlagConsoleFormat))

		OnCommand(
			xc,
			gcvalues,
			targetRef)

		return nil
	},
}
