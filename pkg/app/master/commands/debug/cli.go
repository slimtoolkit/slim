package debug

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

//Debug container

const (
	Name  = "debug"
	Usage = "Debug the target container image from a debug container"
	Alias = "dbg"

	FlagDebugImage    = "debug-image"
	FlagDebugImageCmd = "debug-image-cmd"

	DefaultDebugImage    = "nicolaka/netshoot"
	DefaultDebugImageCmd = "/bin/bash"
)

type CommandParams struct {
	/// the running container which we want to attach to
	TargetRef string
	/// the name/id of the container image used for debugging
	DebugContainerImage string
	/// CMD used for launching the debugging image
	DebugContainerImageCmd string
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        FlagDebugImage,
			DefaultText: DefaultDebugImage,
			Required:    false,
		},
		&cli.StringFlag{
			Name:        FlagDebugImageCmd,
			DefaultText: DefaultDebugImageCmd,
			Required:    false,
		},
	},
	Action: func(ctx *cli.Context) error {
		if ctx.Args().Len() < 1 {
			fmt.Printf("docker-slim[%s]: missing target info...\n\n", Name)
			cli.ShowCommandHelp(ctx, Name)
			return nil
		}

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		commandParams := &CommandParams{
			TargetRef:              ctx.String(commands.FlagTarget),
			DebugContainerImage:    ctx.String(FlagDebugImage),
			DebugContainerImageCmd: ctx.String(FlagDebugImageCmd),
		}

		xc := app.NewExecutionContext(Name)

		if commandParams.TargetRef == "" {
			if ctx.Args().Len() < 1 {
				xc.Out.Error("param.target", "missing target")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				commandParams.TargetRef = ctx.Args().First()
			}
		}

		if commandParams.DebugContainerImage == "" {
			commandParams.DebugContainerImage = DefaultDebugImage
		}
		if commandParams.DebugContainerImageCmd == "" {
			commandParams.DebugContainerImageCmd = DefaultDebugImageCmd
		}

		OnCommand(
			xc,
			gcvalues,
			commandParams)

		return nil
	},
}
