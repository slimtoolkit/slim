package app

// Run starts the master app
func Run() {
	initSignalHandlers()
	runCli()
}
