package registry

import (
	"fmt"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli/v2"
)

const (
	Name  = "registry"
	Usage = "Execute registry operations"
	Alias = "r"

	PullCmdName      = "pull"
	PullCmdNameUsage = "Pull a container image from registry"
	PushCmdName      = "push"
	PushCmdNameUsage = "Push a container image to a registry"
	CopyCmdName      = "copy"
	CopyCmdNameUsage = "Copy a container image from one registry to another"
)

func fullCmdName(subCmdName string) string {
	return fmt.Sprintf("%s.%s", Name, subCmdName)
}

type PullCommandParams struct {
	SaveToDocker bool
}

func PullCommandFlagValues(ctx *cli.Context) (*PullCommandParams, error) {
	values := &PullCommandParams{
		SaveToDocker: ctx.Bool(FlagSaveToDocker),
	}

	return values, nil
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Subcommands: []*cli.Command{
		{
			Name:  PullCmdName,
			Usage: PullCmdNameUsage,
			Flags: []cli.Flag{
				cflag(FlagSaveToDocker),
			},
			Action: func(ctx *cli.Context) error {
				gcvalues, err := commands.GlobalFlagValues(ctx)
				if err != nil {
					return err
				}

				cparams, err := PullCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				xc := app.NewExecutionContext(fullCmdName(PullCmdName))
				OnPullCommand(xc, gcvalues, cparams)
				return nil
			},
		},
		{
			Name:  PushCmdName,
			Usage: PushCmdNameUsage,
			Action: func(ctx *cli.Context) error {
				gcvalues, err := commands.GlobalFlagValues(ctx)
				if err != nil {
					return err
				}

				xc := app.NewExecutionContext(fullCmdName(PushCmdName))
				OnPushCommand(xc, gcvalues)
				return nil
			},
		},
		{
			Name:  CopyCmdName,
			Usage: CopyCmdNameUsage,
			Action: func(ctx *cli.Context) error {
				gcvalues, err := commands.GlobalFlagValues(ctx)
				if err != nil {
					return err
				}

				xc := app.NewExecutionContext(fullCmdName(CopyCmdName))
				OnCopyCommand(xc, gcvalues)
				return nil
			},
		},
	},
}
