package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/probe"
)

func init() {
	probe.RegisterCommand()
}
