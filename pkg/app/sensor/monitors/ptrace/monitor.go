//go:build !arm64
// +build !arm64

package ptrace

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/monitor/ptrace"
	"github.com/docker-slim/docker-slim/pkg/report"
)

type status struct {
	report *report.PtMonitorReport
	err    error
}

type monitor struct {
	ctx    context.Context
	cancel context.CancelFunc

	runOpt AppRunOpt

	// TODO: Move the logic behind these two fields to the artifact processig stage.
	includeNew bool
	origPaths  map[string]struct{}

	// To receive signals that should be delivered to the target app.
	signalCh <-chan os.Signal
	errorCh  chan<- error

	app *ptrace.App

	status status
	doneCh chan struct{}
}

func NewMonitor(
	ctx context.Context,
	runOpt AppRunOpt,
	includeNew bool,
	origPaths map[string]struct{},
	signalCh <-chan os.Signal,
	errorCh chan<- error,
) Monitor {
	ctx, cancel := context.WithCancel(ctx)
	return &monitor{
		ctx:    ctx,
		cancel: cancel,

		runOpt: runOpt,

		includeNew: includeNew,
		origPaths:  origPaths,

		signalCh: signalCh,
		errorCh:  errorCh,

		doneCh: make(chan struct{}),
	}
}

func (m *monitor) Start() error {
	log.
		WithField("name", m.runOpt.Cmd).
		WithField("args", m.runOpt.Args).
		Debug("sensor: starting target app...")
	log.Info("ptmon: Start")

	// Despite the name, ptrace.Run() is not blocking.
	app, err := ptrace.Run(
		m.ctx,
		m.runOpt,
		m.includeNew,
		m.origPaths,
		m.signalCh,
		m.errorCh,
	)
	if err != nil {
		return errors.SE("sensor.ptrace.Run/ptrace.Run", "call.error", err)
	}
	m.app = app

	appState := <-app.StateCh
	log.
		WithField("state", appState).
		Debugf("ptmon: pta state watcher - new target app state")

	if appState == ptrace.AppFailed {
		// Don't need to wait for the 'done' state.
		log.Error("ptmon: pta state watcher - target app failed")
		return errors.SE("sensor.ptrace.Run/ptrace.Run", "call.error", err)
	}
	if appState != ptrace.AppStarted {
		// Cannot really happen.
		log.Error("ptmon: pta state watcher - unexpected target app state")
		return fmt.Errorf("ptmon: unexpected target app state %q", appState)
	}

	// The sync part of the start was successful.

	// Tracking the completetion of the monitor.
	go func() {
		appState := <-app.StateCh
		if appState == ptrace.AppDone {
			m.status.report = <-app.ReportCh
		} else {
			m.status.err = fmt.Errorf("ptmon: target app failed with state %q", appState)
		}

		// Monitor is done.
		close(m.doneCh)
	}()

	return nil
}

func (m *monitor) Cancel() {
	m.cancel()
}

func (m *monitor) Done() <-chan struct{} {
	return m.doneCh
}

func (m *monitor) Status() (*report.PtMonitorReport, error) {
	return m.status.report, m.status.err
}
