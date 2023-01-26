//go:build linux
// +build linux

package sensor

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
	syscall.SIGSTOP,
	syscall.SIGCONT,
}

func startSystemSignalsMonitor(cleanup func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, signals...)
	go func() {
		sig := <-sigChan
		log.Debugf("sensor: cleanup on signal (%v)...", sig)
		cleanup()
		os.Exit(0)
	}()
}

func signalFromString(s string) syscall.Signal {
	if !strings.HasPrefix(s, "SIG") {
		s = "SIG" + s
	}
	return unix.SignalNum(s)
}
