package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

var cmdXray = cli.Command{
	Name:    cmdSpecs[CmdXray].name,
	Aliases: []string{cmdSpecs[CmdXray].alias},
	Usage:   cmdSpecs[CmdXray].usage,
	Flags: []cli.Flag{
		cflag(FlagTarget),
		cli.StringSliceFlag{
			Name:   FlagChanges,
			Value:  &cli.StringSlice{""},
			Usage:  FlagChangesUsage,
			EnvVar: "DSLIM_CHANGES",
		},
		cli.StringSliceFlag{
			Name:   FlagLayer,
			Value:  &cli.StringSlice{},
			Usage:  FlagLayerUsage,
			EnvVar: "DSLIM_LAYER",
		},
		cli.BoolFlag{
			Name:   FlagAddImageManifest,
			Usage:  FlagAddImageManifestUsage,
			EnvVar: "DSLIM_XRAY_IMAGE_MANIFEST",
		},
		cli.BoolFlag{
			Name:   FlagAddImageConfig,
			Usage:  FlagAddImageConfigUsage,
			EnvVar: "DSLIM_XRAY_IMAGE_CONFIG",
		},
		cflag(FlagRemoveFileArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		targetRef := ctx.String(FlagTarget)

		if targetRef == "" {
			if len(ctx.Args()) < 1 {
				fmt.Printf("docker-slim[xray]: missing image ID/name...\n\n")
				cli.ShowCommandHelp(ctx, CmdXray)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		changes, err := parseChangeTypes(ctx.StringSlice(FlagChanges))
		if err != nil {
			fmt.Printf("docker-slim[xray]: invalid change types: %v\n", err)
			return err
		}

		layers, err := parseTokenSet(ctx.StringSlice(FlagLayer))
		if err != nil {
			fmt.Printf("docker-slim[xray]: invalid layer selectors: %v\n", err)
			return err
		}

		doAddImageManifest := ctx.Bool(FlagAddImageManifest)
		doAddImageConfig := ctx.Bool(FlagAddImageConfig)
		doRmFileArtifacts := ctx.Bool(FlagRemoveFileArtifacts)

		ec := &commands.ExecutionContext{}

		commands.OnXray(
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
