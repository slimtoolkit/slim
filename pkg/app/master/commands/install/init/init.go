package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/install"
)

func init() {
	install.RegisterCommand()
}
