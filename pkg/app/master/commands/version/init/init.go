package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands/version"
)

func init() {
	version.RegisterCommand()
}
