package monitors

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors/fanotify"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors/pevent"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors/ptrace"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
)

const (
	errorChanBufSize   = 100
	errorChanDrainTime = 200 * time.Millisecond
)

type CompositeReport struct {
	PeReport  *report.PeMonitorReport
	FanReport *report.FanMonitorReport
	PtReport  *report.PtMonitorReport
}

type CompositeMonitor interface {
	// Start() is not reentrant!
	Start() error

	// Just a helper getter.
	StartCommand() *command.StartMonitor

	Cancel()

	// Done() is reentrant. Every invocation returns the same instance of the channel.
	Done() <-chan struct{}

	// Errors() method is a way to communicate non-fatal monitor's error
	// conditions back to the caller. The method is reentrant. Every invocation
	// returns the same instance of the channel. The error channel is never closed,
	// but ideally it should stop returning eny errors after the monitor is done
	// (the actual behavior might differ due to the subordinate monitors, especially
	// the one at pkg/monitors/ptrace).
	Errors() <-chan error

	// Helper method to read left-over non-critical error events after the monitor
	// is done.
	DrainErrors() []error

	// TODO: Consider adding the Files() method for the incremental
	//       report storing.
	// Files() <-chan *ArtifactProp

	Status() (*CompositeReport, error)
}

// The sensor is designed to be capable of processing multiple
// StartMonitor commands (sequentially?). However, monitors are
// supposed to be one-shot entities. Every time the sensor receives
// a new start command, it should initialize a new instance of the
// composite monitor using fresh instances of the underlying mons.
type monitor struct {
	cmd *command.StartMonitor

	// peMon  *pevent.Monitor
	fanMon fanotify.Monitor
	ptMon  ptrace.Monitor

	doneCh   chan struct{}
	doneOnce sync.Once

	errorCh chan error
}

type NewCompositeMonitorFunc func(
	ctx context.Context,
	cmd *command.StartMonitor,
	workDir string,
	mountPoint string,
	origPaths map[string]struct{},
	signalCh <-chan os.Signal,
) (CompositeMonitor, error)

func NewCompositeMonitor(
	ctx context.Context,
	cmd *command.StartMonitor,
	workDir string,
	mountPoint string,
	origPaths map[string]struct{},
	signalChan <-chan os.Signal,
) (CompositeMonitor, error) {
	log.Info("sensor: creating monitors...")

	//tmp: disable PEVENTs (due to problems with the new boot2docker host OS)
	usePEMon, err := system.DefaultKernelFeatures.IsCompiled("CONFIG_PROC_EVENTS")
	usePEMon = false
	if (err == nil) && usePEMon {
		log.Info("sensor: proc events are available!")
		_ = pevent.Run(ctx.Done())
		//ProcEvents are not enabled in the default boot2docker kernel
	}

	fanMon := fanotify.NewMonitor(ctx, mountPoint, cmd.IncludeNew, origPaths)

	errorCh := make(chan error, errorChanBufSize)
	ptMon := ptrace.NewMonitor(
		ctx,
		ptrace.AppRunOpt{
			Cmd:                 cmd.AppName,
			Args:                cmd.AppArgs,
			WorkDir:             workDir,
			User:                cmd.AppUser,
			RunAsUser:           cmd.RunTargetAsUser,
			RTASourcePT:         cmd.RTASourcePT,
			ReportOnMainPidExit: cmd.ReportOnMainPidExit,
		},
		cmd.IncludeNew,
		origPaths,
		signalChan,
		errorCh,
	)

	return Compose(cmd, fanMon, ptMon, errorCh), nil
}

func Compose(
	cmd *command.StartMonitor,
	fanMon fanotify.Monitor,
	ptMon ptrace.Monitor,
	errorCh chan error,
) *monitor {
	return &monitor{
		cmd: cmd,

		// TODO: peMon:  peMon,
		fanMon: fanMon,
		ptMon:  ptMon,

		errorCh: errorCh,
	}
}

// TODO: Consider adding an option to make fanotify
//       and pevent monitor errors non-fatal.
func (m *monitor) Start() error {
	log.Info("sensor: starting monitors...")

	// if err := m.peMon.Start(); err != nil {
	// 	return err
	// }

	if err := m.fanMon.Start(); err != nil {
		log.
			WithError(err).
			Error("sensor: composite monitor - FAN failed to start running")
		return err
	}

	if err := m.ptMon.Start(); err != nil {
		log.
			WithError(err).
			Error("sensor: composite monitor - PTAN failed to start running")
		return err
	}

	return nil
}

func (m *monitor) StartCommand() *command.StartMonitor {
	return m.cmd
}

func (m *monitor) Cancel() {
	// m.peMon.Cancel()
	m.fanMon.Cancel()
	m.ptMon.Cancel()
}

func (m *monitor) Done() <-chan struct{} {
	m.doneOnce.Do(func() {
		m.doneCh = make(chan struct{})

		go func() {
			log.Debug("sensor: composite monitor - waiting for sub-monitors...")

			// The order matters here because the ptrace monitor is the
			// driving one while the others are rather passive observers.
			<-m.ptMon.Done()
			log.Debug("sensor: composite monitor - ptmon is done")

			// m.peMon.Cancel()
			m.fanMon.Cancel()

			// <-m.peMon.Done()
			<-m.fanMon.Done()
			log.Debug("sensor: composite monitor - fanmon is done")

			// The composite is done when all its subordinates are done.
			close(m.doneCh)
		}()
	})

	return m.doneCh
}

func (m *monitor) Errors() <-chan error {
	return m.errorCh
}

func (m *monitor) DrainErrors() (errors []error) {
	timer := time.After(errorChanDrainTime)

	for {
		select {
		case <-timer:
			return errors

		case err := <-m.errorCh:
			errors = append(errors, err)
		}
	}
}

func (m *monitor) Status() (*CompositeReport, error) {
	// peReport, peErr := m.peMon.Status()
	fanReport, fanErr := m.fanMon.Status()
	ptReport, ptErr := m.ptMon.Status()

	if fanErr != nil || ptErr != nil {
		return nil, fmt.Errorf(
			"one or more monitors failed: fanotify.error=%q, ptrace.error=%q",
			fanErr, ptErr,
		)
	}

	return &CompositeReport{
		// PeReport: peReport,
		FanReport: fanReport,
		PtReport:  ptReport,
	}, nil
}

func NonCriticalError(err error) error {
	return fmt.Errorf("non-critical monitor error: %w", err)
}
