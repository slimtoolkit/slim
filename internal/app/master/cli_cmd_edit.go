package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

var cmdEdit = cli.Command{
	Name:    cmdSpecs[CmdEdit].name,
	Aliases: []string{cmdSpecs[CmdEdit].alias},
	Usage:   cmdSpecs[CmdEdit].usage,
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		if len(ctx.Args()) < 1 {
			fmt.Printf("docker-slim[edit]: missing target info...\n\n")
			cli.ShowCommandHelp(ctx, CmdEdit)
			return nil
		}

		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		ec := &commands.ExecutionContext{}

		commands.OnEdit(
			gcvalues,
			targetRef,
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
