package update

import (
	"fmt"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

const (
	Name  = "update"
	Usage = "Updates slim"
	Alias = "u"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		initFlagShowProgress(),
	},
	Action: func(ctx *cli.Context) error {
		doDebug := ctx.Bool(command.FlagDebug)
		statePath := ctx.String(command.FlagStatePath)
		inContainer, isDSImage := command.IsInContainer(ctx.Bool(command.FlagInContainer))
		archiveState := command.ArchiveState(ctx.String(command.FlagArchiveState), inContainer)
		doShowProgress := ctx.Bool(command.FlagShowProgress)

		OnCommand(doDebug, statePath, archiveState, inContainer, isDSImage, doShowProgress)
		return nil
	},
}

func initFlagShowProgress() cli.Flag {
	//enable 'show-progress' by default only on Mac OS X
	var doShowProgressFlag cli.Flag
	switch runtime.GOOS {
	case "darwin":
		doShowProgressFlag = &cli.BoolFlag{
			Name:    command.FlagShowProgress,
			Value:   true,
			Usage:   fmt.Sprintf("%s (default: true)", command.FlagShowProgressUsage),
			EnvVars: []string{"DSLIM_UPDATE_SHOW_PROGRESS"},
		}
	default:
		doShowProgressFlag = &cli.BoolFlag{
			Name:    command.FlagShowProgress,
			Usage:   fmt.Sprintf("%s (default: false)", command.FlagShowProgressUsage),
			EnvVars: []string{"DSLIM_UPDATE_SHOW_PROGRESS"},
		}
	}

	return doShowProgressFlag
}
