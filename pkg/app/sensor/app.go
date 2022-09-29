//go:build linux
// +build linux

package sensor

import (
	"context"
	"errors"
	"flag"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/artifacts"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/controlled"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/execution"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/standalone"
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
	enableDebugFlagUsage   = "enable debug logging"
	enableDebugFlagDefault = false

	logLevelFlagUsage   = "set the logging level ('debug', 'info' (default), 'warn', 'error', 'fatal', 'panic')"
	logLevelFlagDefault = "info"

	logFormatFlagUsage   = "set the format used by logs ('text' (default), or 'json')"
	logFormatFlagDefault = "text"

	logFileFlagUsage   = "set the log redirection to a file (allowing to keep sensor's output separate from the target app's output)"
	logFileFlagDefault = ""

	sensorModeFlagUsage   = "set the sensor execution mode ('controlled' when sensor expect the driver docker-slim app to manipulate its lifecycle; or 'standalone' when sensor depends on nothing but the target app"
	sensorModeFlagDefault = sensorModeControlled

	commandFileFlagUsage   = "JSONL-encoded file with one ore more sensor commands"
	commandFileFlagDefault = app.DefaultArtifactDirPath + "/commands.json"

	lifecycleHookCommandFlagUsage   = "(optional) path to an executable that'll be invoked at various sensor lifecycle events (post-start, pre-shutdown, etc)"
	lifecycleHookCommandFlagDefault = ""

	stopSignalFlagUsage   = "signal to stop the target app and start producing the report"
	stopSignalFlagDefault = "TERM"

	stopGracePeriodFlagUsage   = "time to wait for the graceful termination of the target app (before sensor will send it SIGKILL)"
	stopGracePeriodFlagDefault = 5 * time.Second

	// Soon to become flags?
	defaultEventsFilePath = app.DefaultArtifactDirPath + "/events.json"
)

var (
	enableDebug          *bool          = flag.Bool("debug", enableDebugFlagDefault, enableDebugFlagUsage)
	logLevel             *string        = flag.String("log-level", logLevelFlagDefault, logLevelFlagUsage)
	logFormat            *string        = flag.String("log-format", logFormatFlagDefault, logFormatFlagUsage)
	logFile              *string        = flag.String("log-file", logFormatFlagDefault, logFileFlagUsage)
	sensorMode           *string        = flag.String("mode", sensorModeFlagDefault, sensorModeFlagUsage)
	commandFile          *string        = flag.String("command-file", commandFileFlagDefault, commandFileFlagUsage)
	lifecycleHookCommand *string        = flag.String("lifecycle-hook", lifecycleHookCommandFlagDefault, lifecycleHookCommandFlagUsage)
	stopSignal           *string        = flag.String("stop-signal", stopSignalFlagDefault, stopSignalFlagUsage)
	stopGracePeriod      *time.Duration = flag.Duration("stop-grace-period", stopGracePeriodFlagDefault, stopGracePeriodFlagUsage)

	errUnknownMode = errors.New("unknown sensor mode")
)

func init() {
	flag.BoolVar(enableDebug, "d", enableDebugFlagDefault, enableDebugFlagUsage)
	flag.StringVar(logLevel, "l", logLevelFlagDefault, logLevelFlagUsage)
	flag.StringVar(logFormat, "f", logFormatFlagDefault, logFormatFlagUsage)
	flag.StringVar(logFile, "o", "", logFormatFlagUsage)
	flag.StringVar(sensorMode, "m", sensorModeFlagDefault, sensorModeFlagUsage)
	flag.StringVar(commandFile, "c", commandFileFlagDefault, commandFileFlagUsage)
	flag.StringVar(lifecycleHookCommand, "a", lifecycleHookCommandFlagDefault, lifecycleHookCommandFlagUsage)
	flag.StringVar(stopSignal, "s", stopSignalFlagDefault, stopSignalFlagUsage)
	flag.DurationVar(stopGracePeriod, "w", stopGracePeriodFlagDefault, stopGracePeriodFlagUsage)
}

// Run starts the sensor app
func Run() {
	flag.Parse()

	errutil.FailOn(configureLogger(*enableDebug, *logLevel, *logFormat, *logFile))

	activeCaps, maxCaps, err := sysenv.Capabilities(0)
	errutil.WarnOn(err)
	log.Infof("sensor: ver=%v", version.Current())
	log.Debugf("sensor: uid=%v euid=%v", os.Getuid(), os.Geteuid())
	log.Debugf("sensor: privileged => %v", sysenv.IsPrivileged())
	log.Debugf("sensor: active capabilities => %#v", activeCaps)
	log.Debugf("sensor: max capabilities => %#v", maxCaps)
	log.Debugf("sensor: sysinfo => %#v", system.GetSystemInfo())
	log.Debugf("sensor: kernel flags => %#v", system.DefaultKernelFeatures.Raw)
	log.Debugf("sensor: args => %#v", os.Args)

	ctx := context.Background()
	exe := newExecution(ctx, *sensorMode, *commandFile, *lifecycleHookCommand)
	defer exe.Close()

	exe.HookSensorPostStart()

	sen := newSensor(ctx, exe, *sensorMode)
	if err := sen.Run(); err != nil {
		exe.PubEvent(event.Error, err.Error())
	}

	exe.HookSensorPreShutdown()

	log.Info("sensor: done!")
}

func newExecution(
	ctx context.Context,
	mode string,
	commandFile string,
	lifecycleHookCommand string,
) execution.Interface {
	if mode == sensorModeControlled {
		exe, err := execution.NewControlled(ctx, lifecycleHookCommand)
		errutil.FailOn(err)
		return exe
	}

	if mode == sensorModeStandalone {
		exe, err := execution.NewStandalone(
			ctx,
			commandFile,
			defaultEventsFilePath,
			lifecycleHookCommand,
		)
		errutil.FailOn(err)
		return exe
	}

	errutil.FailOn(errUnknownMode)
	return nil
}

type sensor interface {
	Run() error
}

func newSensor(
	ctx context.Context,
	exe execution.Interface,
	mode string,
) sensor {
	workDir, err := os.Getwd()
	errutil.WarnOn(err)
	log.Debugf("sensor: cwd => %s", workDir)

	mountPoint := "/"
	log.Debugf("sensor: mount point => %s", mountPoint)

	if mode == sensorModeControlled {
		ctx, cancel := context.WithCancel(ctx)

		// To preserve the backward compatibility, don't forward
		// signals to the target app in the default (controlled) mode.
		initSignalHandlers(func() {
			cancel()
			time.Sleep(2 * time.Second)
		})

		return controlled.NewSensor(
			ctx,
			exe,
			monitors.NewCompositeMonitor,
			artifacts.NewArtifactor(app.DefaultArtifactDirPath),
			workDir,
			mountPoint,
		)
	}

	if mode == sensorModeStandalone {
		return standalone.NewSensor(
			ctx,
			exe,
			monitors.NewCompositeMonitor,
			artifacts.NewArtifactor(app.DefaultArtifactDirPath),
			workDir,
			mountPoint,
			signalFromString(*stopSignal),
			*stopGracePeriod,
		)
	}

	exe.PubEvent(event.StartMonitorFailed, errUnknownMode.Error())
	errutil.FailOn(errUnknownMode)
	return nil
}
