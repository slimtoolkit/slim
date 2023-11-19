package signals

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var AppContinueChan = make(chan struct{})
var appDoneChan = make(chan struct{})

var signals = []os.Signal{
	syscall.SIGUSR1,
}

func InitHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, signals...)
	log.Debugf("slim: listening for signals - %+v", signals)
	go func() {
		for {
			select {
			case <-appDoneChan:
				return
			case sig := <-sigChan:
				switch sig {
				case syscall.SIGUSR1:
					log.Debug("slim: continue signal")
					AppContinueChan <- struct{}{}
				default:
					log.Debugf("slim: other signal (%v)...", sig)
				}
			}
		}
	}()
}
