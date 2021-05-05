package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/profile"
)

func init() {
	profile.RegisterCommand()
}
