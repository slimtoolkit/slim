package app

import (
	//"os"

	"github.com/docker-slim/docker-slim/internal/app/sensor/ipc"

	log "github.com/Sirupsen/logrus"
)

/* use - TBD
func cleanupOnStartup() {
	if _, err := os.Stat("/tmp/docker-slim-sensor.cmds.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-sensor.cmds.ipc"); err != nil {
			log.Warnf("Error removing unix socket %s: %s", "/tmp/docker-slim-sensor.cmds.ipc", err.Error())
		}
	}

	if _, err := os.Stat("/tmp/docker-slim-sensor.events.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-sensor.events.ipc"); err != nil {
			log.Warnf("Error removing unix socket %s: %s", "/tmp/docker-slim-sensor.events.ipc", err.Error())
		}
	}
}
*/

func cleanupOnShutdown() {
	log.Debug("cleanupOnShutdown()")

	if doneChan != nil {
		close(doneChan)
		doneChan = nil
	}

	ipc.ShutdownChannels()
}
