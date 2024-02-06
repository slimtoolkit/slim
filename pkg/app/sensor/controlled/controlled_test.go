package controlled_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slimtoolkit/slim/pkg/app/sensor/artifact"
	"github.com/slimtoolkit/slim/pkg/app/sensor/controlled"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor/fanotify"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor/ptrace"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/test/stub/sensor/execution"
	stubmonitor "github.com/slimtoolkit/slim/pkg/test/stub/sensor/monitor"
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
		del mondel.Publisher,
		artifactsDir string,
		mountPoint string,
		origPaths map[string]struct{},
	) (monitor.CompositeMonitor, error) {
		return monitor.Compose(
			cmd,
			nil,
			fanMon,
			ptMon,
			nil,
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

func (a *artifactorStub) Process(
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
		nil, //Monitor Data Event Log
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
		nil, //Monitor Data Event Log
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
		nil, //Monitor Data Event Log
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
		nil, //Monitor Data Event Log
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
