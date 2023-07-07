package controlled_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/artifact"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/controlled"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitor"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitor/fanotify"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitor/ptrace"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/test/stub/sensor/execution"
	stubmonitor "github.com/docker-slim/docker-slim/pkg/test/stub/sensor/monitor"
)

// Stubs
func newStubMonitorFunc(
	ctx context.Context,
	fanMon fanotify.Monitor,
	ptMon ptrace.Monitor,
) monitor.NewCompositeMonitorFunc {
	if fanMon == nil {
		fanMon = stubmonitor.NewFanMonitor(ctx)
	}
	if ptMon == nil {
		ptMon = stubmonitor.NewPtMonitor(ctx)
	}

	return func(
		ctx context.Context,
		cmd *command.StartMonitor,
		workDir string,
		artifactsDir string,
		mountPoint string,
		origPaths map[string]struct{},
		signalCh <-chan os.Signal,
	) (monitor.CompositeMonitor, error) {
		return monitor.Compose(
			cmd,
			fanMon,
			ptMon,
			nil,
		), nil
	}
}

type artifactorStub struct{}

var _ artifact.Processor = &artifactorStub{}

func (a *artifactorStub) ArtifactsDir() string {
	return ""
}

func (a *artifactorStub) GetCurrentPaths(root string, excludes []string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

func (a *artifactorStub) PrepareEnv(cmd *command.StartMonitor) error {
	return nil
}

func (a *artifactorStub) Archive() error {
	return nil
}

func (a *artifactorStub) ProcessReports(
	cmd *command.StartMonitor,
	mountPoint string,
	peReport *report.PeMonitorReport,
	fanReport *report.FanMonitorReport,
	ptReport *report.PtMonitorReport,
) error {
	return nil
}

// Tests
func TestStartStopShutdown(t *testing.T) {
	ctx := context.Background()
	exe := execution.NewExecution()
	sen := controlled.NewSensor(
		ctx,
		exe,
		newStubMonitorFunc(ctx, nil, nil),
		&artifactorStub{},
		"", "",
	)

	go func() {
		time.Sleep(100 * time.Millisecond)
		exe.SendCommand(&command.StartMonitor{})

		time.Sleep(100 * time.Millisecond)
		exe.SendCommand(&command.StopMonitor{})

		time.Sleep(100 * time.Millisecond)
		exe.SendCommand(&command.ShutdownSensor{})
	}()

	if err := sen.Run(); err != nil {
		t.Fatal("Unexpected sensor run error:", err)
	}
}

func TestShutdownBeforeStart(t *testing.T) {
	ctx := context.Background()
	exe := execution.NewExecution()
	sen := controlled.NewSensor(
		ctx,
		exe,
		newStubMonitorFunc(ctx, nil, nil),
		&artifactorStub{},
		"", "",
	)

	go func() {
		exe.SendCommand(&command.ShutdownSensor{})
	}()

	if err := sen.Run(); err != nil {
		t.Fatal("Unexpected sensor run error:", err)
	}
}

func TestStartFollowedByShutdown(t *testing.T) {
	ctx := context.Background()
	exe := execution.NewExecution()
	sen := controlled.NewSensor(
		ctx,
		exe,
		newStubMonitorFunc(ctx, nil, nil),
		&artifactorStub{},
		"", "",
	)

	go func() {
		exe.SendCommand(&command.StartMonitor{})
		exe.SendCommand(&command.ShutdownSensor{})
	}()

	if err := sen.Run(); !errors.Is(err, controlled.ErrPrematureShutdown) {
		t.Fatal("Unexpected sensor run error:", err)
	}
}

func TestStopNonStartedMonitor(t *testing.T) {
	ctx := context.Background()
	exe := execution.NewExecution()
	sen := controlled.NewSensor(
		ctx,
		exe,
		newStubMonitorFunc(ctx, nil, nil),
		&artifactorStub{},
		"", "",
	)

	go func() {
		exe.SendCommand(&command.StopMonitor{})
		exe.SendCommand(&command.ShutdownSensor{})
	}()

	if err := sen.Run(); err != nil {
		t.Fatal("Unexpected sensor run error:", err)
	}
}
