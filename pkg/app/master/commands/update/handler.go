package update

import (
	"github.com/slimtoolkit/slim/pkg/app/master/update"
)

// OnCommand implements the 'update' command
func OnCommand(doDebug bool, statePath, archiveState string, inContainer, isDSImage bool, doShowProgress bool) {
	update.Run(doDebug, statePath, inContainer, isDSImage, doShowProgress)
}
