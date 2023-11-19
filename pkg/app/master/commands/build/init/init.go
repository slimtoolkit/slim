package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands/build"
)

func init() {
	build.RegisterCommand()
}
