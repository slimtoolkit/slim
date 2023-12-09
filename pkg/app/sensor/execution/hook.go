package execution

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/acounter"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
)

type kind string

const (
	sensorPostStart     kind = "sensor-post-start"
	sensorPreShutdown   kind = "sensor-pre-shutdown"
	monitorPreStart     kind = "monitor-pre-start"
	targetAppRunning    kind = "target-app-running"
	monitorPostShutdown kind = "monitor-post-shutdown"
	monitorFailed       kind = "monitor-failed"
)

// todo:
// add 'kind' to the lifecycle event and
// pass the whole event to the lifecycle hook command
type LifecycleEvent struct {
	Timestamp int64  `json:"ts"`
	SeqNumber uint64 `json:"sn"`
}

type hookExecutor struct {
	ctx       context.Context
	cmd       string
	lastHook  kind
	seqNumber acounter.Type
}

func (h *hookExecutor) State() string {
	return fmt.Sprintf("%s/%s", h.cmd, string(h.lastHook))
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

func (h *hookExecutor) HookTargetAppRunning() {
	h.doHook(targetAppRunning)
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

	event := LifecycleEvent{
		Timestamp: time.Now().UTC().UnixNano(),
		SeqNumber: h.seqNumber.Inc(),
	}

	h.lastHook = k
	//todo: pass event as a base64 encoded json string as an extra param to 'cmd'
	cmd := exec.CommandContext(h.ctx, h.cmd, string(k))
	out, err := cmd.CombinedOutput()

	logger := log.
		WithField("kind", string(k)).
		WithField("event", fmt.Sprintf("%+v", event)).
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
