package dockerclipm

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

func RegisterCommand() {
	commands.CLI = append(commands.CLI, CLI)
}
