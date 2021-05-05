package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/edit"
)

func init() {
	edit.RegisterCommand()
}
