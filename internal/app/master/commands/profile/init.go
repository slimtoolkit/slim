package profile

import (
	"github.com/docker-slim/docker-slim/internal/app/master/commands"
)

func init() {
	commands.CLI = append(commands.CLI, CLI)
	commands.CommandFlagSuggestions[Name] = CommandFlagSuggestions
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
