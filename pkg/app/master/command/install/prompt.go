package install

import (
	"github.com/c-bata/go-prompt"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(FlagDockerCLIPlugin), Description: FlagDockerCLIPluginUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(FlagDockerCLIPlugin): command.CompleteBool,
	},
}
