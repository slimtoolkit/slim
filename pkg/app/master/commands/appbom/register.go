package appbom

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands"
)

func RegisterCommand() {
	commands.AddCLICommand(
		Name,
		CLI,
		CommandSuggestion,
		nil)
}
