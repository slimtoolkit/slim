package install

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli/v2"
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

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagDockerCLIPlugin,
			Usage:   FlagDockerCLIPluginUsage,
			EnvVars: []string{"DSLIM_INSTALL_DOCKER_CLI_PLUGIN"},
		},
	},
	Action: func(ctx *cli.Context) error {
		doDebug := ctx.Bool(commands.FlagDebug)
		statePath := ctx.String(commands.FlagStatePath)
		inContainer, isDSImage := commands.IsInContainer(ctx.Bool(commands.FlagInContainer))
		archiveState := commands.ArchiveState(ctx.String(commands.FlagArchiveState), inContainer)
		dockerCLIPlugin := ctx.Bool(FlagDockerCLIPlugin)

		OnCommand(doDebug, statePath, archiveState, inContainer, isDSImage, dockerCLIPlugin)
		return nil
	},
}
