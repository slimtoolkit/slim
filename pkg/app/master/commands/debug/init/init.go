package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/debug"
)

func init() {
	debug.RegisterCommand()
}
