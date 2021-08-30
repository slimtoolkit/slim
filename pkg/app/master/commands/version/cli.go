package version

import (
	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli"
)

const (
	Name  = "version"
	Usage = "Shows docker-slim and docker version information"
	Alias = "v"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		doDebug := ctx.GlobalBool(commands.FlagDebug)
		inContainer, isDSImage := commands.IsInContainer(ctx.GlobalBool(commands.FlagInContainer))
		clientConfig := commands.GetDockerClientConfig(ctx)

		xc := app.NewExecutionContext(Name)

		OnCommand(xc,
			doDebug,
			inContainer,
			isDSImage,
			clientConfig)

		//app.ShowCommunityInfo()
		return nil
	},
}
