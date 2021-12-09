package install

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
		{Text: commands.FullFlagName(FlagDockerCLIPlugin), Description: FlagDockerCLIPluginUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(FlagDockerCLIPlugin): commands.CompleteBool,
	},
}
