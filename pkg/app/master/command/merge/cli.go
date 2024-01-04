package merge

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

const (
	Name  = "merge"
	Usage = "Merge two container images (optimized to merge minified images)"
	Alias = "m"
)

//FUTURE/TODO: extend it to be a generic merge function not limited to minified images

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		cflag(FlagImage),
		cflag(FlagUseLastImageMetadata),
		cflag(FlagTag),
	},
	Action: func(ctx *cli.Context) error {
		gfvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gfvalues.QuietCLIMode,
			gfvalues.OutputFormat)

		if ctx.Args().Len() < 1 {
			xc.Out.Error("param.target", "missing target image ID/name")
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		cfvalues, err := CommandFlagValues(xc, ctx)
		if err != nil {
			//CommandFlagValues() outputs the error messages already
			return nil
		}

		OnCommand(
			xc,
			gfvalues,
			cfvalues)

		return nil
	},
}

type CommandParams struct {
	FirstImage           string   `json:"first_image"`
	LastImage            string   `json:"last_image"`
	UseLastImageMetadata bool     `json:"use_last_image_metadata"`
	OutputTags           []string `json:"output_tags"`
}

func CommandFlagValues(xc *app.ExecutionContext, ctx *cli.Context) (*CommandParams, error) {
	values := &CommandParams{
		UseLastImageMetadata: ctx.Bool(FlagUseLastImageMetadata),
		OutputTags:           ctx.StringSlice(FlagTag),
	}

	images := ctx.StringSlice(FlagImage)
	if len(images) > 0 {
		if len(images) < 2 {
			xc.Out.Error("param.image", "must have two image references")
			cli.ShowCommandHelp(ctx, Name)
			return nil, fmt.Errorf("must have two image references")
		}

		values.FirstImage = images[0]
		values.LastImage = images[1]
	}

	if ctx.Args().Len() > 0 {
		if ctx.Args().Len() < 2 {
			xc.Out.Error("param.image", "must have two image references")
			cli.ShowCommandHelp(ctx, Name)
			return nil, fmt.Errorf("must have two image references")
		}

		values.FirstImage = ctx.Args().Get(0)
		values.LastImage = ctx.Args().Get(1)
	}

	if values.FirstImage == "" || values.LastImage == "" {
		xc.Out.Error("param.image", "not enough image references")
		cli.ShowCommandHelp(ctx, Name)
		return nil, fmt.Errorf("not enough image references")
	}

	return values, nil
}
