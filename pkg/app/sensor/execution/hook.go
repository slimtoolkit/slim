package execution

import (
	"context"
	"os/exec"

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
	if err == nil {
		log.
			WithField("kind", k).
			WithField("output", string(out)).
			Info("lifecycle hook command succeeded")
	} else {
		log.
			WithError(err).
			WithField("kind", k).
			WithField("output", string(out)).
			Info("lifecycle hook command failed")
	}
}
