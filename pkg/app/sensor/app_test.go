//go:build e2e
// +build e2e

package sensor_test

import (
	"context"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	testsensor "github.com/docker-slim/docker-slim/pkg/test/e2e/sensor"
	testutil "github.com/docker-slim/docker-slim/pkg/test/util"
)

const (
	imageSimpleService = "docker.io/library/nginx:1.21"
	imageSimpleCLI     = "docker.io/library/alpine:3.16.2"
)

var (
	sensorFullLifecycleSequence = []string{
		"sensor: uid=0 euid=0",
		"sensor: creating monitors...",
		"sensor: starting monitors...",
		"fanmon: Start",
		"ptmon: Start",
		"sensor: monitor - saving report",
		"sensor: monitor - saving report",
	}

	sensorLifecycleHookSequence = []string{
		"sensor-post-start",
		"monitor-pre-start",
		"monitor-post-shutdown",
		"sensor-pre-shutdown",
	}
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestSimpleSensorRun_Controlled_CLI(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleCLI)
	defer sensor.Cleanup(t, ctx)

	sensor.StartControlledOrFail(t, ctx)

	sensor.SendStartCommandOrFail(t, ctx,
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppNameArgs("cat", "/etc/alpine-release"),
		),
	)
	sensor.ExpectEvent(t, event.StartMonitorDone)

	time.Sleep(1 * time.Second)

	sensor.SendStopCommandOrFail(t, ctx)
	sensor.ExpectEvent(t, event.StopMonitorDone)

	sensor.ShutdownOrFail(t, ctx)
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t, "/bin/cat", "/bin/busybox", "/etc/alpine-release")
	sensor.AssertReportNotIncludesFiles(t, "/bin/echo2", "/etc/resolve.conf")
}

func TestSimpleSensorRun_Controlled_Service(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleService)
	defer sensor.Cleanup(t, ctx)

	sensor.StartControlledOrFail(t, ctx)

	sensor.SendStartCommandOrFail(t, ctx)
	sensor.ExpectEvent(t, event.StartMonitorDone)

	time.Sleep(5 * time.Second)

	sensor.SendStopCommandOrFail(t, ctx)
	sensor.ExpectEvent(t, event.StopMonitorDone)

	sensor.ShutdownOrFail(t, ctx)
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx,
		"nginx/1.21",
		"start worker processes",
	)

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t,
		"/bin/sh",
		"/etc/nginx/nginx.conf",
		"/etc/nginx/conf.d/default.conf",
		"/var/cache/nginx",
		"/var/run",
		// Here is an interesting one - in the controlled (default) mode, sensor doesn't
		// await the target process termination. Hence, no cleanup on the nginx side
		// happens, and the pid file remains in the report.
		"/run/nginx.pid",
	)
	sensor.AssertReportNotIncludesFiles(t,
		"/bin/bash",
		"/bin/cat",
		"/etc/apt/sources.list",
	)
}

func TestSimpleSensorRun_Standalone_CLI(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleCLI)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, []string{"cat", "/etc/alpine-release"})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t, "/bin/cat", "/bin/busybox", "/etc/alpine-release")
	sensor.AssertReportNotIncludesFiles(t, "/bin/echo2", "/etc/resolve.conf")

	sensor.AssertSensorEventFileContains(t, ctx,
		event.StartMonitorDone,
		event.StopMonitorDone,
		event.ShutdownSensorDone,
	)
}

func TestSimpleSensorRun_Standalone_Service(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleService)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, nil)
	go testutil.Delayed(ctx, 5*time.Second, func() {
		sensor.SignalOrFail(t, ctx, syscall.SIGTERM)
	})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx,
		"nginx/1.21",
		"start worker processes",
		"(SIGTERM) received from 1, exiting",
	)

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t,
		"/bin/sh",
		"/etc/nginx/nginx.conf",
		"/etc/nginx/conf.d/default.conf",
		"/var/cache/nginx",
		"/var/run",
	)
	sensor.AssertReportNotIncludesFiles(t,
		"/bin/bash",
		"/bin/cat",
		"/etc/apt/sources.list",
		// Here is an interesting one - in the standalone mode sensor
		// tries gracefully terminate the target process by forwarding
		// it the StopSignal from it receives from the runtime. Nginx
		// exits and cleans up its pid file.
		"/run/nginx.pid",
	)
}

func TestSensorLogsGoToFile(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleCLI,
		testsensor.WithSensorLogsToFile(),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, []string{"echo", "123456879"})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertTargetAppLogsEqualTo(t, ctx, "123456879")

	// When WithSensorLogsToFile() is used, sensor's logs become part of artifacts.
	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
}

func TestAccessedButThenDeletedFilesShouldBeReported(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	t.Skip("Fix for the sensor's logic is required!")

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleCLI)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, []string{
		"sh", "-c", "cat /etc/alpine-release; rm /etc/alpine-release",
	})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t, "/etc/alpine-release")
}

func TestPreservedPathsWorkWithFilesDeletedDuringProbing(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	t.Skip("Fix for the sensor's logic is required!")

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleCLI)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx,
		[]string{"sh", "-c", "cat /etc/alpine-release; rm /etc/alpine-release"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithPreserves("/etc/alpine-release"),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t, "/etc/alpine-release")
}

func newTestRun(t *testing.T) string {
	runID := t.Name() + "-" + strings.SplitN(uuid.New().String(), "-", 2)[0]
	log.Debugf("New test run %s", runID)
	return runID
}

func TestLifecycleHook_Controlled_CLI(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t,
		ctx,
		t.TempDir(),
		runID,
		imageSimpleCLI,
		testsensor.WithLifecycleHook("echo"),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartControlledOrFail(t, ctx)

	sensor.SendStartCommandOrFail(t, ctx,
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppNameArgs("cat", "/etc/alpine-release"),
		),
	)

	time.Sleep(1 * time.Second)

	sensor.SendStopCommandOrFail(t, ctx)

	sensor.ShutdownOrFail(t, ctx)
	sensor.WaitOrFail(t, ctx)

	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertSensorLogsContain(t, ctx, sensorLifecycleHookSequence...)
}

func TestLifecycleHook_Standalone_CLI(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t,
		ctx,
		t.TempDir(),
		runID,
		imageSimpleCLI,
		testsensor.WithLifecycleHook("echo"),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, []string{"cat", "/etc/alpine-release"})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertSensorLogsContain(t, ctx, sensorLifecycleHookSequence...)
}
