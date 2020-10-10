package app

import "github.com/docker-slim/docker-slim/internal/app/master/signals"

// Run starts the master app
func Run() {
	signals.InitHandlers()
	runCli()
}
