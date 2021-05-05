package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/lint"
)

func init() {
	lint.RegisterCommand()
}
