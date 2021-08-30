package edit

import (
	"fmt"

	"github.com/urfave/cli"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

const (
	Name  = "edit"
	Usage = "Edit container image"
	Alias = "e"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		if len(ctx.Args()) < 1 {
			fmt.Printf("docker-slim[%s]: missing target info...\n\n", Name)
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		xc := app.NewExecutionContext(Name)

		OnCommand(
			xc,
			gcvalues,
			targetRef)

		return nil
	},
}
