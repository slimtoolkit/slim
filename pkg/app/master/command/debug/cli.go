package debug

import (
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
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
	KubeconfigDefault = "${HOME}/.kube/config"
	NamespaceDefault  = "default"
)

type NVPair struct {
	Name  string
	Value string
}

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
	/// ENTRYPOINT used launching the debugging container
	Entrypoint []string
	/// CMD used launching the debugging container
	Cmd []string
	/// WORKDIR used launching the debugging container
	Workdir string
	/// Environment variables used launching the debugging container
	EnvVars []NVPair
	/// launch the debug container with an interactive terminal attached (like '--it' in docker)
	DoTerminal bool
	/// make it look like shell is running in the target container
	DoRunAsTargetShell bool
	/// Kubeconfig file path (k8s runtime)
	Kubeconfig string
	/// Debug session container name
	Session string
	/// Simple (non-debug) action - list namespaces
	ActionListNamespaces bool
	/// Simple (non-debug) action - list pods
	ActionListPods bool
	/// Simple (non-debug) action - list debuggable container
	ActionListDebuggableContainers bool
	/// Simple (non-debug) action - list debug sessions
	ActionListSessions bool
	/// Simple (non-debug) action - show debug sessions logs
	ActionShowSessionLogs bool
	/// Simple (non-debug) action - connect to an existing debug session
	ActionConnectSession bool
}

func ParseNameValueList(list []string) []NVPair {
	var pairs []NVPair
	for _, val := range list {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		parts := strings.SplitN(val, "=", 2)
		if len(parts) != 2 {
			continue
		}

		nv := NVPair{Name: parts[0], Value: parts[1]}
		pairs = append(pairs, nv)
	}

	return pairs
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
		cflag(FlagWorkdir),
		cflag(FlagEnv),
		cflag(FlagTerminal),
		cflag(FlagRunAsTargetShell),
		cflag(FlagListSessions),
		cflag(FlagShowSessionLogs),
		cflag(FlagConnectSession),
		cflag(FlagSession),
		cflag(FlagListNamespaces),
		cflag(FlagListPods),
		cflag(FlagListDebuggableContainers),
		cflag(FlagListDebugImage),
		cflag(FlagKubeconfig),
	},
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		if ctx.Bool(FlagListDebugImage) {
			xc.Out.State("action.list_debug_images")
			for k, v := range debugImages {
				xc.Out.Info("debug.image", ovars{"name": k, "description": v})
			}

			return nil
		}

		commandParams := &CommandParams{
			Runtime:                        ctx.String(FlagRuntime),
			TargetRef:                      ctx.String(FlagTarget),
			TargetNamespace:                ctx.String(FlagNamespace),
			TargetPod:                      ctx.String(FlagPod),
			DebugContainerImage:            ctx.String(FlagDebugImage),
			DoTerminal:                     ctx.Bool(FlagTerminal),
			DoRunAsTargetShell:             ctx.Bool(FlagRunAsTargetShell),
			Kubeconfig:                     ctx.String(FlagKubeconfig),
			Workdir:                        ctx.String(FlagWorkdir),
			EnvVars:                        ParseNameValueList(ctx.StringSlice(FlagEnv)),
			Session:                        ctx.String(FlagSession),
			ActionListNamespaces:           ctx.Bool(FlagListNamespaces),
			ActionListPods:                 ctx.Bool(FlagListPods),
			ActionListDebuggableContainers: ctx.Bool(FlagListDebuggableContainers),
			ActionListSessions:             ctx.Bool(FlagListSessions),
			ActionShowSessionLogs:          ctx.Bool(FlagShowSessionLogs),
			ActionConnectSession:           ctx.Bool(FlagConnectSession),
		}

		if commandParams.Runtime != KubernetesRuntime &&
			(commandParams.ActionListNamespaces ||
				commandParams.ActionListPods) {
			var actionName string
			if commandParams.ActionListNamespaces {
				actionName = FlagListNamespaces
			}

			if commandParams.ActionListPods {
				actionName = FlagListPods
			}

			xc.Out.Error("param", "unsupported runtime flag")
			xc.Out.State("exited",
				ovars{
					"runtime.provided": commandParams.Runtime,
					"runtime.required": KubernetesRuntime,
					"action":           actionName,
					"exit.code":        -1,
				})

			xc.Exit(-1)
		}

		var err error
		if rawEntrypoint := ctx.String(FlagEntrypoint); rawEntrypoint != "" {
			commandParams.Entrypoint, err = command.ParseExec(rawEntrypoint)
			if err != nil {
				return err
			}
		}

		if rawCmd := ctx.String(FlagCmd); rawCmd != "" {
			commandParams.Cmd, err = command.ParseExec(rawCmd)
			if err != nil {
				return err
			}
		}

		//explicitly setting the entrypoint and/or cmd clears
		//implies a custom debug session where the 'RATS' setting should be ignored
		if len(commandParams.Entrypoint) > 0 || len(commandParams.Cmd) > 0 {
			commandParams.DoRunAsTargetShell = false
		}

		if commandParams.DoRunAsTargetShell {
			commandParams.DoTerminal = true
		}

		if !commandParams.ActionListNamespaces &&
			!commandParams.ActionListPods &&
			!commandParams.ActionListDebuggableContainers &&
			!commandParams.ActionListSessions &&
			!commandParams.ActionShowSessionLogs &&
			!commandParams.ActionConnectSession &&
			commandParams.TargetRef == "" {
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
					cmdSlice := ctx.Args().Slice()[2:]
					var cmdClean []string
					for _, v := range cmdSlice {
						v = strings.TrimSpace(v)
						if v != "" {
							cmdClean = append(cmdClean, v)
						}
					}
					if len(cmdClean) > 0 {
						commandParams.Cmd = cmdClean
						commandParams.DoTerminal = false
						commandParams.DoRunAsTargetShell = false
					}
				}
			}
		}

		if commandParams.DebugContainerImage == "" {
			commandParams.DebugContainerImage = BusyboxImage
		}

		OnCommand(
			xc,
			gcvalues,
			commandParams)

		return nil
	},
}
