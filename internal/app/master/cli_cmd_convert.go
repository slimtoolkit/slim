package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

var cmdConvert = cli.Command{
	Name:    cmdSpecs[CmdConvert].name,
	Aliases: []string{cmdSpecs[CmdConvert].alias},
	Usage:   cmdSpecs[CmdConvert].usage,
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		if len(ctx.Args()) < 1 {
			fmt.Printf("docker-slim[convert]: missing target info...\n\n")
			cli.ShowCommandHelp(ctx, CmdConvert)
			return nil
		}

		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		targetRef := ctx.Args().First()

		ec := &commands.ExecutionContext{}

		commands.OnConvert(
			gcvalues,
			targetRef,
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
