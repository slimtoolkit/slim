package standalone

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/artifact"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/execution"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitor"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
)

const (
	signalBufSize = 10
)

type Sensor struct {
	ctx context.Context
	exe execution.Interface

	newMonitor monitor.NewCompositeMonitorFunc
	artifactor artifact.Processor

	workDir    string
	mountPoint string

	stopSignal      os.Signal
	stopGracePeriod time.Duration
}

func NewSensor(
	ctx context.Context,
	exe execution.Interface,
	newMonitor monitor.NewCompositeMonitorFunc,
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
		artifactor:      artifactor,
		workDir:         workDir,
		mountPoint:      mountPoint,
		stopSignal:      stopSignal,
		stopGracePeriod: stopGracePeriod,
	}
}

func (s *Sensor) Run() error {
	cmd, ok := (<-s.exe.Commands()).(*command.StartMonitor)
	if !ok {
		log.
			WithField("cmd", fmt.Sprintf("%+v", cmd)).
			Error("sensor: unexpected start monitor command")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed)
		return fmt.Errorf("unexpected start monitor command: %+v", cmd)
	}

	if err := s.artifactor.PrepareEnv(cmd); err != nil {
		log.WithError(err).Error("sensor: artifactor.PrepareEnv() failed")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed)
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
		s.artifactor.ArtifactsDir(),
		s.mountPoint,
		origPaths,
		initSignalForwardingChannel(s.ctx, s.stopSignal, s.stopGracePeriod),
	)
	if err != nil {
		log.WithError(err).Error("sensor: failed to create composite monitor")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed)
		return err
	}

	if err := mon.Start(); err != nil {
		log.WithError(err).Error("sensor: failed to start composite monitor")
		s.exe.HookMonitorFailed()
		s.exe.PubEvent(event.StartMonitorFailed)
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

	if err := s.artifactor.ProcessReports(
		cmd,
		s.mountPoint,
		report.PeReport,
		report.FanReport,
		report.PtReport,
	); err != nil {
		log.WithError(err).Error("sensor: artifact.ProcessReports() failed")
		return fmt.Errorf("saving reports failed: %w", err)
	}

	s.exe.PubEvent(event.ShutdownSensorDone)
	return nil
}

func (s *Sensor) runMonitor(mon monitor.CompositeMonitor) {
loop:
	for {
		select {
		case <-mon.Done():
			break loop

		case err := <-mon.Errors():
			log.WithError(err).Warn("sensor: non-critical monitor error condition")
			s.exe.PubEvent(event.Error, monitor.NonCriticalError(err).Error())

		case <-time.After(time.Second * 5):
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

func initSignalForwardingChannel(
	ctx context.Context,
	stopSignal os.Signal,
	stopGracePeriod time.Duration,
) <-chan os.Signal {
	signalCh := make(chan os.Signal, signalBufSize)

	go func() {
		log.Debug("sensor: starting forwarding signals to target app...")

		ch := make(chan os.Signal)
		signal.Notify(ch)

		for {
			select {
			case <-ctx.Done():
				log.Debug("sensor: forwarding signal to target app no more - sensor is done")
				return
			case s := <-ch:
				if s != syscall.SIGCHLD {
					// Due to ptrace, SIGCHLDs flood the output.
					// TODO: Log SIGCHLD if ptrace-ing is off.
					log.WithField("signal", s).Debug("sensor: forwarding signal to target app")
				}
				signalCh <- s

				if s == stopSignal {
					log.Debug("sensor: recieved stop signal - starting grace period")

					// Starting the grace period
					select {
					case <-ctx.Done():
						log.Debug("sensor: finished before grace timeout - dismantling SIGKILL")
					case <-time.After(stopGracePeriod):
						log.Debug("sensor: grace timeout expired - SIGKILL goes to target app")
						signalCh <- syscall.SIGKILL
					}
					return
				}
			}
		}
	}()

	return signalCh
}
