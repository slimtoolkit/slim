package version

import (
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

const (
	Name  = "version"
	Usage = "Shows slim and docker version information"
	Alias = "v"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		doDebug := ctx.Bool(command.FlagDebug)
		inContainer, isDSImage := command.IsInContainer(ctx.Bool(command.FlagInContainer))
		clientConfig := command.GetDockerClientConfig(ctx)

		xc := app.NewExecutionContext(Name, ctx.String(command.FlagConsoleFormat))

		OnCommand(xc,
			doDebug,
			inContainer,
			isDSImage,
			clientConfig)

		//app.ShowCommunityInfo()
		return nil
	},
}
