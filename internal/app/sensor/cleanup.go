// +build linux

package app

import (
	log "github.com/Sirupsen/logrus"
)

func cleanupOnShutdown() {
	log.Debug("cleanupOnShutdown()")

	if doneChan != nil {
		close(doneChan)
		doneChan = nil
	}
}
