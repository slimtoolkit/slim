package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands/containerize"
)

func init() {
	containerize.RegisterCommand()
}
