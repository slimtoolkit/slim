package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/command/containerize"
)

func init() {
	containerize.RegisterCommand()
}
