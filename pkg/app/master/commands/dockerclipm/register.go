package dockerclipm

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands"
)

func RegisterCommand() {
	commands.CLI = append(commands.CLI, CLI)
}
