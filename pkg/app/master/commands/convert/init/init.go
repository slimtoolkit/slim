package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/convert"
)

func init() {
	convert.RegisterCommand()
}
