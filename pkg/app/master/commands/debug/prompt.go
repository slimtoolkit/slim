package debug

import (
	"fmt"

	"github.com/c-bata/go-prompt"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &commands.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: commands.FullFlagName(FlagRuntime), Description: FlagRuntimeUsage},
		{Text: commands.FullFlagName(FlagTarget), Description: FlagTargetUsage},
		{Text: commands.FullFlagName(FlagNamespace), Description: FlagNamespaceUsage},
		{Text: commands.FullFlagName(FlagPod), Description: FlagPodUsage},
		{Text: commands.FullFlagName(FlagDebugImage), Description: FlagDebugImageUsage},
		{Text: commands.FullFlagName(FlagEntrypoint), Description: FlagEntrypointUsage},
		{Text: commands.FullFlagName(FlagCmd), Description: FlagCmdUsage},
		{Text: commands.FullFlagName(FlagWorkdir), Description: FlagWorkdirUsage},
		{Text: commands.FullFlagName(FlagEnv), Description: FlagEnvUsage},
		{Text: commands.FullFlagName(FlagTerminal), Description: FlagTerminalUsage},
		{Text: commands.FullFlagName(FlagRunAsTargetShell), Description: FlagRunAsTargetShellUsage},
		{Text: commands.FullFlagName(FlagListSessions), Description: FlagListSessionsUsage},
		{Text: commands.FullFlagName(FlagShowSessionLogs), Description: FlagShowSessionLogsUsage},
		{Text: commands.FullFlagName(FlagConnectSession), Description: FlagConnectSessionUsage},
		{Text: commands.FullFlagName(FlagSession), Description: FlagSessionUsage},
		{Text: commands.FullFlagName(FlagListNamespaces), Description: FlagListNamespacesUsage},
		{Text: commands.FullFlagName(FlagListPods), Description: FlagListPodsUsage},
		{Text: commands.FullFlagName(FlagListDebuggableContainers), Description: FlagListDebuggableContainersUsage},
		{Text: commands.FullFlagName(FlagListDebugImage), Description: FlagListDebugImageUsage},
		{Text: commands.FullFlagName(FlagKubeconfig), Description: FlagKubeconfigUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(FlagRuntime):                  completeRuntime,
		commands.FullFlagName(FlagTarget):                   completeTarget,
		commands.FullFlagName(FlagDebugImage):               completeDebugImage,
		commands.FullFlagName(FlagTerminal):                 commands.CompleteTBool,
		commands.FullFlagName(FlagRunAsTargetShell):         commands.CompleteTBool,
		commands.FullFlagName(FlagListSessions):             commands.CompleteBool,
		commands.FullFlagName(FlagShowSessionLogs):          commands.CompleteBool,
		commands.FullFlagName(FlagConnectSession):           commands.CompleteBool,
		commands.FullFlagName(FlagSession):                  completeSession,
		commands.FullFlagName(FlagListNamespaces):           commands.CompleteBool,
		commands.FullFlagName(FlagListPods):                 commands.CompleteBool,
		commands.FullFlagName(FlagListDebuggableContainers): commands.CompleteBool,
		commands.FullFlagName(FlagListDebugImage):           commands.CompleteBool,
		commands.FullFlagName(FlagNamespace):                completeNamespace,
		commands.FullFlagName(FlagPod):                      completePod,
	},
}

func getDebugImageValues() []prompt.Suggest {
	var values []prompt.Suggest
	for k, v := range debugImages {
		value := prompt.Suggest{Text: k, Description: v}
		values = append(values, value)
	}

	return values
}

func completeDebugImage(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(getDebugImageValues(), token, true)
}

var runtimeValues = []prompt.Suggest{
	{Text: DockerRuntime, Description: "Docker runtime - debug a container running in Docker"},
	{Text: KubernetesRuntime, Description: "Kubernetes runtime - debug a container running in Kubernetes"},
}

func completeRuntime(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(runtimeValues, token, true)
}

func completeNamespace(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := commands.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := commands.FullFlagName(FlagRuntime)
		if rtFlagVals, found := ccs.CommandFlags[runtimeFlag]; found {
			if len(rtFlagVals) > 0 && rtFlagVals[0] == KubernetesRuntime {
				kubeconfig := KubeconfigDefault
				kubeconfigFlag := commands.FullFlagName(FlagKubeconfig)
				kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
				if found && len(kcFlagVals) > 0 {
					kubeconfig = kcFlagVals[0]
				}

				names, _ := listNamespacesWithConfig(kubeconfig)
				for _, name := range names {
					value := prompt.Suggest{Text: name}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}

func completePod(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := commands.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := commands.FullFlagName(FlagRuntime)
		if rtFlagVals, found := ccs.CommandFlags[runtimeFlag]; found {
			if len(rtFlagVals) > 0 && rtFlagVals[0] == KubernetesRuntime {
				kubeconfig := KubeconfigDefault
				kubeconfigFlag := commands.FullFlagName(FlagKubeconfig)
				kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
				if found && len(kcFlagVals) > 0 {
					kubeconfig = kcFlagVals[0]
				}

				namespace := NamespaceDefault
				namespaceFlag := commands.FullFlagName(FlagNamespace)
				nsFlagVals, found := ccs.CommandFlags[namespaceFlag]
				if found && len(nsFlagVals) > 0 {
					namespace = nsFlagVals[0]
				}

				names, _ := listActivePodsWithConfig(kubeconfig, namespace)
				for _, name := range names {
					value := prompt.Suggest{Text: name}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}

func completeTarget(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := commands.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := commands.FullFlagName(FlagRuntime)
		rtFlagVals, found := ccs.CommandFlags[runtimeFlag]
		if found && len(rtFlagVals) > 0 && rtFlagVals[0] == KubernetesRuntime {
			kubeconfig := KubeconfigDefault
			kubeconfigFlag := commands.FullFlagName(FlagKubeconfig)
			kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
			if found && len(kcFlagVals) > 0 {
				kubeconfig = kcFlagVals[0]
			}

			namespace := NamespaceDefault
			namespaceFlag := commands.FullFlagName(FlagNamespace)
			nsFlagVals, found := ccs.CommandFlags[namespaceFlag]
			if found && len(nsFlagVals) > 0 {
				namespace = nsFlagVals[0]
			}

			var pod string
			podFlag := commands.FullFlagName(FlagPod)
			podFlagVals, found := ccs.CommandFlags[podFlag]
			if found && len(podFlagVals) > 0 {
				pod = podFlagVals[0]
			}

			result, err := listDebuggableK8sContainersWithConfig(kubeconfig, namespace, pod)
			if err == nil {
				for cname, iname := range result {
					value := prompt.Suggest{
						Text:        cname,
						Description: fmt.Sprintf("image: %s", iname),
					}
					values = append(values, value)
				}
			}
		} else {
			//either no explicit 'runtime' param or other/docker runtime
			//todo: need a way to access/pass the docker client struct (or just pass the connect params)
			result, err := listDebuggableDockerContainersWithConfig(ccs.Dclient)
			if err == nil {
				for cname, iname := range result {
					value := prompt.Suggest{
						Text:        cname,
						Description: fmt.Sprintf("image: %s", iname),
					}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}

func completeSession(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := commands.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		csessValStr := ccs.GetCFValue(FlagConnectSession)

		runtimeFlag := commands.FullFlagName(FlagRuntime)
		rtFlagVals, found := ccs.CommandFlags[runtimeFlag]
		if found && len(rtFlagVals) > 0 && rtFlagVals[0] == KubernetesRuntime {
			kubeconfig := KubeconfigDefault
			kubeconfigFlag := commands.FullFlagName(FlagKubeconfig)
			kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
			if found && len(kcFlagVals) > 0 {
				kubeconfig = kcFlagVals[0]
			}

			namespace := ccs.GetCFValueWithDefault(FlagNamespace, NamespaceDefault)

			var pod string
			podFlag := commands.FullFlagName(FlagPod)
			podFlagVals, found := ccs.CommandFlags[podFlag]
			if found && len(podFlagVals) > 0 {
				pod = podFlagVals[0]
			}

			target := ccs.GetCFValue(FlagTarget)

			result, err := listK8sDebugContainersWithConfig(
				kubeconfig,
				namespace,
				pod,
				target,
				commands.IsTrueStr(csessValStr))

			if err == nil {
				for _, info := range result {
					desc := fmt.Sprintf("state: %s / start_time: %s / target: %s / image: %s",
						info.State,
						info.StartTime,
						info.TargetContainerName,
						info.SpecImage)
					value := prompt.Suggest{
						Text:        info.Name,
						Description: desc,
					}
					values = append(values, value)
				}
			}
		} else {
			//either no explicit 'runtime' param or other/docker runtime
			//todo: need a way to access/pass the docker client struct (or just pass the connect params)
			var target string
			targetFlag := commands.FullFlagName(FlagTarget)
			targetFlagVals, found := ccs.CommandFlags[targetFlag]
			if found && len(targetFlagVals) > 0 {
				target = targetFlagVals[0]
			}

			result, err := listDockerDebugContainersWithConfig(ccs.Dclient,
				target,
				commands.IsTrueStr(csessValStr))
			if err == nil {
				for _, info := range result {
					desc := fmt.Sprintf("state: %s / start_time: %s / target: %s / image: %s",
						info.State,
						info.StartTime,
						info.TargetContainerName,
						info.SpecImage)
					value := prompt.Suggest{
						Text:        info.Name,
						Description: desc,
					}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}
