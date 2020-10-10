package help

import (
	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/c-bata/go-prompt"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

func init() {
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
