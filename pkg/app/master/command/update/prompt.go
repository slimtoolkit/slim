package update

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
		{Text: command.FullFlagName(command.FlagShowProgress), Description: command.FlagShowProgressUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagShowProgress): command.CompleteProgress,
	},
}
