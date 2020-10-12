package help

import (
	"github.com/docker-slim/docker-slim/internal/app/master/commands"
)

func init() {
	commands.CLI = append(commands.CLI, CLI)
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
