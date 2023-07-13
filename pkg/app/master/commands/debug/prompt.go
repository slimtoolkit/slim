package debug

import (
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
		{Text: commands.FullFlagName(FlagTerminal), Description: FlagTerminalUsage},
		{Text: commands.FullFlagName(FlagListDebugImage), Description: FlagListDebugImageUsage},
		{Text: commands.FullFlagName(FlagKubeconfig), Description: FlagKubeconfigUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(FlagRuntime): completeRuntimes,
		//commands.FullFlagName(FlagTarget): completeXXXX,
		commands.FullFlagName(FlagDebugImage):     completeDebugImages,
		commands.FullFlagName(FlagTerminal):       commands.CompleteTBool,
		commands.FullFlagName(FlagListDebugImage): commands.CompleteBool,
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

func completeDebugImages(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(getDebugImageValues(), token, true)
}

var runtimeValues = []prompt.Suggest{
	{Text: DockerRuntime, Description: "Docker runtime - debug a container running in Docker"},
	{Text: KubernetesRuntime, Description: "Kubernetes runtime - debug a container running in Kubernetes"},
}

func completeRuntimes(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(runtimeValues, token, true)
}
