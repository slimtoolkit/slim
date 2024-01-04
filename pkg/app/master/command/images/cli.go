package images

import (
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	cmd "github.com/slimtoolkit/slim/pkg/command"
)

const (
	Name  = string(cmd.Images)
	Usage = "Get information about container images"
	Alias = "i"
)

//todo soon: add a lot of useful filtering flags
// (to show new images from last hour, to show images in use, by size, with details, etc)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		OnCommand(
			xc,
			gcvalues)

		return nil
	},
}
