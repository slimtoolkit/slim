package init

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/xray"
)

func init() {
	xray.RegisterCommand()
}
