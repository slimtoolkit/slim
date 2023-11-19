//go:build !arm64
// +build !arm64

package ptrace

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/errors"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/monitor/ptrace"
	"github.com/slimtoolkit/slim/pkg/report"
)

type status struct {
	report *report.PtMonitorReport
	err    error
}

type monitor struct {
	ctx    context.Context
	cancel context.CancelFunc

	del mondel.Publisher

	artifactsDir string

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

	logger *log.Entry
}

func NewMonitor(
	ctx context.Context,
	del mondel.Publisher,
	artifactsDir string,
	runOpt AppRunOpt,
	includeNew bool,
	origPaths map[string]struct{},
	signalCh <-chan os.Signal,
	errorCh chan<- error,
) Monitor {
	logger := log.WithFields(log.Fields{
		"app": "sensor",
		"com": "ptmon",
	})

	ctx, cancel := context.WithCancel(ctx)
	return &monitor{
		ctx:    ctx,
		cancel: cancel,

		del: del,

		artifactsDir: artifactsDir,

		runOpt: runOpt,

		includeNew: includeNew,
		origPaths:  origPaths,

		signalCh: signalCh,
		errorCh:  errorCh,

		doneCh: make(chan struct{}),
		logger: logger,
	}
}

func (m *monitor) Start() error {
	logger := m.logger.WithField("op", "sensor.pt.monitor.Start")
	logger.Info("call")
	defer logger.Info("exit")

	logger.WithFields(log.Fields{
		"name": m.runOpt.Cmd,
		"args": m.runOpt.Args,
	}).Debug("starting target app...")

	// Despite the name, ptrace.Run() is not blocking.
	app, err := ptrace.Run(
		m.ctx,
		m.del,
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
	logger.
		WithField("state", appState).
		Debugf("pta state watcher - new target app state")

	if appState == ptrace.AppFailed {
		// Don't need to wait for the 'done' state.
		logger.Error("pta state watcher - target app failed")
		return fmt.Errorf("ptmon: target app startup failed: %q", appState)
	}
	if appState != ptrace.AppStarted {
		// Cannot really happen.
		logger.Error("pta state watcher - unexpected target app state")
		return fmt.Errorf("ptmon: unexpected target app state %q", appState)
	}

	// The sync part of the start was successful.

	// Tracking the completetion of the monitor.
	go func() {
		logger := m.logger.WithField("op", "sensor.pt.monitor.completetion.monitor")
		logger.Info("call")
		defer logger.Info("exit")

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
