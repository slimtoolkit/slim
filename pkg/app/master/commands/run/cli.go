package run

import (
	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/urfave/cli/v2"
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
		TargetRef:        ctx.String(commands.FlagTarget),
		DoPull:           ctx.Bool(commands.FlagPull),
		DockerConfigPath: ctx.String(commands.FlagDockerConfigPath),
		RegistryAccount:  ctx.String(commands.FlagRegistryAccount),
		RegistrySecret:   ctx.String(commands.FlagRegistrySecret),
		DoShowPullLogs:   ctx.Bool(commands.FlagShowPullLogs),
		DoLiveLogs:       ctx.Bool(FlagLiveLogs),
		DoTerminal:       ctx.Bool(FlagTerminal),
		EnvVars:          ctx.StringSlice(commands.FlagEnv),
		DoRemoveOnExit:   ctx.Bool(FlagRemove),
		DoDetach:         ctx.Bool(FlagDetach),
	}

	var err error
	if rawEntrypoint := ctx.String(commands.FlagEntrypoint); rawEntrypoint != "" {
		values.Entrypoint, err = commands.ParseExec(rawEntrypoint)
		if err != nil {
			return nil, err
		}
	}

	if rawCmd := ctx.String(commands.FlagCmd); rawCmd != "" {
		values.Cmd, err = commands.ParseExec(rawCmd)
		if err != nil {
			return nil, err
		}
	}

	if rawVolumes := ctx.StringSlice(commands.FlagVolume); len(rawVolumes) > 0 {
		values.Volumes, err = commands.ParseVolumeMountsAsList(rawVolumes)
		if err != nil {
			return nil, err
		}
	}

	if rawPorts := ctx.StringSlice(FlagPublishPort); len(rawPorts) > 0 {
		values.PublishPorts, err = commands.ParsePortBindings(rawPorts)
	}

	return values, nil
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		commands.Cflag(commands.FlagTarget),
		commands.Cflag(commands.FlagPull),
		commands.Cflag(commands.FlagDockerConfigPath),
		commands.Cflag(commands.FlagRegistryAccount),
		commands.Cflag(commands.FlagRegistrySecret),
		commands.Cflag(commands.FlagShowPullLogs),
		commands.Cflag(commands.FlagEntrypoint),
		commands.Cflag(commands.FlagCmd),
		cflag(FlagLiveLogs),
		cflag(FlagTerminal),
		cflag(FlagPublishPort),
		commands.Cflag(commands.FlagEnv),
		commands.Cflag(commands.FlagVolume),
		cflag(FlagRemove),
		cflag(FlagDetach),
	},
	Action: func(ctx *cli.Context) error {
		xc := app.NewExecutionContext(Name)

		gparams, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

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
