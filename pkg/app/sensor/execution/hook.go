package execution

import (
	"context"
	"os/exec"

	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	log "github.com/sirupsen/logrus"
)

type kind string

const (
	sensorPostStart     kind = "sensor-post-start"
	sensorPreShutdown   kind = "sensor-pre-shutdown"
	monitorPreStart     kind = "monitor-pre-start"
	monitorPostShutdown kind = "monitor-post-shutdown"
	monitorFailed       kind = "monitor-failed"
)

type hookExecutor struct {
	ctx context.Context
	cmd string
}

func (h *hookExecutor) HookSensorPostStart() {
	h.doHook(sensorPostStart)
}

func (h *hookExecutor) HookSensorPreShutdown() {
	h.doHook(sensorPreShutdown)
}

func (h *hookExecutor) HookMonitorPreStart() {
	h.doHook(monitorPreStart)
}

func (h *hookExecutor) HookMonitorPostShutdown() {
	h.doHook(monitorPostShutdown)
}

func (h *hookExecutor) HookMonitorFailed() {
	h.doHook(monitorFailed)
}

func (h *hookExecutor) doHook(k kind) {
	if len(h.cmd) == 0 {
		return
	}

	cmd := exec.CommandContext(h.ctx, h.cmd, string(k))
	out, err := cmd.CombinedOutput()

	logger := log.
		WithField("command", h.cmd).
		WithField("exit_code", cmd.ProcessState.ExitCode()).
		WithField("output", string(out))

	// Some lifecycle hooks are really fast - hence, the IsNoChildProcesses() check.
	if err == nil || errutil.IsNoChildProcesses(err) {
		logger.Debugf("sensor: %s hook succeeded", k)
	} else {
		logger.WithError(err).Warnf("sensor: %s hook failed", k)
	}
}
