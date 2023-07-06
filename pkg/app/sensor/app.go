//go:build linux
// +build linux

package sensor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/artifact"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/controlled"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/execution"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitor"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/standalone"
	"github.com/docker-slim/docker-slim/pkg/appbom"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/sysenv"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/version"
)

const (
	// Execution modes
	sensorModeControlled = "controlled"
	sensorModeStandalone = "standalone"

	// Flags

	getAppBomFlagUsage   = "get sensor application BOM"
	getAppBomFlagDefault = false

	enableDebugFlagUsage   = "enable debug logging"
	enableDebugFlagDefault = false

	logLevelFlagUsage   = "set the logging level ('debug', 'info', 'warn', 'error', 'fatal', 'panic')"
	logLevelFlagDefault = "info"

	logFormatFlagUsage   = "set the logging format ('text', or 'json')"
	logFormatFlagDefault = "text"

	logFileFlagUsage   = "enable logging redirection to a file (allowing to keep sensor's output separate from the target app's output)"
	logFileFlagDefault = ""

	sensorModeFlagUsage   = "set the sensor execution mode ('controlled' when sensor expect the driver docker-slim app to manipulate its lifecycle; or 'standalone' when sensor depends on nothing but the target app"
	sensorModeFlagDefault = sensorModeControlled

	commandsFileFlagUsage   = "provide a JSONL-encoded file with one ore more sensor commands (standalone mode only)"
	commandsFileFlagDefault = "/opt/_slim/commands.json"

	lifecycleHookCommandFlagUsage   = "set path to an executable that'll be invoked at various sensor lifecycle events (post-start, pre-shutdown, etc)"
	lifecycleHookCommandFlagDefault = ""

	// Should stopSignal and stopGracePeriod become StartMonitor
	// command's fields instead? Hypothetically, in a multi-command
	// monitoring run, these two params may have different values.
	stopSignalFlagUsage   = "set the signal to stop the target app (and, eventually, the sensor)"
	stopSignalFlagDefault = "TERM"

	stopGracePeriodFlagUsage   = "set the time to wait for the graceful termination of the target app (before sensor SIGKILL's it)"
	stopGracePeriodFlagDefault = 5 * time.Second

	artifactsDirFlagUsage   = "output director for all sensor artifacts"
	artifactsDirFlagDefault = app.DefaultArtifactsDirPath
)

var (
	getAppBom            *bool          = flag.Bool("appbom", getAppBomFlagDefault, getAppBomFlagUsage)
	enableDebug          *bool          = flag.Bool("debug", enableDebugFlagDefault, enableDebugFlagUsage)
	logLevel             *string        = flag.String("log-level", logLevelFlagDefault, logLevelFlagUsage)
	logFormat            *string        = flag.String("log-format", logFormatFlagDefault, logFormatFlagUsage)
	logFile              *string        = flag.String("log-file", logFileFlagDefault, logFileFlagUsage)
	sensorMode           *string        = flag.String("mode", sensorModeFlagDefault, sensorModeFlagUsage)
	commandsFile         *string        = flag.String("command-file", commandsFileFlagDefault, commandsFileFlagUsage)
	artifactsDir         *string        = flag.String("artifacts-dir", artifactsDirFlagDefault, artifactsDirFlagUsage)
	lifecycleHookCommand *string        = flag.String("lifecycle-hook", lifecycleHookCommandFlagDefault, lifecycleHookCommandFlagUsage)
	stopSignal           *string        = flag.String("stop-signal", stopSignalFlagDefault, stopSignalFlagUsage)
	stopGracePeriod      *time.Duration = flag.Duration("stop-grace-period", stopGracePeriodFlagDefault, stopGracePeriodFlagUsage)

	errUnknownMode = errors.New("unknown sensor mode")
)

func init() {
	flag.BoolVar(getAppBom, "b", getAppBomFlagDefault, getAppBomFlagUsage)
	flag.BoolVar(enableDebug, "d", enableDebugFlagDefault, enableDebugFlagUsage)
	flag.StringVar(logLevel, "l", logLevelFlagDefault, logLevelFlagUsage)
	flag.StringVar(logFormat, "f", logFormatFlagDefault, logFormatFlagUsage)
	flag.StringVar(logFile, "o", logFileFlagDefault, logFileFlagUsage)
	flag.StringVar(sensorMode, "m", sensorModeFlagDefault, sensorModeFlagUsage)
	flag.StringVar(commandsFile, "c", commandsFileFlagDefault, commandsFileFlagUsage)
	flag.StringVar(artifactsDir, "e", artifactsDirFlagDefault, artifactsDirFlagUsage)
	flag.StringVar(lifecycleHookCommand, "a", lifecycleHookCommandFlagDefault, lifecycleHookCommandFlagUsage)
	flag.StringVar(stopSignal, "s", stopSignalFlagDefault, stopSignalFlagUsage)
	flag.DurationVar(stopGracePeriod, "w", stopGracePeriodFlagDefault, stopGracePeriodFlagUsage)
}

func dumpAppBom() {
	info := appbom.Get()
	if info == nil {
		return
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(" ", " ")
	_ = encoder.Encode(info)
	fmt.Printf("%s\n", out.String())
}

// Run starts the sensor app
func Run() {
	flag.Parse()

	if *getAppBom {
		dumpAppBom()
		return
	}

	errutil.FailOn(configureLogger(*enableDebug, *logLevel, *logFormat, *logFile))

	activeCaps, maxCaps, err := sysenv.Capabilities(0)
	errutil.WarnOn(err)
	log.Infof("sensor: ver=%v", version.Current())
	log.Debugf("sensor: args => %#v", os.Args)

	log.Tracef("sensor: uid=%v euid=%v", os.Getuid(), os.Geteuid())
	log.Tracef("sensor: privileged => %v", sysenv.IsPrivileged())
	log.Tracef("sensor: active capabilities => %#v", activeCaps)
	log.Tracef("sensor: max capabilities => %#v", maxCaps)
	log.Tracef("sensor: sysinfo => %#v", system.GetSystemInfo())
	log.Tracef("sensor: kernel flags => %#v", system.DefaultKernelFeatures.Raw)

	var artifactsExtra []string
	if len(*commandsFile) > 0 {
		artifactsExtra = append(artifactsExtra, *commandsFile)
	}
	if len(*logFile) > 0 {
		artifactsExtra = append(artifactsExtra, *logFile)
	}
	artifactor := artifact.NewProcessor(*artifactsDir, artifactsExtra)

	ctx := context.Background()
	exe, err := newExecution(
		ctx,
		*sensorMode,
		*commandsFile,
		filepath.Join(*artifactsDir, "events.json"),
		*lifecycleHookCommand,
	)
	if err != nil {
		errutil.WarnOn(artifactor.Archive())
		errutil.FailOn(err) // calls os.Exit(1)
	}

	// There is a number of errutil.FailOn() below, so no way to rely on defer:
	// We need to make sure `exe` is closed before archiving - otherwise some
	// artifacts might be missing due to the non-flushed buffers!

	exe.HookSensorPostStart()

	sen, err := newSensor(ctx, exe, *sensorMode, artifactor)
	if err != nil {
		exe.Close()
		errutil.WarnOn(artifactor.Archive())
		errutil.FailOn(err) // calls os.Exit(1)
	}

	if err := sen.Run(); err != nil {
		exe.PubEvent(event.Error, err.Error())
		log.WithError(err).Error("sensor: run finished with error")
		if errors.Is(err, monitor.ErrInsufficientPermissions) {
			log.Info("sensor: Instrumented containers require root and ALL capabilities enabled. Example: `docker run --user root --cap-add ALL app:v1-instrumented`")
		}
		if errors.Is(err, monitor.ErrInsufficientPermissions) {
		}
	} else {
		log.Info("sensor: run finished succesfully")
	}

	exe.Close()
	errutil.WarnOn(artifactor.Archive())

	// We have to "stop" the execution and dump the artifacts
	// before calling the pre-shutdown hook (that may want to
	// upload the artifacts somewhere).
	// Not ideal calling it after exe.Close() but should be safe.
	exe.HookSensorPreShutdown()
	log.Info("sensor: exiting...")
}

func newExecution(
	ctx context.Context,
	mode string,
	commandsFile string,
	eventsFile string,
	lifecycleHookCommand string,
) (execution.Interface, error) {
	switch mode {
	case sensorModeControlled:
		return execution.NewControlled(ctx, lifecycleHookCommand)
	case sensorModeStandalone:
		return execution.NewStandalone(
			ctx,
			commandsFile,
			eventsFile,
			lifecycleHookCommand,
		)
	}

	return nil, errUnknownMode
}

type sensor interface {
	Run() error
}

func newSensor(
	ctx context.Context,
	exe execution.Interface,
	mode string,
	artifactor artifact.Processor,
) (sensor, error) {
	workDir, err := os.Getwd()
	errutil.WarnOn(err)
	log.Debugf("sensor: cwd => %s", workDir)

	mountPoint := "/"
	log.Debugf("sensor: mount point => %s", mountPoint)

	switch mode {
	case sensorModeControlled:
		ctx, cancel := context.WithCancel(ctx)

		// To preserve the backward compatibility, don't forward
		// signals to the target app in the default (controlled) mode.
		startSystemSignalsMonitor(func() {
			cancel()
			time.Sleep(2 * time.Second)
		})

		return controlled.NewSensor(
			ctx,
			exe,
			monitor.NewCompositeMonitor,
			artifactor,
			workDir,
			mountPoint,
		), nil
	case sensorModeStandalone:
		return standalone.NewSensor(
			ctx,
			exe,
			monitor.NewCompositeMonitor,
			artifactor,
			workDir,
			mountPoint,
			signalFromString(*stopSignal),
			*stopGracePeriod,
		), nil
	}

	exe.PubEvent(event.StartMonitorFailed, errUnknownMode.Error())
	return nil, errUnknownMode
}
