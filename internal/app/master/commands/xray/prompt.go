package xray

import (
	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/c-bata/go-prompt"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &commands.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: commands.FullFlagName(commands.FlagTarget), Description: commands.FlagTargetUsage},
		{Text: commands.FullFlagName(FlagChanges), Description: FlagChangesUsage},
		{Text: commands.FullFlagName(FlagLayer), Description: FlagLayerUsage},
		{Text: commands.FullFlagName(FlagAddImageManifest), Description: FlagAddImageManifestUsage},
		{Text: commands.FullFlagName(FlagAddImageConfig), Description: FlagAddImageConfigUsage},
		{Text: commands.FullFlagName(commands.FlagRemoveFileArtifacts), Description: commands.FlagRemoveFileArtifactsUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(commands.FlagTarget):              commands.CompleteTarget,
		commands.FullFlagName(FlagChanges):                      completeLayerChanges,
		commands.FullFlagName(FlagAddImageManifest):             commands.CompleteBool,
		commands.FullFlagName(FlagAddImageConfig):               commands.CompleteBool,
		commands.FullFlagName(commands.FlagRemoveFileArtifacts): commands.CompleteBool,
	},
}

var layerChangeValues = []prompt.Suggest{
	{Text: "none", Description: "Don't show any file system change details in image layers"},
	{Text: "all", Description: "Show all file system change details in image layers"},
	{Text: "delete", Description: "Show only 'delete' file system change details in image layers"},
	{Text: "modify", Description: "Show only 'modify' file system change details in image layers"},
	{Text: "add", Description: "Show only 'add' file system change details in image layers"},
}

func completeLayerChanges(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(layerChangeValues, token, true)
}

func init() {
	commands.CommandFlagSuggestions[Name] = CommandFlagSuggestions
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
