package ptrace

import (
	"github.com/slimtoolkit/slim/pkg/monitor/ptrace"
	"github.com/slimtoolkit/slim/pkg/report"
)

type AppRunOpt = ptrace.AppRunOpt

type Monitor interface {
	// Starts the long running monitoring. The method itself is not
	// blocking and not reentrant!
	Start() error

	// Cancels the underlying ptrace execution context but doesn't
	// make the current monitor done immediately. You still need to await
	// the final cleanup with <-mon.Done() before accessing the status.
	Cancel()

	// With Done clients can await for the monitoring completion.
	// The method is reentrant - every invocation returns the same
	// instance of the channel.
	Done() <-chan struct{}

	Status() (*report.PtMonitorReport, error)
}
