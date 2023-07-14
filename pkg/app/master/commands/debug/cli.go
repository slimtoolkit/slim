package debug

import (
	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

//Debug container

const (
	Name  = "debug"
	Usage = "Debug the target container from a debug (side-car) container"
	Alias = "dbg"
)

const (
	DockerRuntime     = "docker"
	KubernetesRuntime = "k8s"
)

type CommandParams struct {
	/// the runtime environment type
	Runtime string
	/// the running container which we want to attach to
	TargetRef string
	/// the target namespace (k8s runtime)
	TargetNamespace string
	/// the target pod (k8s runtime)
	TargetPod string
	/// the name/id of the container image used for debugging
	DebugContainerImage string
	/// ENTRYPOINT used for launching the debugging image
	Entrypoint []string
	/// CMD used for launching the debugging image
	Cmd []string
	/// launch the debug container with an interactive terminal attached (like '--it' in docker)
	DoTerminal bool
	/// Kubeconfig file path (k8s runtime)
	Kubeconfig string
}

var debugImages = map[string]string{
	NicolakaNetshootImage: "Network trouble-shooting swiss-army container - https://github.com/nicolaka/netshoot",
	KoolkitsNodeImage:     "Node.js KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/nodejs",
	KoolkitsPythonImage:   "Python KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/python",
	KoolkitsGolangImage:   "Go KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/golang",
	KoolkitsJVMImage:      "JVM KoolKit - https://github.com/lightrun-platform/koolkits/blob/main/jvm/README.md",
	DigitaloceanDoksImage: "Kubernetes manifests for investigation and troubleshooting - https://github.com/digitalocean/doks-debug",
	ZinclabsUbuntuImage:   "Common utilities for debugging your cluster - https://github.com/openobserve/debug-container",
	BusyboxImage:          "A lightweight image with common unix utilities - https://busybox.net/about.html",
	WolfiBaseImage:        "A lightweight Wolfi base image - https://github.com/chainguard-images/images/tree/main/images/wolfi-base",
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		cflag(FlagRuntime),
		cflag(FlagTarget),
		cflag(FlagNamespace),
		cflag(FlagPod),
		cflag(FlagDebugImage),
		cflag(FlagEntrypoint),
		cflag(FlagCmd),
		cflag(FlagTerminal),
		cflag(FlagListDebugImage),
		cflag(FlagKubeconfig),
	},
	Action: func(ctx *cli.Context) error {
		xc := app.NewExecutionContext(Name, ctx.String(commands.FlagConsoleFormat))

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		if ctx.Bool(FlagListDebugImage) {
			for k, v := range debugImages {
				xc.Out.Info("debug.image", ovars{"name": k, "description": v})
			}

			return nil
		}

		commandParams := &CommandParams{
			Runtime:             ctx.String(FlagRuntime),
			TargetRef:           ctx.String(FlagTarget),
			TargetNamespace:     ctx.String(FlagNamespace),
			TargetPod:           ctx.String(FlagPod),
			DebugContainerImage: ctx.String(FlagDebugImage),
			DoTerminal:          ctx.Bool(FlagTerminal),
			Kubeconfig:          ctx.String(FlagKubeconfig),
		}

		if rawEntrypoint := ctx.String(FlagEntrypoint); rawEntrypoint != "" {
			commandParams.Entrypoint, err = commands.ParseExec(rawEntrypoint)
			if err != nil {
				return err
			}
		}

		if rawCmd := ctx.String(FlagCmd); rawCmd != "" {
			commandParams.Cmd, err = commands.ParseExec(rawCmd)
			if err != nil {
				return err
			}
		}

		if commandParams.TargetRef == "" {
			if ctx.Args().Len() < 1 {
				if commandParams.Runtime != KubernetesRuntime {
					xc.Out.Error("param.target", "missing target")
					cli.ShowCommandHelp(ctx, Name)
					return nil
				}
				//NOTE:
				//It's ok to not specify the target container for k8s
				//We'll pick the default or first container in the target pod
			} else {
				commandParams.TargetRef = ctx.Args().First()
				if ctx.Args().Len() > 1 && ctx.Args().Slice()[1] == "--" {
					//NOTE:
					//Keep the original 'no terminal' behavior
					//use this shortcut mode as a way to quickly
					//run one off commands in the debugged container
					//When there's 'no terminal' we show
					//the debugger container log at the end.
					//TODO: revisit the behavior later...
					commandParams.DoTerminal = false
					commandParams.Cmd = ctx.Args().Slice()[2:]
				}
			}
		}

		if commandParams.DebugContainerImage == "" {
			commandParams.DebugContainerImage = NicolakaNetshootImage
		}

		OnCommand(
			xc,
			gcvalues,
			commandParams)

		return nil
	},
}
