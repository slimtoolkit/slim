package run

import (
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

const (
	Name  = "run"
	Usage = "Run one or more containers"
	Alias = "r"
)

type CommandParams struct {
	TargetRef        string
	DoPull           bool
	DockerConfigPath string
	RegistryAccount  string
	RegistrySecret   string
	DoShowPullLogs   bool
	Entrypoint       []string
	Cmd              []string
	DoLiveLogs       bool
	DoTerminal       bool
	PublishPorts     map[dockerapi.Port][]dockerapi.PortBinding
	EnvVars          []string
	Volumes          []config.VolumeMount
	DoRemoveOnExit   bool
	DoDetach         bool
}

func CommandFlagValues(ctx *cli.Context) (*CommandParams, error) {
	values := &CommandParams{
		TargetRef:        ctx.String(command.FlagTarget),
		DoPull:           ctx.Bool(command.FlagPull),
		DockerConfigPath: ctx.String(command.FlagDockerConfigPath),
		RegistryAccount:  ctx.String(command.FlagRegistryAccount),
		RegistrySecret:   ctx.String(command.FlagRegistrySecret),
		DoShowPullLogs:   ctx.Bool(command.FlagShowPullLogs),
		DoLiveLogs:       ctx.Bool(FlagLiveLogs),
		DoTerminal:       ctx.Bool(FlagTerminal),
		EnvVars:          ctx.StringSlice(command.FlagEnv),
		DoRemoveOnExit:   ctx.Bool(FlagRemove),
		DoDetach:         ctx.Bool(FlagDetach),
	}

	var err error
	if rawEntrypoint := ctx.String(command.FlagEntrypoint); rawEntrypoint != "" {
		values.Entrypoint, err = command.ParseExec(rawEntrypoint)
		if err != nil {
			return nil, err
		}
	}

	if rawCmd := ctx.String(command.FlagCmd); rawCmd != "" {
		values.Cmd, err = command.ParseExec(rawCmd)
		if err != nil {
			return nil, err
		}
	}

	if rawVolumes := ctx.StringSlice(command.FlagVolume); len(rawVolumes) > 0 {
		values.Volumes, err = command.ParseVolumeMountsAsList(rawVolumes)
		if err != nil {
			return nil, err
		}
	}

	if rawPorts := ctx.StringSlice(FlagPublishPort); len(rawPorts) > 0 {
		values.PublishPorts, err = command.ParsePortBindings(rawPorts)
	}

	return values, nil
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		command.Cflag(command.FlagTarget),
		command.Cflag(command.FlagPull),
		command.Cflag(command.FlagDockerConfigPath),
		command.Cflag(command.FlagRegistryAccount),
		command.Cflag(command.FlagRegistrySecret),
		command.Cflag(command.FlagShowPullLogs),
		command.Cflag(command.FlagEntrypoint),
		command.Cflag(command.FlagCmd),
		cflag(FlagLiveLogs),
		cflag(FlagTerminal),
		cflag(FlagPublishPort),
		command.Cflag(command.FlagEnv),
		command.Cflag(command.FlagVolume),
		cflag(FlagRemove),
		cflag(FlagDetach),
	},
	Action: func(ctx *cli.Context) error {
		gparams := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gparams.QuietCLIMode,
			gparams.OutputFormat)

		cparams, err := CommandFlagValues(ctx)
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

		OnCommand(
			xc,
			gparams,
			cparams)

		return nil
	},
}
