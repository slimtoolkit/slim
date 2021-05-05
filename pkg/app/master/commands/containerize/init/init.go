package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/containerize"
)

func init() {
	containerize.RegisterCommand()
}
