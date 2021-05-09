package probe

import (
	"fmt"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli"
)

//Standalone probing

const (
	Name  = "probe"
	Usage = "Probe target endpoint"
	Alias = "prb"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		if len(ctx.Args()) < 1 {
			fmt.Printf("docker-slim[%s]: missing target info...\n\n", Name)
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		gcvalues, err := commands.GlobalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		xc := commands.NewExecutionContext(Name)

		OnCommand(
			xc,
			gcvalues,
			targetRef)

		commands.ShowCommunityInfo()
		return nil
	},
}
