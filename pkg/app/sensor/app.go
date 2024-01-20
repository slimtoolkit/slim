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

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/sensor/artifact"
	"github.com/slimtoolkit/slim/pkg/app/sensor/controlled"
	"github.com/slimtoolkit/slim/pkg/app/sensor/execution"
	"github.com/slimtoolkit/slim/pkg/app/sensor/monitor"
	"github.com/slimtoolkit/slim/pkg/app/sensor/standalone"
	"github.com/slimtoolkit/slim/pkg/app/sensor/standalone/control"
	"github.com/slimtoolkit/slim/pkg/appbom"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/sysenv"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/version"
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

	sensorModeFlagUsage   = "set the sensor execution mode ('controlled' when sensor expect the driver 'slim' app to manipulate its lifecycle; or 'standalone' when sensor depends on nothing but the target app"
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

	enableMondelFlagUsage   = "enable monitor data event logging"
	enableMondelFlagDefault = false
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
	enableMondel         *bool          = flag.Bool("mondel", enableMondelFlagDefault, enableMondelFlagUsage)

	errUnknownMode = errors.New("unknown sensor mode")
)

func eventsFilePath() string {
	return filepath.Join(*artifactsDir, "events.json")
}

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
	flag.BoolVar(enableMondel, "n", enableMondelFlagDefault, enableMondelFlagUsage)
}

// Run starts the sensor app
func Run() {
	flag.Parse()

	if *getAppBom {
		dumpAppBom()
		return
	}

	errutil.FailOn(configureLogger(*enableDebug, *logLevel, *logFormat, *logFile))
	ctx := context.Background()
	if len(os.Args) > 1 && os.Args[1] == "control" {
		if err := runControlCommand(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "Control command failed: "+err.Error())
			os.Exit(1)
		}
		return
	}

	activeCaps, maxCaps, err := sysenv.Capabilities(0)
	errutil.WarnOn(err)

	sr := &report.SensorReport{
		Version: version.Current(),
		Args:    os.Args,
	}

	log.Infof("sensor: ver=%v", sr.Version)
	log.Debugf("sensor: args => %#v", sr.Args)

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
	artifactor := artifact.NewProcessor(sr, *artifactsDir, artifactsExtra)

	exe, err := newExecution(
		ctx,
		*sensorMode,
		*commandsFile,
		eventsFilePath(),
		*lifecycleHookCommand,
	)
	if err != nil {
		errutil.WarnOn(artifactor.Archive())
		errutil.FailOn(err) // calls os.Exit(1)
	}

	mondelFile := filepath.Join(*artifactsDir, report.DefaultMonDelFileName)
	del := mondel.NewPublisher(ctx, *enableMondel, mondelFile)

	sen, err := newSensor(ctx, exe, *sensorMode, artifactor, del, *artifactsDir)
	if err != nil {
		exe.Close()
		errutil.WarnOn(artifactor.Archive())
		errutil.FailOn(err) // calls os.Exit(1)
	}

	if err := sen.Run(); err != nil {
		log.WithError(err).Error("sensor: run finished with error")
		if errors.Is(err, monitor.ErrInsufficientPermissions) {
			log.Info("sensor: Instrumented containers require root and ALL capabilities enabled. Example: `docker run --user root --cap-add ALL app:v1-instrumented`")
		}
	} else {
		log.Info("sensor: run finished succesfully")
	}

	exe.Close()

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
	del mondel.Publisher,
	artifactsDir string,
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
			del,
			artifactor,
			workDir,
			mountPoint,
		), nil
	case sensorModeStandalone:
		return standalone.NewSensor(
			ctx,
			exe,
			monitor.NewCompositeMonitor,
			del,
			artifactor,
			workDir,
			mountPoint,
			signalFromString(*stopSignal),
			*stopGracePeriod,
		), nil
	}

	exe.PubEvent(event.StartMonitorFailed,
		&event.StartMonitorFailedData{
			Component: event.ComSensorConstructor,
			State:     event.StateSensorTypeCreating,
			Context: map[string]string{
				event.CtxSensorType: mode,
			},
			Errors: []string{errUnknownMode.Error()},
		})
	return nil, errUnknownMode
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

// sensor control <stop-target-app|wait-for-event|change-log-level|...>
func runControlCommand(ctx context.Context) error {
	if len(os.Args) < 3 {
		return errors.New("missing command")
	}

	cmd := control.Command(os.Args[2])

	switch cmd {
	case control.StopTargetAppCommand:
		if err := control.ExecuteStopTargetAppCommand(ctx, *commandsFile); err != nil {
			return fmt.Errorf("error stopping target app: %w", err)
		}

	case control.WaitForEventCommand:
		if len(os.Args) < 4 {
			return errors.New("missing event name")
		}
		if err := control.ExecuteWaitEvenCommand(ctx, eventsFilePath(), event.Type(os.Args[3])); err != nil {
			return fmt.Errorf("error waiting for sensor event: %w", err)
		}

	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}

	return nil
}
