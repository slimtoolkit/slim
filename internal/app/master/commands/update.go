package commands

import (
	"github.com/docker-slim/docker-slim/internal/app/master/update"
)

// OnUpdate implements the 'update' docker-slim command
func OnUpdate(doDebug bool, statePath string, doShowProgress bool) {
	update.Run(doDebug, statePath, doShowProgress)
}
