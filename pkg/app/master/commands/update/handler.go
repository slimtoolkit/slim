package update

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/update"
)

// OnCommand implements the 'update' command
func OnCommand(doDebug bool, statePath, archiveState string, inContainer, isDSImage bool, doShowProgress bool) {
	update.Run(doDebug, statePath, inContainer, isDSImage, doShowProgress)
}
