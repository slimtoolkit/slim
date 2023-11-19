package update

import (
	"github.com/c-bata/go-prompt"

	"github.com/slimtoolkit/slim/pkg/app/master/commands"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &commands.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: commands.FullFlagName(commands.FlagShowProgress), Description: commands.FlagShowProgressUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(commands.FlagShowProgress): commands.CompleteProgress,
	},
}
