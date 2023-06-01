package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/server"
)

func init() {
	server.RegisterCommand()
}
