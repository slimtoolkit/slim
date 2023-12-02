package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/command/build"
)

func init() {
	build.RegisterCommand()
}
