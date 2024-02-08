package probe

import (
	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"

	"github.com/urfave/cli/v2"
)

//Standalone probing

const (
	Name  = "probe"
	Usage = "Probe target endpoint"
	Alias = "prb"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: append([]cli.Flag{
		cflag(FlagTarget),
		cflag(FlagPort),
	}, command.HTTPProbeFlagsBasic()...),
	Action: func(ctx *cli.Context) error {
		gparams, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
		if !ok || gparams == nil {
			return command.ErrNoGlobalParams
		}

		xc := app.NewExecutionContext(
			Name,
			gparams.QuietCLIMode,
			gparams.OutputFormat)

		targetEndpoint := ctx.String(FlagTarget)
		if targetEndpoint == "" {
			if ctx.Args().Len() < 1 {
				xc.Out.Error("param.target", "missing target")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetEndpoint = ctx.Args().First()
			}
		}

		httpProbeOpts := command.GetHTTPProbeOptions(xc, ctx, true)
		targetPorts := ctx.UintSlice(FlagPort)
		OnCommand(
			xc,
			gparams,
			targetEndpoint,
			targetPorts,
			httpProbeOpts)

		return nil
	},
}
