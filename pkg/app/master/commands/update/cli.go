package update

import (
	"fmt"
	"runtime"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	"github.com/urfave/cli"
)

const (
	Name  = "update"
	Usage = "Updates docker-slim"
	Alias = "u"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		initFlagShowProgress(),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		doDebug := ctx.GlobalBool(commands.FlagDebug)
		statePath := ctx.GlobalString(commands.FlagStatePath)
		inContainer, isDSImage := commands.IsInContainer(ctx.GlobalBool(commands.FlagInContainer))
		archiveState := commands.ArchiveState(ctx.GlobalString(commands.FlagArchiveState), inContainer)
		doShowProgress := ctx.Bool(commands.FlagShowProgress)

		OnCommand(doDebug, statePath, archiveState, inContainer, isDSImage, doShowProgress)
		commands.ShowCommunityInfo()
		return nil
	},
}

func initFlagShowProgress() cli.Flag {
	//enable 'show-progress' by default only on Mac OS X
	var doShowProgressFlag cli.Flag
	switch runtime.GOOS {
	case "darwin":
		doShowProgressFlag = cli.BoolTFlag{
			Name:   commands.FlagShowProgress,
			Usage:  fmt.Sprintf("%s (default: true)", commands.FlagShowProgressUsage),
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	default:
		doShowProgressFlag = cli.BoolFlag{
			Name:   commands.FlagShowProgress,
			Usage:  fmt.Sprintf("%s (default: false)", commands.FlagShowProgressUsage),
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	}

	return doShowProgressFlag
}
