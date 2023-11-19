package init

import (
	"github.com/slimtoolkit/slim/pkg/app/master/commands/lint"
)

func init() {
	lint.RegisterCommand()
}
