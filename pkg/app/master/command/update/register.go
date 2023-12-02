package update

import (
	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

func RegisterCommand() {
	command.AddCLICommand(
		Name,
		CLI,
		CommandSuggestion,
		CommandFlagSuggestions)
}
