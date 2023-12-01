package dockerclipm

import (
	"github.com/c-bata/go-prompt"

	"github.com/slimtoolkit/slim/pkg/app/master/commands"
)

func RegisterCommand() {
	commands.AddCLICommand(
		Name,
		CLI,
		prompt.Suggest{},
		nil)
}
