//go:build !arm64
// +build !arm64

package ptrace

import (
	"context"
	"os"
	"time"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/monitor/ptrace"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/sirupsen/logrus"
)

// Run starts the PTRACE monitor
func Run(
	ctx context.Context,
	rtaSourcePT bool,
	standalone bool,
	errorCh chan<- error,
	appStartAckCh chan<- bool,
	signalCh <-chan os.Signal,
	appName string,
	appArgs []string,
	dirName string,
	appUser string,
	runTargetAsUser bool,
	includeNew bool,
	origPaths map[string]interface{},
) <-chan *report.PtMonitorReport {
	log.Info("ptmon: Run")

	reportCh := make(chan *report.PtMonitorReport, 1)
	appStateCh := make(chan ptrace.AppState, 10)

	_, err := ptrace.Run(
		ctx,
		rtaSourcePT,
		standalone,
		appName,
		appArgs,
		dirName,
		appUser,
		runTargetAsUser,
		reportCh,
		errorCh,
		appStateCh,
		signalCh,
		includeNew,
		origPaths)
	if err != nil {
		if appStartAckCh != nil {
			appStartAckCh <- false
			time.Sleep(2 * time.Second)
		}

		if errorCh != nil {
			errorCh <- errors.SE("sensor.ptrace.Run/ptrace.Run", "call.error", err)
			time.Sleep(3 * time.Second)
		}

		errutil.FailOn(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug("ptmon: pta state watcher - stopping...")
				return

			case state := <-appStateCh:
				log.Debugf("ptmon: pta state watcher - state => %v", state)

				switch state {
				case ptrace.AppStarted:
					log.Debug("ptmon: pta state watcher - state(started)...")
					if appStartAckCh != nil {
						appStartAckCh <- true
					}

				case ptrace.AppFailed:
					log.Debug("ptmon: pta state watcher - state(failed)...")
					if appStartAckCh != nil {
						appStartAckCh <- false
					}
					return // Don't need to wait for the 'done' state.

				case ptrace.AppDone:
					log.Debug("ptmon: pta state watcher - state(terminated)...")
				}
			}
		}
	}()

	return reportCh
}
