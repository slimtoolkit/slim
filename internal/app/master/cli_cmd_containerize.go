package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

var cmdContainerize = cli.Command{
	Name:    cmdSpecs[CmdContainerize].name,
	Aliases: []string{cmdSpecs[CmdContainerize].alias},
	Usage:   cmdSpecs[CmdContainerize].usage,
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		if len(ctx.Args()) < 1 {
			fmt.Printf("docker-slim[containerize]: missing target info...\n\n")
			cli.ShowCommandHelp(ctx, CmdContainerize)
			return nil
		}

		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		ec := &commands.ExecutionContext{}

		commands.OnContainerize(
			gcvalues,
			targetRef,
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
