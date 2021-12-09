package install

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli"
)

const (
	Name  = "install"
	Usage = "Installs docker-slim"
	Alias = "i"
)

const (
	FlagDockerCLIPlugin      = "docker-cli-plugin"
	FlagDockerCLIPluginUsage = "Install as Docker CLI plugin"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:   FlagDockerCLIPlugin,
			Usage:  FlagDockerCLIPluginUsage,
			EnvVar: "DSLIM_INSTALL_DOCKER_CLI_PLUGIN",
		},
	},
	Action: func(ctx *cli.Context) error {
		doDebug := ctx.GlobalBool(commands.FlagDebug)
		statePath := ctx.GlobalString(commands.FlagStatePath)
		inContainer, isDSImage := commands.IsInContainer(ctx.GlobalBool(commands.FlagInContainer))
		archiveState := commands.ArchiveState(ctx.GlobalString(commands.FlagArchiveState), inContainer)
		dockerCLIPlugin := ctx.Bool(FlagDockerCLIPlugin)

		OnCommand(doDebug, statePath, archiveState, inContainer, isDSImage, dockerCLIPlugin)
		return nil
	},
}
