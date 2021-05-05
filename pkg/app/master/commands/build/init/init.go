package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/build"
)

func init() {
	build.RegisterCommand()
}
