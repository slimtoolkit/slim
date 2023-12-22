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

	"github.com/slimtoolkit/slim/pkg/app/sensor/standalone/control"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/report"
	testsensor "github.com/slimtoolkit/slim/pkg/test/e2e/sensor"
	testutil "github.com/slimtoolkit/slim/pkg/test/util"
)

const (
	imageSimpleService = "docker.io/library/nginx:1.21"
	imageSimpleCLI     = "docker.io/library/alpine:3.16.2"
)

var (
	sensorFullLifecycleSequence = []string{
		"sensor: ver=",
		"sensor: creating monitors...",
		"sensor: starting monitors...",
		"sensor: run finished succesfully",
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

	sensor.AssertSensorEventsFileContains(t, ctx,
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

func TestAppStdoutToFile(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleCLI,
		testsensor.WithSensorLogsToFile(),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(
		t, ctx,
		[]string{"sh", "-c", "echo 123456789; echo 987654321 >&2"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppStdoutToFile(),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "123456789")
	sensor.AssertTargetAppLogsContain(t, ctx, "987654321")
	sensor.AssertTargetAppStdoutFileEqualsTo(t, ctx, "123456789")
}

func TestAppStderrToFile(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleCLI,
		testsensor.WithSensorLogsToFile(),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(
		t, ctx,
		[]string{"sh", "-c", "echo 123456789; echo 987654321 >&2"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppStderrToFile(),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "123456789")
	sensor.AssertTargetAppLogsContain(t, ctx, "987654321")
	sensor.AssertTargetAppStderrFileEqualsTo(t, ctx, "987654321")
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
		testsensor.WithSensorLifecycleHook("echo"),
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
		testsensor.WithSensorLifecycleHook("echo"),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, []string{"cat", "/etc/alpine-release"})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertTargetAppLogsContain(t, ctx, "3.16.2")

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertSensorLogsContain(t, ctx, sensorLifecycleHookSequence...)
}

func TestRunTargetAsUser(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleCLI)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(
		t, ctx, []string{"whoami"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithAppUser("daemon"),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx, "daemon")
}

func TestTargetAppEnvVars(t *testing.T) {
	cases := []struct {
		image string
		user  string
		home  string
	}{
		// nixery.dev/shell lacks the /etc/passwd file: all UIDs should end up with HOME=/
		{image: "nixery.dev/shell", user: "0", home: "/"},
		{image: "nixery.dev/shell", user: "65534", home: "/"},

		// Alpine
		{image: imageSimpleCLI, home: "/root"},
		{image: imageSimpleCLI, user: "root", home: "/root"},
		{image: imageSimpleCLI, user: "0", home: "/root"},
		{image: imageSimpleCLI, user: "nobody", home: "/"},
		{image: imageSimpleCLI, user: "65534", home: "/"}, // nobody's UID
		{image: imageSimpleCLI, user: "bin", home: "/bin"},
		{image: imageSimpleCLI, user: "1", home: "/bin"}, // bin's UID
		{image: imageSimpleCLI, user: "daemon", home: "/sbin"},
		{image: imageSimpleCLI, user: "2", home: "/sbin"}, // daemon's UID
		{image: imageSimpleCLI, user: "nosuchuser", home: "/"},
		{image: imageSimpleCLI, user: "14567", home: "/"}, // hopefully, no such UID

		// Nginx
		{image: imageSimpleService, user: "nginx", home: "/nonexistent"},
		{image: imageSimpleService, user: "101", home: "/nonexistent"}, // nginx's UID
		{image: imageSimpleService, user: "nobody", home: "/"},
		{image: imageSimpleService, user: "65534", home: "/"}, // nobody's UID
		{image: imageSimpleService, user: "daemon", home: "/usr/sbin"},
		{image: imageSimpleService, user: "1", home: "/usr/sbin"}, // daemon's UID
		{image: imageSimpleService, user: "nosuchuser", home: "/"},
		{image: imageSimpleService, user: "14567", home: "/"}, // hopefully, no such UID
	}

	for _, tcase := range cases {
		func() {
			runID := newTestRun(t)
			ctx := context.Background()

			sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, tcase.image)
			defer sensor.Cleanup(t, ctx)

			var startOpts []testsensor.StartMonitorOpt
			if len(tcase.user) > 0 {
				startOpts = append(startOpts, testsensor.WithAppUser(tcase.user))
			}
			sensor.StartStandaloneOrFail(
				t, ctx, []string{"env"},
				testsensor.NewMonitorStartCommand(startOpts...),
			)
			sensor.WaitOrFail(t, ctx)

			sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
			sensor.AssertTargetAppLogsContain(t, ctx, "HOME="+tcase.home)
		}()
	}
}

func TestArchiveArtifacts_HappyPath(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleCLI,
		testsensor.WithSensorLogsToFile(),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(
		t, ctx,
		[]string{"cat", "/etc/alpine-release"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppStdoutToFile(),
			testsensor.WithAppStderrToFile(),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.DownloadArtifactsOrFail(t, ctx)

	sensor.AssertArtifactsArchiveContains(t, ctx,
		report.DefaultContainerReportFileName,
		testsensor.EventsFileName,
		testsensor.CommandsFileName,
		testsensor.SensorLogFileName,
		testsensor.AppStdoutFileName,
		testsensor.AppStderrFileName,
	)
}

func TestArchiveArtifacts_CustomLocation(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleCLI,
		testsensor.WithSensorLogsToFile(),
		testsensor.WithSensorArtifactsDir("/opt/not-dockerslim-at-all/files"),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(
		t, ctx,
		[]string{"cat", "/etc/alpine-release"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppStdoutToFile(),
			testsensor.WithAppStderrToFile(),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.DownloadArtifactsOrFail(t, ctx)

	sensor.AssertArtifactsArchiveContains(t, ctx,
		report.DefaultContainerReportFileName,
		testsensor.EventsFileName,
		testsensor.CommandsFileName,
		testsensor.SensorLogFileName,
		testsensor.AppStdoutFileName,
		testsensor.AppStderrFileName,
	)
}

func TestArchiveArtifacts_SensorFailure_NoCaps(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleCLI,
		testsensor.WithSensorLogsToFile(),
		testsensor.WithSensorCapabilities(), // Cancels out the default --cap-add=ALL.
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(
		t, ctx,
		[]string{"cat", "/etc/alpine-release"},
		testsensor.NewMonitorStartCommand(
			testsensor.WithSaneDefaults(),
			testsensor.WithAppStdoutToFile(),
			testsensor.WithAppStderrToFile(),
			testsensor.WithAppStderrToFile(),
		),
	)
	sensor.WaitOrFail(t, ctx)

	sensor.DownloadArtifactsOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, []string{
		"sensor: creating monitors...",
		"sensor: starting monitors...",
		"sensor: composite monitor - FAN failed to start running", // <-- failure!
		"sensor: run finished with error",
	}...)

	sensor.AssertArtifactsArchiveContains(t, ctx,
		testsensor.EventsFileName,
		testsensor.CommandsFileName,
		testsensor.SensorLogFileName,
	)
}

func TestArchiveArtifacts_SensorFailure_NoRoot(t *testing.T) {
	// It's a fairly common failure scenario.
	t.Skip("Implement me!")
}

func TestStopSignal_ForceKill(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	wrongStopSignal := syscall.SIGUSR1 // This signal isn't going to make Nginx exit.
	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleService,
		testsensor.WithStopSignal(wrongStopSignal), // Emulate misconfiguration.
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, nil)
	go testutil.Delayed(ctx, 5*time.Second, func() {
		// However, the sensor will terminate the target app
		// anyway, when the grace period after receiving the
		// stop signal is over.
		sensor.SignalOrFail(t, ctx, wrongStopSignal)
	})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
	sensor.AssertTargetAppLogsContain(t, ctx,
		"nginx/1.21",
		"start worker processes",
		"sensor: stop signal was sent to target app - starting grace period",
		"sensor: grace timeout expired - SIGKILL goes to target app",
	)

	sensor.DownloadArtifactsOrFail(t, ctx)
	sensor.AssertReportIncludesFiles(t,
		"/etc/nginx/nginx.conf",
		"/etc/nginx/conf.d/default.conf",
		"/var/cache/nginx",
		"/var/run",
		// Because the target app was terminated by SIGKILL,
		// the pid file is not cleaned up.
		"/run/nginx.pid",
	)
	sensor.AssertReportNotIncludesFiles(t,
		"/bin/bash",
		"/bin/cat",
		"/etc/apt/sources.list",
	)
}

func TestControlCommands_StopTargetApp(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(t, ctx, t.TempDir(), runID, imageSimpleService)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, nil)

	go testutil.Delayed(ctx, 5*time.Second, func() {
		sensor.ExecuteControlCommandOrFail(t, ctx, control.StopTargetAppCommand)
		sensor.WaitForEventOrFail(t, ctx, event.StopMonitorDone)
		sensor.WaitForEventOrFail(t, ctx, event.ShutdownSensorDone)

		// In the real world, there might be some (long) time between
		// the stop command and the target app signalling - maybe
		// we need to simulate that here?
		sensor.SignalOrFail(t, ctx, syscall.SIGQUIT)
	})

	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)
}

func TestEnableMondel(t *testing.T) {
	runID := newTestRun(t)
	ctx := context.Background()

	sensor := testsensor.NewSensorOrFail(
		t, ctx, t.TempDir(), runID, imageSimpleService,
		testsensor.WithEnableMondel(),
	)
	defer sensor.Cleanup(t, ctx)

	sensor.StartStandaloneOrFail(t, ctx, nil)
	go testutil.Delayed(ctx, 5*time.Second, func() {
		sensor.SignalOrFail(t, ctx, syscall.SIGTERM)
	})
	sensor.WaitOrFail(t, ctx)

	sensor.AssertSensorLogsContain(t, ctx, sensorFullLifecycleSequence...)

	sensor.DownloadArtifactsOrFail(t, ctx)

	sensor.AssertMondelIncludesFiles(t,
		"/etc/nginx/nginx.conf",
		"/etc/nginx/conf.d/default.conf",

		// TODO: investigate why these files are not included in the mondel (but are in the creport).
		// "/bin/sh",
		// "/var/cache/nginx",
		// "/var/run",
	)
	sensor.AssertMondelNotIncludesFiles(t,
		"/bin/bash",
		"/bin/cat",
		"/etc/apt/sources.list",

		// TODO: investigate why this file is included in the mondel (but not in the creport).
		// "/run/nginx.pid",
	)

	// Uncomment when the mondel and creport file sets are synced.
	// sensor.AssertReportAndMondelFileListsMatch(t)
}
