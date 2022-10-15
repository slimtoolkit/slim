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

	PullCmdName        = "pull"
	PullCmdNameUsage   = "Pull a container image from registry"
	PushCmdName        = "push"
	PushCmdNameUsage   = "Push a container image to a registry"
	CopyCmdName        = "copy"
	CopyCmdNameUsage   = "Copy a container image from one registry to another"
	ServerCmdName      = "server"
	ServerCmdNameUsage = "Start a registry server"
)

func fullCmdName(subCmdName string) string {
	return fmt.Sprintf("%s.%s", Name, subCmdName)
}

type PullCommandParams struct {
	TargetRef    string
	SaveToDocker bool
}

func PullCommandFlagValues(ctx *cli.Context) (*PullCommandParams, error) {
	values := &PullCommandParams{
		TargetRef:    ctx.String(commands.FlagTarget),
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
				commands.Cflag(commands.FlagTarget),
				cflag(FlagSaveToDocker),
			},
			Action: func(ctx *cli.Context) error {
				xc := app.NewExecutionContext(fullCmdName(PullCmdName), ctx.String(commands.FlagConsoleFormat))

				gcvalues, err := commands.GlobalFlagValues(ctx)
				if err != nil {
					return err
				}

				cparams, err := PullCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				if cparams.TargetRef == "" {
					if ctx.Args().Len() < 1 {
						xc.Out.Error("param.target", "missing target")
						cli.ShowCommandHelp(ctx, Name)
						return nil
					} else {
						cparams.TargetRef = ctx.Args().First()
					}
				}

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

				xc := app.NewExecutionContext(fullCmdName(PushCmdName), ctx.String(commands.FlagConsoleFormat))
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

				xc := app.NewExecutionContext(fullCmdName(CopyCmdName), ctx.String(commands.FlagConsoleFormat))
				OnCopyCommand(xc, gcvalues)
				return nil
			},
		},
		{
			Name:  ServerCmdName,
			Usage: ServerCmdNameUsage,
			Action: func(ctx *cli.Context) error {
				gcvalues, err := commands.GlobalFlagValues(ctx)
				if err != nil {
					return err
				}

				xc := app.NewExecutionContext(fullCmdName(ServerCmdName), ctx.String(commands.FlagConsoleFormat))
				OnServerCommand(xc, gcvalues)
				return nil
			},
		},
	},
}
