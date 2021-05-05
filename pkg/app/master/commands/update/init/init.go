package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/update"
)

func init() {
	update.RegisterCommand()
}
