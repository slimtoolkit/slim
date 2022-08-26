//go:build linux
// +build linux

package app

import (
	log "github.com/sirupsen/logrus"
)

func cleanupOnShutdown() {
	log.Debug("cleanupOnShutdown()")

	if doneChan != nil {
		close(doneChan)
		doneChan = nil
	}
}
