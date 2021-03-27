package version

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

func init() {
	commands.CLI = append(commands.CLI, CLI)
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
