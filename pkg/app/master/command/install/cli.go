package install

import (
	"github.com/slimtoolkit/slim/pkg/app/master/command"

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
		doDebug := ctx.Bool(command.FlagDebug)
		statePath := ctx.String(command.FlagStatePath)
		inContainer, isDSImage := command.IsInContainer(ctx.Bool(command.FlagInContainer))
		archiveState := command.ArchiveState(ctx.String(command.FlagArchiveState), inContainer)

		binDir := ctx.Bool(FlagBinDir)
		dockerCLIPlugin := ctx.Bool(FlagDockerCLIPlugin)

		OnCommand(doDebug, statePath, archiveState, inContainer, isDSImage, binDir, dockerCLIPlugin)
		return nil
	},
}
