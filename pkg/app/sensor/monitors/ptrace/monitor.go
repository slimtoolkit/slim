// +build !arm64

package ptrace

import (
	"time"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/monitor/ptrace"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/sirupsen/logrus"
)

// Run starts the PTRACE monitor
func Run(
	errorCh chan error,
	ackCh chan<- bool,
	startCh <-chan int,
	stopCh chan struct{},
	appName string,
	appArgs []string,
	dirName string,
	appUser string,
	runTargetAsUser bool) <-chan *report.PtMonitorReport {
	log.Info("ptmon: Run")

	ptApp, err := ptrace.Run(
		appName,
		appArgs,
		dirName,
		appUser,
		runTargetAsUser,
		nil,
		errorCh,
		nil,
		stopCh)
	if err != nil {
		if ackCh != nil {
			ackCh <- false
			time.Sleep(2 * time.Second)
		}

		if errorCh != nil {
			sensorErr := errors.SE("sensor.ptrace.Run/ptrace.Run", "call.error", err)
			errorCh <- sensorErr
			time.Sleep(3 * time.Second)
		}

		errutil.FailOn(err)
	}

	go func() {
		for {
			select {
			case <-stopCh:
				log.Debug("ptmon: pta state watcher - stopping...")
				return
			case state := <-ptApp.StateCh:
				log.Debugf("ptmon: pta state watcher - state => %v", state)
				switch state {
				case ptrace.AppStarted:
					log.Debug("ptmon: pta state watcher - state(started)...")
					if ackCh != nil {
						ackCh <- true
					}
				case ptrace.AppFailed:
					log.Debug("ptmon: pta state watcher - state(failed)...")
					if ackCh != nil {
						ackCh <- false
					}
					return
				case ptrace.AppDone, ptrace.AppExited:
					log.Debug("ptmon: pta state watcher - state(terminated)...")
				}
			}
		}
	}()

	return ptApp.ReportCh
}
