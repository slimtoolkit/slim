package install

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands"

	"github.com/urfave/cli/v2"
)

const (
	Name  = "install"
	Usage = "Installs slim"
	Alias = "in"
)

const (
	FlagBinDir      = "bin-dir"
	FlagBinDirUsage = "Install binaries to the standard user app bin directory (/usr/local/bin)"

	FlagDockerCLIPlugin      = "docker-cli-plugin"
	FlagDockerCLIPluginUsage = "Install as Docker CLI plugin"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    FlagBinDir,
			Usage:   FlagBinDirUsage,
			EnvVars: []string{"DSLIM_INSTALL_BIN_DIR"},
		},
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

		binDir := ctx.Bool(FlagBinDir)
		dockerCLIPlugin := ctx.Bool(FlagDockerCLIPlugin)

		OnCommand(doDebug, statePath, archiveState, inContainer, isDSImage, binDir, dockerCLIPlugin)
		return nil
	},
}
