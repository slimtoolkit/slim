package merge

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
		{Text: commands.FullFlagName(FlagImage), Description: FlagImageUsage},
		{Text: commands.FullFlagName(FlagUseLastImageMetadata), Description: FlagUseLastImageMetadataUsage},
		{Text: commands.FullFlagName(FlagTag), Description: FlagTagUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(FlagUseLastImageMetadata): commands.CompleteBool,
	},
}
