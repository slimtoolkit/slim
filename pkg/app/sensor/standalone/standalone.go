package standalone

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/slimtoolkit/slim/pkg/app/sensor/artifact"
	"github.com/slimtoolkit/slim/pkg/app/sensor/execution"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
)

type Sensor struct {
	ctx context.Context
	exe execution.Interface

	newMonitor monitor.NewCompositeMonitorFunc
	del        mondel.Publisher
	artifactor artifact.Processor

	workDir    string
	mountPoint string

	signalCh chan os.Signal

	stopSignal          os.Signal
	stopGracePeriod     time.Duration
	stopCommandReceived bool
}

func NewSensor(
	ctx context.Context,
	exe execution.Interface,
	newMonitor monitor.NewCompositeMonitorFunc,
	del mondel.Publisher,
	artifactor artifact.Processor,
	workDir string,
	mountPoint string,
	stopSignal os.Signal,
	stopGracePeriod time.Duration,
) *Sensor {
	return &Sensor{
		ctx:             ctx,
		exe:             exe,
		newMonitor:      newMonitor,
		del:             del,
		artifactor:      artifactor,
		workDir:         workDir,
		mountPoint:      mountPoint,
		stopSignal:      stopSignal,
		stopGracePeriod: stopGracePeriod,
	}
}

func (s *Sensor) Run() error {
	s.exe.HookSensorPostStart()

	err := s.run()
	if err != nil {
		s.exe.PubEvent(event.Error, err.Error())
	}

	// We have to dump the artifacts before invokin the pre-shutdown
	// hook - it may want to upload the artifacts somewhere.
	errutil.WarnOn(s.artifactor.Archive())

	s.exe.HookSensorPreShutdown()
	s.exe.PubEvent(event.ShutdownSensorDone)

	// The target app can be done before the stop signal is received
	// because of two reasons:
	//
	// - App terminated on its own (e.g., a typical CLI use case)
	//
	// - The "stop monitor" control command was received.
	//
	// In the latter case, sensor needs to wait for the stop signal
	// before proceeding with the shutdown because otherwise the container
	// runtime may restart the container which is not desirable.
	if s.stopCommandReceived {
		select {
		case <-s.ctx.Done():
		case <-s.signalCh:
		}
	}

	return err
}

func (s *Sensor) run() error {
	raw := <-s.exe.Commands()
	cmd, ok := (raw).(*command.StartMonitor)
	if !ok {
		log.
			WithField("cmd", fmt.Sprintf("%+v", cmd)).
			Error("sensor: unexpected start monitor command")
		s.exe.HookMonitorFailed()

		s.exe.PubEvent(event.StartMonitorFailed,
			&event.StartMonitorFailedData{
				Component: event.ComSensorCmdServer,
				State:     event.StateCmdStartMonCmdWaiting,
				Context: map[string]string{
					event.CtxCmdType: string(raw.GetName()),
				},
			})

		return fmt.Errorf("unexpected start monitor command: %+v", cmd)
	}

	if err := s.artifactor.PrepareEnv(cmd); err != nil {
		log.WithError(err).Error("sensor: artifactor.PrepareEnv() failed")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed,
			&event.StartMonitorFailedData{
				Component: event.ComSensorCmdServer,
				State:     event.StateEnvPreparing,
				Errors:    []string{err.Error()},
			})
		return fmt.Errorf("failed to prepare artifacts env: %w", err)
	}

	origPaths, err := s.artifactor.GetCurrentPaths(s.mountPoint, cmd.Excludes)
	if err != nil {
		log.WithError(err).Error("sensor: artifactor.GetCurrentPaths() failed")
		return fmt.Errorf("failed to enumerate current paths: %w", err)
	}

	s.exe.HookMonitorPreStart()

	mon, err := s.newMonitor(
		s.ctx,
		cmd,
		s.workDir,
		s.del,
		s.artifactor.ArtifactsDir(),
		s.mountPoint,
		origPaths,
	)
	if err != nil {
		log.WithError(err).Error("sensor: failed to create composite monitor")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed,
			&event.StartMonitorFailedData{
				Component: event.ComSensorCmdServer,
				State:     event.StateMonCreating,
				Errors:    []string{err.Error()},
			})
		return err
	}

	s.signalCh = make(chan os.Signal, 1024)
	go s.runSignalForwarder(mon)

	if err := mon.Start(); err != nil {
		log.WithError(err).Error("sensor: failed to start composite monitor")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed,
			&event.StartMonitorFailedData{
				Component: event.ComSensorCmdServer,
				State:     event.StateMonStarting,
				Errors:    []string{err.Error()},
			})
		return err
	}
	s.exe.PubEvent(event.StartMonitorDone)

	s.runMonitor(mon)

	report, err := mon.Status()
	if err != nil {
		log.WithError(err).Error("sensor: target app failed")
		return err
	}
	log.Info("sensor: target app is done")
	s.exe.HookMonitorPostShutdown()
	s.exe.PubEvent(event.StopMonitorDone)

	if err := s.artifactor.Process(
		cmd,
		s.mountPoint,
		report.PeReport,
		report.FanReport,
		report.PtReport,
	); err != nil {
		log.WithError(err).Error("sensor: artifact.Process() failed")
		return fmt.Errorf("saving reports failed: %w", err)
	}

	return nil
}

func (s *Sensor) runMonitor(mon monitor.CompositeMonitor) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-mon.Done():
			break loop

		case cmd := <-s.exe.Commands():
			log.Infof("sensor: recieved control command => %s", cmd.GetName())
			if cmd.GetName() == command.StopMonitorName {
				s.stopCommandReceived = true
				s.signalTargetApp(mon, s.stopSignal)
			} else {
				log.Warnf("sensor: unsupported control command => %s", cmd.GetName())
			}

		case err := <-mon.Errors():
			log.WithError(err).Warn("sensor: non-critical monitor error condition")
			s.exe.PubEvent(event.Error, monitor.NonCriticalError(err).Error())

		case <-ticker.C:
			s.exe.HookTargetAppRunning()
			log.Debug(".")
		}

		// TODO: Implement me!
		// case file := <-mon.Files():
		//   Stub for the incremental artifact storing.
	}

	// A bit of code duplication to avoid starting a goroutine
	// for error event handling - keeping the control flow
	// "single-threaded" keeps reasoning about the logic.
	for _, err := range mon.DrainErrors() {
		log.WithError(err).Warn("sensor: non-critical monitor error condition (drained)")
		s.exe.PubEvent(event.Error, monitor.NonCriticalError(err).Error())
	}
}

// TODO: Combine the signal forwarder loop with the run monitor loop
//
//	to avoid competting event loops - this will simplify the code
//	and let us avoid subtle race conditions when the stop signal
//	arrives while the app is being stoped due to the stop control command.
func (s *Sensor) runSignalForwarder(mon monitor.CompositeMonitor) {
	log.Debug("sensor: starting forwarding signals to target app...")

	signal.Notify(s.signalCh)

	for {
		select {
		case <-s.ctx.Done():
			log.Debug("sensor: forwarding signal to target app no more - sensor is done")
			return

		case sig := <-s.signalCh:
			if s.stopCommandReceived && sig == s.stopSignal {
				signal.Stop(s.signalCh)
				close(s.signalCh)
			}

			select {
			case <-mon.Done():
				log.Debug("sensor: skipping signal forwarding - target app is done")

			default:
				if sig != syscall.SIGCHLD {
					// Due to ptrace, SIGCHLDs flood the output.
					// TODO: Log SIGCHLD if ptrace-ing is off.
					log.Debugf("sensor: forwarding signal %s to target app", unix.SignalName(sig.(syscall.Signal)))
				}
				s.signalTargetApp(mon, sig)
			}
		}
	}
}

func (s *Sensor) signalTargetApp(mon monitor.CompositeMonitor, sig os.Signal) {
	mon.SignalTargetApp(sig)

	if sig == s.stopSignal {
		log.Debug("sensor: stop signal was sent to target app - starting grace period")

		go func() {
			// Starting the grace period.
			timer := time.NewTimer(s.stopGracePeriod)
			defer func() {
				if !timer.Stop() {
					<-timer.C
				}
			}()

			select {
			case <-mon.Done():
				log.Debug("sensor: target app finished before grace timeout - dismantling SIGKILL")

			case <-timer.C:
				log.Debug("sensor: grace timeout expired - SIGKILL goes to target app")
				mon.SignalTargetApp(syscall.SIGKILL)
			}
		}()
	}
}
