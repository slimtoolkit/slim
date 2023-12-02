package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/command/version"
)

func init() {
	version.RegisterCommand()
}
