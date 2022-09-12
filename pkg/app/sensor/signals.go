//go:build linux
// +build linux

package sensor

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
	syscall.SIGSTOP,
	syscall.SIGCONT,
}

func initSignalHandlers(cleanup func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, signals...)
	go func() {
		sig := <-sigChan
		log.Debugf("sensor: cleanup on signal (%v)...", sig)
		cleanup()
		os.Exit(0)
	}()
}
