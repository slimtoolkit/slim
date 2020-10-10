package update

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
		{Text: commands.FullFlagName(commands.FlagShowProgress), Description: commands.FlagShowProgressUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(commands.FlagShowProgress): commands.CompleteProgress,
	},
}

func init() {
	commands.CommandFlagSuggestions[Name] = CommandFlagSuggestions
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
