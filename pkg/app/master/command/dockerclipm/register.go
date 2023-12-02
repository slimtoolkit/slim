package dockerclipm

import (
	"github.com/c-bata/go-prompt"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

func RegisterCommand() {
	command.AddCLICommand(
		Name,
		CLI,
		prompt.Suggest{},
		nil)
}
