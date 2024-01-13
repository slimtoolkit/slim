package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor/fanotify"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor/ptrace"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
)

const (
	signalChanBufSize = 10

	errorChanBufSize   = 100
	errorChanDrainTime = 200 * time.Millisecond

	// Some monitors are passive. If the driving monitor
	// is too fast, the passive ones may not have a chance
	// to track all the needed events (used to happen often
	// between the driving ptrace and observing fanotify mons).
	minPassiveMonitoring = 1 * time.Second
)

var (
	ErrInsufficientPermissions = errors.New("insufficient permissions")
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

	SignalTargetApp(s os.Signal)

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
	del mondel.Publisher

	startedAt time.Time

	// peMon  *pevent.Monitor
	fanMon fanotify.Monitor
	ptMon  ptrace.Monitor

	// Inspired by os/exec.Cmd
	closeAfterDone []io.Closer

	signalCh chan os.Signal

	doneCh   chan struct{}
	doneOnce sync.Once

	errorCh chan error
}

var _ CompositeMonitor = (*monitor)(nil)

type NewCompositeMonitorFunc func(
	ctx context.Context,
	cmd *command.StartMonitor,
	workDir string,
	del mondel.Publisher,
	artifactsDir string,
	mountPoint string,
	origPaths map[string]struct{},
) (CompositeMonitor, error)

func NewCompositeMonitor(
	ctx context.Context,
	cmd *command.StartMonitor,
	workDir string,
	del mondel.Publisher,
	artifactsDir string,
	mountPoint string,
	origPaths map[string]struct{},
) (CompositeMonitor, error) {
	log.Info("sensor: creating monitors...")

	errorCh := make(chan error, errorChanBufSize)

	fanMon := fanotify.NewMonitor(
		ctx,
		del,
		artifactsDir,
		mountPoint,
		cmd.IncludeNew,
		origPaths,
		errorCh,
	)

	var closeAfterDone []io.Closer

	appStdout := os.Stdout
	if cmd.AppStdoutToFile {
		sink, file, err := dupAppStdStream(artifactsDir, os.Stdout, "stdout")
		if err != nil {
			return nil, err
		}
		closeAfterDone = append(closeAfterDone, sink, file)
		appStdout = sink
	}

	appStderr := os.Stderr
	if cmd.AppStderrToFile {
		sink, file, err := dupAppStdStream(artifactsDir, os.Stderr, "stderr")
		if err != nil {
			closeAll(closeAfterDone)
			return nil, err
		}
		closeAfterDone = append(closeAfterDone, sink, file)
		appStderr = sink
	}

	signalCh := make(chan os.Signal, signalChanBufSize)

	ptMon := ptrace.NewMonitor(
		ctx,
		del,
		artifactsDir,
		ptrace.AppRunOpt{
			Cmd:                 cmd.AppName,
			Args:                cmd.AppArgs,
			AppStdout:           appStdout,
			AppStderr:           appStderr,
			WorkDir:             workDir,
			User:                cmd.AppUser,
			RunAsUser:           cmd.RunTargetAsUser,
			RTASourcePT:         cmd.RTASourcePT,
			ReportOnMainPidExit: cmd.ReportOnMainPidExit,
		},
		cmd.IncludeNew,
		origPaths,
		signalCh,
		errorCh,
	)

	m := Compose(cmd, del, fanMon, ptMon, signalCh, errorCh)
	m.closeAfterDone = closeAfterDone

	return m, nil
}

func Compose(
	cmd *command.StartMonitor,
	del mondel.Publisher,
	fanMon fanotify.Monitor,
	ptMon ptrace.Monitor,
	signalCh chan os.Signal,
	errorCh chan error,
) *monitor {
	return &monitor{
		cmd: cmd,
		del: del,
		// TODO: peMon:  peMon,
		fanMon: fanMon,
		ptMon:  ptMon,

		signalCh: signalCh,
		errorCh:  errorCh,
	}
}

// TODO: Consider adding an option to make fanotify
//
//	and pevent monitor errors non-fatal.
func (m *monitor) Start() error {
	log.Info("sensor: starting monitors...")

	// if err := m.peMon.Start(); err != nil {
	// 	return err
	// }

	if err := m.fanMon.Start(); err != nil {
		log.WithError(err).Debug("sensor: composite monitor - FAN error")
		log.Error("sensor: composite monitor - FAN failed to start running")

		if strings.Contains(err.Error(), "operation not permitted") {
			return ErrInsufficientPermissions
		}

		closeAll(m.closeAfterDone)
		return err
	}

	if err := m.ptMon.Start(); err != nil {
		log.WithError(err).Debug("sensor: composite monitor - PTAN error")
		log.Error("sensor: composite monitor - PTAN failed to start running")

		closeAll(m.closeAfterDone)
		return err
	}

	m.startedAt = time.Now()

	return nil
}

func (m *monitor) StartCommand() *command.StartMonitor {
	return m.cmd
}

func (m *monitor) SignalTargetApp(s os.Signal) {
	m.signalCh <- s
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

			// This code smells... If the ptrace monitor finished too quickly,
			// we have to give the other monitors more time to increase their
			// chances to track the needed events. But we only do this if the
			// driving monitor finished successfully.
			elapsed := time.Since(m.startedAt)
			if _, err := m.ptMon.Status(); err == nil && elapsed < minPassiveMonitoring {
				time.Sleep(minPassiveMonitoring - elapsed)
			}

			// m.peMon.Cancel()
			m.fanMon.Cancel()

			// <-m.peMon.Done()
			<-m.fanMon.Done()
			log.Debug("sensor: composite monitor - fanmon is done")

			closeAll(m.closeAfterDone)

			//need to call del.Stop here to make sure we get all drained monitor events
			if m.del != nil {
				m.del.Stop()
			}

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

// Using simple io.MultiWriter(os.Stdout, os.File) would make cmd.Wait()
// block until either the cmd's stdout is closed or the multi-writer is closed.
// However, both are impossible. We need the Wait() to return much earlier
// than the process termination (see pkg/monitors/ptrace logic), and multi-writer
// cannot be closed at all. Hence, the pipe trick.
func dupAppStdStream(artifactsDir string, w io.Writer, kind string) (*os.File, *os.File, error) {
	filename := filepath.Join(artifactsDir, "app_"+kind+".log")

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open file %q to duplicate app's %s stream: %w", filename, kind, err)
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("cannot create pipe for app %s stream: %w", kind, err)
	}

	go func() {
		n, err := io.Copy(io.MultiWriter(w, f), pr)
		log.Debugf("dupAppStdStream: io.Copy() finished; written=%d error=%v", n, err)
	}()

	return pw, f, nil
}

func closeAll(cs []io.Closer) {
	for _, c := range cs {
		errutil.WarnOn(c.Close())
	}
}
