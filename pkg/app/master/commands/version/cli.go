package version

import (
	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli/v2"
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
		doDebug := ctx.Bool(commands.FlagDebug)
		inContainer, isDSImage := commands.IsInContainer(ctx.Bool(commands.FlagInContainer))
		clientConfig := commands.GetDockerClientConfig(ctx)

		xc := app.NewExecutionContext(Name, ctx.String(commands.FlagConsoleFormat))

		OnCommand(xc,
			doDebug,
			inContainer,
			isDSImage,
			clientConfig)

		//app.ShowCommunityInfo()
		return nil
	},
}
