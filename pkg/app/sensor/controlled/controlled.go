package controlled

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/sensor/artifact"
	"github.com/slimtoolkit/slim/pkg/app/sensor/execution"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
)

var ErrPrematureShutdown = errors.New("sensor shutdown before monitor stop")

type Sensor struct {
	ctx context.Context
	exe execution.Interface

	newMonitor monitor.NewCompositeMonitorFunc
	del        mondel.Publisher
	artifactor artifact.Processor

	workDir    string
	mountPoint string
}

func NewSensor(
	ctx context.Context,
	exe execution.Interface,
	newMonitor monitor.NewCompositeMonitorFunc,
	del mondel.Publisher,
	artifactor artifact.Processor,
	workDir string,
	mountPoint string,
) *Sensor {
	return &Sensor{
		ctx:        ctx,
		exe:        exe,
		newMonitor: newMonitor,
		del:        del,
		artifactor: artifactor,
		workDir:    workDir,
		mountPoint: mountPoint,
	}
}

// Sensor can be in two interchanging (and mutually exclusive) "states":
//
//   - (I) No monitor is running
//     -> ShutdownSensor command arrives => clean exit
//     -> StartMonitor command arrives   => go to state II.
//     -> Any other command              => grumble but keep waiting
//
//   - (II) Monitor is running
//     -> StopMonitor command arrives    => stop the mon, dump the report, and go to state I.
//     -> ShutdownSensor command arrives => cancel monitoring, grumble, and exit
//     -> Any other command              => grumble but keep waiting
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

	return err
}

func (s *Sensor) run() error {
	log.Info("sensor: waiting for commands...")

	for {
		mon, err := s.runWithoutMonitor()
		if err != nil {
			s.exe.HookMonitorFailed()
			s.exe.PubEvent(event.StartMonitorFailed,
				&event.StartMonitorFailedData{
					Component: event.ComMonitorRunner, //TODO: need to get to the real component
					State:     s.exe.State(),
					Errors:    []string{err.Error()},
				})

			return fmt.Errorf("run sensor without monitor failed: %w", err)
		}

		if mon == nil {
			return nil
		}

		s.exe.PubEvent(event.StartMonitorDone)

		if err := s.runWithMonitor(mon); err != nil {
			return fmt.Errorf("run sensor with monitor failed: %w", err)
		}

		s.exe.HookMonitorPostShutdown()
		s.exe.PubEvent(event.StopMonitorDone)
	}
}

func (s *Sensor) runWithoutMonitor() (monitor.CompositeMonitor, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-s.exe.Commands():
			log.Debugf("sensor: new command => %+v", cmd)

			switch typedCmd := cmd.(type) {
			case *command.StartMonitor:
				return s.startMonitor(typedCmd)

			case *command.ShutdownSensor:
				return nil, nil // Clean exit

			default:
				log.Warn("sensor: ignoring unknown or unexpected command => ", cmd)
			} // eof: type switch

		case <-ticker.C:
			log.Debug(".")
		} // eof: select
	}
}

func (s *Sensor) startMonitor(cmd *command.StartMonitor) (monitor.CompositeMonitor, error) {
	if err := s.artifactor.PrepareEnv(cmd); err != nil {
		log.WithError(err).Error("sensor: artifactor.PrepareEnv() failed")
		return nil, fmt.Errorf("failed to prepare artifacts env: %w", err)
	}

	origPaths, err := s.artifactor.GetCurrentPaths(s.mountPoint, cmd.Excludes)
	if err != nil {
		log.WithError(err).Error("sensor: artifactor.GetCurrentPaths() failed")
		return nil, fmt.Errorf("failed to enumerate current paths: %w", err)
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
		return nil, err
	}

	if err := mon.Start(); err != nil {
		log.WithError(err).Error("sensor: failed to start composite monitor")
		return nil, err
	}

	log.Info("sensor: monitor started...")

	return mon, nil
}

func (s *Sensor) runWithMonitor(mon monitor.CompositeMonitor) error {
	log.Debug("sensor: monitor.worker - waiting to stop monitoring...")
	log.Debug("sensor: error collector - waiting for errors...")

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	// Only two ways out of this: either StopMonitor or ShutdownSensor.
	stopCommandReceived := false

loop:
	for {
		select {
		case <-mon.Done():
			break loop

		case cmd := <-s.exe.Commands():
			switch cmd.(type) {
			case *command.StopMonitor:
				stopCommandReceived = true
				mon.Cancel()
				break loop

			case *command.ShutdownSensor:
				mon.Cancel() // Dirty exit - abandoning the results.
				return ErrPrematureShutdown

			default:
				log.Info("sensor: ignoring unknown or unexpected command => ", cmd)
			} // eof: type switch

		case err := <-mon.Errors():
			log.WithError(err).Warn("sensor: non-critical monitor error condition")
			s.exe.PubEvent(event.Error, monitor.NonCriticalError(err).Error())

		case <-ticker.C:
			s.exe.HookTargetAppRunning()
			log.Debug(".")
		} // eof: select
	}

	if !stopCommandReceived {
		// Monitor can finish before the stop command is received.
		// In such case, we have to await the explicit stop.
		if cmd, ok := (<-s.exe.Commands()).(*command.StopMonitor); !ok {
			return fmt.Errorf("sensor received unepxected command: %#+v", cmd)
		}
	}

	return s.processMonitoringResults(mon)
}

func (s *Sensor) processMonitoringResults(mon monitor.CompositeMonitor) error {
	// A bit of code duplication to avoid starting a goroutine
	// for error event handling - keeping the control flow
	// "single-threaded" keeps reasoning about the logic.
	for _, err := range mon.DrainErrors() {
		log.WithError(err).Warn("sensor: non-critical monitor error condition (drained)")
		s.exe.PubEvent(event.Error, monitor.NonCriticalError(err).Error())
	}

	log.Info("sensor: composite monitor is done, checking status...")

	report, err := mon.Status()
	if err != nil {
		log.WithError(err).Error("sensor: composite monitor failed")
		return fmt.Errorf("composite monitor failed: %w", err)
	}

	if err := s.artifactor.Process(
		mon.StartCommand(),
		s.mountPoint,
		report.PeReport,
		report.FanReport,
		report.PtReport,
	); err != nil {
		log.WithError(err).Error("sensor: artifact.Process() failed")
		return fmt.Errorf("saving reports failed: %w", err)
	}
	return nil // Clean exit
}
