package registry

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

const (
	Name  = "registry"
	Usage = "Execute registry operations"
	Alias = "reg"

	PullCmdName      = "pull"
	PullCmdNameUsage = "Pull a container image from registry"

	PushCmdName      = "push"
	PushCmdNameUsage = "Push a container image to a registry"

	CopyCmdName      = "copy"
	CopyCmdNameUsage = "Copy a container image from one registry to another"

	ImageIndexCreateCmdName      = "image-index-create"
	ImageIndexCreateCmdNameUsage = "Create an image index (aka manifest list) with the referenced images (already in the target registry)"

	ServerCmdName      = "server"
	ServerCmdNameUsage = "Start a registry server"
)

func fullCmdName(subCmdName string) string {
	return fmt.Sprintf("%s.%s", Name, subCmdName)
}

type CommonCommandParams struct {
	UseDockerCreds bool
	CredsAccount   string
	CredsSecret    string
}

func CommonCommandFlagValues(ctx *cli.Context) (*CommonCommandParams, error) {
	values := &CommonCommandParams{
		UseDockerCreds: ctx.Bool(FlagUseDockerCreds),
		CredsAccount:   ctx.String(FlagCredsAccount),
		//prefer env var for secret (todo: add interactive and file read modes)
		CredsSecret: ctx.String(FlagCredsSecret),
	}

	return values, nil
}

type PullCommandParams struct {
	*CommonCommandParams
	TargetRef    string
	SaveToDocker bool
}

func PullCommandFlagValues(ctx *cli.Context) (*PullCommandParams, error) {
	common, err := CommonCommandFlagValues(ctx)
	if err != nil {
		return nil, err
	}

	values := &PullCommandParams{
		CommonCommandParams: common,
		TargetRef:           ctx.String(command.FlagTarget),
		SaveToDocker:        ctx.Bool(FlagSaveToDocker),
	}

	return values, nil
}

type PushCommandParams struct {
	*CommonCommandParams
	TargetRef  string
	TargetType string
	AsTag      string
}

const (
	ttDocker = "tt.docker"
	ttTar    = "tt.tar"
	ttOCI    = "tt.oci"
)

func PushCommandFlagValues(ctx *cli.Context) (*PushCommandParams, error) {
	common, err := CommonCommandFlagValues(ctx)
	if err != nil {
		return nil, err
	}

	values := &PushCommandParams{
		CommonCommandParams: common,
		AsTag:               ctx.String(FlagAs),
	}

	if val := ctx.String(FlagDocker); val != "" {
		//todo: validate that this local docker image exists
		values.TargetRef = val
		values.TargetType = ttDocker
	} else if val := ctx.String(FlagTar); val != "" {
		//todo: validate that this local tar file exists
		values.TargetRef = val
		values.TargetType = ttTar
	} else if val := ctx.String(FlagOCI); val != "" {
		//todo: validate that this local directory exists
		values.TargetRef = val
		values.TargetType = ttOCI
	}

	return values, nil
}

type ImageIndexCreateCommandParams struct {
	*CommonCommandParams
	ImageIndexName  string
	ImageNames      []string
	AsManifestList  bool
	InsecureRefs    bool
	DumpRawManifest bool
}

func ImageIndexCreateCommandFlagValues(ctx *cli.Context) (*ImageIndexCreateCommandParams, error) {
	common, err := CommonCommandFlagValues(ctx)
	if err != nil {
		return nil, err
	}

	values := &ImageIndexCreateCommandParams{
		CommonCommandParams: common,
		ImageIndexName:      ctx.String(FlagImageIndexName),
		ImageNames:          ctx.StringSlice(FlagImageName),
		AsManifestList:      ctx.Bool(FlagAsManifestList),
		InsecureRefs:        ctx.Bool(FlagInsecureRefs),
		DumpRawManifest:     ctx.Bool(FlagDumpRawManifest),
	}

	return values, nil
}

type ServerCommandParams struct {
	*CommonCommandParams
	Domain       string
	Address      string
	Port         uint
	UseHTTPS     bool
	CertPath     string
	KeyPath      string
	ReferrersAPI bool
	StorePath    string
	UseMemStore  bool
}

func ServerCommandFlagValues(ctx *cli.Context) (*ServerCommandParams, error) {
	common, err := CommonCommandFlagValues(ctx)
	if err != nil {
		return nil, err
	}

	values := &ServerCommandParams{
		CommonCommandParams: common,
		Domain:              ctx.String(FlagDomain),
		Address:             ctx.String(FlagAddress),
		Port:                ctx.Uint(FlagPort),
		UseHTTPS:            ctx.Bool(FlagHTTPS),
		CertPath:            ctx.String(FlagCertPath),
		KeyPath:             ctx.String(FlagKeyPath),
		ReferrersAPI:        ctx.Bool(FlagReferrersAPI),
		StorePath:           ctx.String(FlagStorePath),
		UseMemStore:         ctx.Bool(FlagMemStore),
	}

	return values, nil
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		cflag(FlagUseDockerCreds),
		cflag(FlagCredsAccount),
		cflag(FlagCredsSecret),
	},
	Subcommands: []*cli.Command{
		{
			Name:  PullCmdName,
			Usage: PullCmdNameUsage,
			Flags: []cli.Flag{
				command.Cflag(command.FlagTarget),
				cflag(FlagSaveToDocker),
			},
			Action: func(ctx *cli.Context) error {
				gcvalues := command.GlobalFlagValues(ctx)
				xc := app.NewExecutionContext(
					fullCmdName(PullCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

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
			Flags: []cli.Flag{
				cflag(FlagAs),
				cflag(FlagDocker),
				cflag(FlagTar),
				cflag(FlagOCI),
			},
			Action: func(ctx *cli.Context) error {
				gcvalues := command.GlobalFlagValues(ctx)
				xc := app.NewExecutionContext(
					fullCmdName(PushCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				cparams, err := PushCommandFlagValues(ctx)
				if err != nil {
					xc.Out.Error("params", err.Error())
					return err
				}

				if cparams.TargetType == "" {
					if ctx.Args().Len() < 1 {
						xc.Out.Error("params.target", "missing pull target")
						return fmt.Errorf("no target selected")
					}

					cparams.TargetRef = ctx.Args().First()
					cparams.TargetType = ttDocker
				}

				OnPushCommand(xc, gcvalues, cparams)
				return nil
			},
		},
		{
			Name:  CopyCmdName,
			Usage: CopyCmdNameUsage,
			Action: func(ctx *cli.Context) error {
				gcvalues := command.GlobalFlagValues(ctx)
				xc := app.NewExecutionContext(
					fullCmdName(CopyCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				OnCopyCommand(xc, gcvalues)
				return nil
			},
		},
		{
			Name:  ImageIndexCreateCmdName,
			Usage: ImageIndexCreateCmdNameUsage,
			Flags: []cli.Flag{
				cflag(FlagImageIndexName),
				cflag(FlagImageName),
				cflag(FlagAsManifestList),
				cflag(FlagInsecureRefs),
				cflag(FlagDumpRawManifest),
			},
			Action: func(ctx *cli.Context) error {
				gcvalues := command.GlobalFlagValues(ctx)
				xc := app.NewExecutionContext(
					fullCmdName(ImageIndexCreateCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				cparams, err := ImageIndexCreateCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				OnImageIndexCreateCommand(xc, gcvalues, cparams)
				return nil
			},
		},
		{
			Name:  ServerCmdName,
			Usage: ServerCmdNameUsage,
			Flags: []cli.Flag{
				cflag(FlagDomain),
				cflag(FlagAddress),
				cflag(FlagPort),
				cflag(FlagHTTPS),
				cflag(FlagCertPath),
				cflag(FlagKeyPath),
				cflag(FlagReferrersAPI),
			},
			Action: func(ctx *cli.Context) error {
				gcvalues := command.GlobalFlagValues(ctx)
				xc := app.NewExecutionContext(
					fullCmdName(ServerCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				cparams, err := ServerCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				OnServerCommand(xc, gcvalues, cparams)
				return nil
			},
		},
	},
}
