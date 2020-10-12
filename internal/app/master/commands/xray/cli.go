package xray

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

var (
	Name  = "xray"
	Usage = "Shows what's inside of your container image and reverse engineers its Dockerfile"
	Alias = "x"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		commands.Cflag(commands.FlagTarget),
		cflag(FlagChanges),
		cflag(FlagLayer),
		cflag(FlagAddImageManifest),
		cflag(FlagAddImageConfig),
		commands.Cflag(commands.FlagRemoveFileArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		targetRef := ctx.String(commands.FlagTarget)

		if targetRef == "" {
			if len(ctx.Args()) < 1 {
				fmt.Printf("docker-slim[%s]: missing image ID/name...\n\n", Name)
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		gcvalues, err := commands.GlobalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		changes, err := commands.ParseChangeTypes(ctx.StringSlice(FlagChanges))
		if err != nil {
			fmt.Printf("docker-slim[%s]: invalid change types: %v\n", Name, err)
			return err
		}

		layers, err := commands.ParseTokenSet(ctx.StringSlice(FlagLayer))
		if err != nil {
			fmt.Printf("docker-slim[%s]: invalid layer selectors: %v\n", Name, err)
			return err
		}

		doAddImageManifest := ctx.Bool(FlagAddImageManifest)
		doAddImageConfig := ctx.Bool(FlagAddImageConfig)
		doRmFileArtifacts := ctx.Bool(commands.FlagRemoveFileArtifacts)

		ec := &commands.ExecutionContext{}

		OnCommand(
			gcvalues,
			targetRef,
			changes,
			layers,
			doAddImageManifest,
			doAddImageConfig,
			doRmFileArtifacts,
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
