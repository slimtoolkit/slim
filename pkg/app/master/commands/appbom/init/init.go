package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands/server"
)

func init() {
	server.RegisterCommand()
}
