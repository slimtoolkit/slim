package monitor

import (
	"context"

	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor/fanotify"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor/ptrace"
	"github.com/slimtoolkit/slim/pkg/report"
)

// Base monitor stub.
type monitorStub struct {
	ctx      context.Context
	cancelFn context.CancelFunc
	errorCh  chan error
}

func newMonitorStub(ctx context.Context) *monitorStub {
	ctx, cancelFn := context.WithCancel(ctx)
	return &monitorStub{
		ctx:      ctx,
		cancelFn: cancelFn,
		errorCh:  make(chan error),
	}
}

func (m *monitorStub) Start() error {
	return nil
}

func (m *monitorStub) Cancel() {
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.cancelFn()
	close(m.errorCh)
}

func (m *monitorStub) Done() <-chan struct{} {
	return m.ctx.Done()
}

// fan monitor stub implements fanotify.Monitor
type FanMonitorStub struct {
	*monitorStub
}

var _ fanotify.Monitor = &FanMonitorStub{}

func NewFanMonitor(ctx context.Context) *FanMonitorStub {
	return &FanMonitorStub{
		monitorStub: newMonitorStub(ctx),
	}
}

func (m *FanMonitorStub) Status() (*report.FanMonitorReport, error) {
	return nil, nil
}

// ptrace monitor stub implements ptrace.Monitor
type PtMonitorStub struct {
	*monitorStub
}

var _ ptrace.Monitor = &PtMonitorStub{}

func NewPtMonitor(ctx context.Context) *PtMonitorStub {
	return &PtMonitorStub{
		monitorStub: newMonitorStub(ctx),
	}
}

func (m *PtMonitorStub) Status() (*report.PtMonitorReport, error) {
	return nil, nil
}
