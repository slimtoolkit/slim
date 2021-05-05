package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/version"
)

func init() {
	version.RegisterCommand()
}
