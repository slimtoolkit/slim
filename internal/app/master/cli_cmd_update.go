package app

import (
	"fmt"
	"runtime"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

func initFlagShowProgress() cli.Flag {
	//enable 'show-progress' by default only on Mac OS X
	var doShowProgressFlag cli.Flag
	switch runtime.GOOS {
	case "darwin":
		doShowProgressFlag = cli.BoolTFlag{
			Name:   FlagShowProgress,
			Usage:  fmt.Sprintf("%s (default: true)", FlagShowProgressUsage),
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	default:
		doShowProgressFlag = cli.BoolFlag{
			Name:   FlagShowProgress,
			Usage:  fmt.Sprintf("%s (default: false)", FlagShowProgressUsage),
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	}

	return doShowProgressFlag
}

var cmdUpdate = cli.Command{
	Name:    cmdSpecs[CmdUpdate].name,
	Aliases: []string{cmdSpecs[CmdUpdate].alias},
	Usage:   cmdSpecs[CmdUpdate].usage,
	Flags: []cli.Flag{
		initFlagShowProgress(),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		doDebug := ctx.GlobalBool(FlagDebug)
		statePath := ctx.GlobalString(FlagStatePath)
		inContainer, isDSImage := isInContainer(ctx.GlobalBool(FlagInContainer))
		archiveState := archiveState(ctx.GlobalString(FlagArchiveState), inContainer)
		doShowProgress := ctx.Bool(FlagShowProgress)

		commands.OnUpdate(doDebug, statePath, archiveState, inContainer, isDSImage, doShowProgress)
		commands.ShowCommunityInfo()
		return nil
	},
}
