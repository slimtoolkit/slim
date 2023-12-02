package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/command/lint"
)

func init() {
	lint.RegisterCommand()
}
