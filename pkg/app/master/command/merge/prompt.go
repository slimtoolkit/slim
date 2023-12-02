package merge

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
		{Text: command.FullFlagName(FlagImage), Description: FlagImageUsage},
		{Text: command.FullFlagName(FlagUseLastImageMetadata), Description: FlagUseLastImageMetadataUsage},
		{Text: command.FullFlagName(FlagTag), Description: FlagTagUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(FlagUseLastImageMetadata): command.CompleteBool,
	},
}
