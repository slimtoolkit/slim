package main

import (
	"os"
	
	"launcher/ipc"

	log "github.com/Sirupsen/logrus"
)

func cleanupOnStartup() {
	if _, err := os.Stat("/tmp/docker-slim-launcher.cmds.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-launcher.cmds.ipc"); err != nil {
			log.Warnf("Error removing unix socket %s: %s", "/tmp/docker-slim-launcher.cmds.ipc", err.Error())
		}
	}

	if _, err := os.Stat("/tmp/docker-slim-launcher.events.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-launcher.events.ipc"); err != nil {
			log.Warnf("Error removing unix socket %s: %s", "/tmp/docker-slim-launcher.events.ipc", err.Error())
		}
	}
}

func cleanupOnShutdown() {
	log.Debug("cleanupOnShutdown()")

	if doneChan != nil {
		close(doneChan)
		doneChan = nil
	}

	ipc.ShutdownChannels()
}
