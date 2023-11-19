package update

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands"
)

func RegisterCommand() {
	commands.CLI = append(commands.CLI, CLI)
	commands.CommandFlagSuggestions[Name] = CommandFlagSuggestions
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
