//go:build linux
// +build linux

package sensor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app/sensor/ipc"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors/fanotify"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors/pevent"
	"github.com/docker-slim/docker-slim/pkg/app/sensor/monitors/ptrace"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/sysenv"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"golang.org/x/sys/unix"

	log "github.com/sirupsen/logrus"
)

const (
	defaultArtifactDirName = "/opt/dockerslim/artifacts"

	sensorModeControlled = "controlled"
	sensorModeStandalone = "standalone"
)

const (
	enableDebugFlagUsage   = "enable debug logging"
	enableDebugFlagDefault = false

	logLevelFlagUsage   = "set the logging level ('debug', 'info' (default), 'warn', 'error', 'fatal', 'panic')"
	logLevelFlagDefault = "info"

	logFormatFlagUsage   = "set the format used by logs ('text' (default), or 'json')"
	logFormatFlagDefault = "text"

	sensorModeFlagUsage   = "set the sensor execution mode ('controlled' when sensor expect the driver docker-slim app to manipulate its lifecycle; or 'standalone' when sensor depends on nothing but the target app"
	sensorModeFlagDefault = sensorModeControlled

	commandFileFlagUsage   = "JSONL-encoded file with one ore more sensor commands"
	commandFileFlagDefault = defaultArtifactDirName + "/commands.json"

	stopSignalFlagUsage   = "signal to stop the target app and start producing the report"
	stopSignalFlagDefault = "TERM"

	stopGracePeriodFlagUsage   = "time to wait for the graceful termination of the target app (before sensor will send it SIGKILL)"
	stopGracePeriodFlagDefault = 5 * time.Second
)

var (
	enableDebug     *bool          = flag.Bool("debug", enableDebugFlagDefault, enableDebugFlagUsage)
	logLevel        *string        = flag.String("log-level", logLevelFlagDefault, logLevelFlagUsage)
	logFormat       *string        = flag.String("log-format", logFormatFlagDefault, logFormatFlagUsage)
	sensorMode      *string        = flag.String("mode", sensorModeFlagDefault, sensorModeFlagUsage)
	commandFile     *string        = flag.String("command-file", commandFileFlagDefault, commandFileFlagUsage)
	stopSignal      *string        = flag.String("stop-signal", stopSignalFlagDefault, stopSignalFlagUsage)
	stopGracePeriod *time.Duration = flag.Duration("stop-grace-period", stopGracePeriodFlagDefault, stopGracePeriodFlagUsage)
)

func init() {
	flag.BoolVar(enableDebug, "d", enableDebugFlagDefault, enableDebugFlagUsage)
	flag.StringVar(logLevel, "l", logLevelFlagDefault, logLevelFlagUsage)
	flag.StringVar(logFormat, "f", logFormatFlagDefault, logFormatFlagUsage)
	flag.StringVar(sensorMode, "m", sensorModeFlagDefault, sensorModeFlagUsage)
	flag.StringVar(commandFile, "c", commandFileFlagDefault, commandFileFlagUsage)
	flag.StringVar(stopSignal, "s", stopSignalFlagDefault, stopSignalFlagUsage)
	flag.DurationVar(stopGracePeriod, "w", stopGracePeriodFlagDefault, stopGracePeriodFlagUsage)
}

// Run starts the sensor app
func Run() {
	flag.Parse()

	errutil.FailOn(
		configureLogger(*enableDebug, *logLevel, *logFormat),
	)

	activeCaps, maxCaps, err := sysenv.Capabilities(0)
	log.Debugf("sensor: uid=%v euid=%v", os.Getuid(), os.Geteuid())
	log.Debugf("sensor: privileged => %v", sysenv.IsPrivileged())
	log.Debugf("sensor: active capabilities => %#v", activeCaps)
	log.Debugf("sensor: max capabilities => %#v", maxCaps)
	log.Debugf("sensor: sysinfo => %#v", system.GetSystemInfo())
	log.Debugf("sensor: kernel flags => %#v", system.DefaultKernelFeatures.Raw)

	log.Infof("sensor: args => %#v", os.Args)

	dirName, err := os.Getwd()
	errutil.WarnOn(err)
	log.Debugf("sensor: cwd => %#v", dirName)

	sensorCtx, sensorCtxCancel := context.WithCancel(context.Background())
	defer func() {
		log.Debug("deferred cleanup on shutdown...")
		sensorCtxCancel()
	}()

	mountPoint := "/"

	switch *sensorMode {
	case sensorModeControlled:
		initSignalHandlers(sensorCtxCancel)

		runControlled(
			sensorCtx,
			dirName,
			mountPoint,
		)
	case sensorModeStandalone:
		runStandalone(
			sensorCtx,
			dirName,
			mountPoint,
			signalFromString(*stopSignal),
			*stopGracePeriod,
		)
	default:
		errutil.FailOn(errors.New("unknown sensor mode"))
	}

	log.Info("sensor: done!")
}

func runControlled(sensorCtx context.Context, dirName, mountPoint string) {
	log.Debug("sensor: starting IPC server...")
	ipcServer, err := ipc.NewServer(sensorCtx.Done())
	errutil.FailOn(err)
	errutil.FailOn(
		ipcServer.Run(),
	)

	errorChan := make(chan error)
	go func() {
		for {
			log.Debug("sensor: error collector - waiting for errors...")
			select {
			case <-sensorCtx.Done():
				log.Debug("sensor: error collector - done...")
				return
			case err := <-errorChan:
				log.Infof("sensor: error collector - forwarding error = %+v", err)
				ipcServer.TryPublishEvt(&event.Message{Name: event.Error, Data: err}, 3)
			}
		}
	}()

	// TODO: Do we need to forward signals to the target app in the controlled mode?
	signalChan := make(chan os.Signal)

	monitorCtx, monitorCtxCancel := context.WithCancel(sensorCtx)

	var startCmd *command.StartMonitor
	var monRes monResult

	log.Info("sensor: waiting for commands...")

	for {
		select {
		case cmd := <-ipcServer.CommandChan():
			log.Debug("\nsensor: command => ", cmd)

			switch typedCmd := cmd.(type) {
			case *command.StartMonitor:
				if typedCmd == nil {
					log.Info("sensor: 'start' monitor command - no data...")
					break
				}

				startCmd = typedCmd
				log.Debugf("sensor: 'start' monitor command (%#v)", startCmd)

				if startCmd.AppUser != "" {
					log.Debugf("sensor: 'start' monitor command - run app as user=%q", startCmd.AppUser)
				}

				if res, err := startMonitor(monitorCtx, startCmd, false, dirName, mountPoint, signalChan, errorChan); err != nil {
					log.Info("sensor: monitor not started...")
					monitorCtxCancel()
					ipcServer.TryPublishEvt(&event.Message{Name: event.StartMonitorFailed}, 3)
					time.Sleep(3 * time.Second) //give error event time to get sent
				} else {
					monRes = res

					//target app started by ptmon... (long story :-))
					//TODO: need to get the target app pid to pemon, so it can filter process events

					log.Infof("sensor: monitor started (%v)...")

					ipcServer.TryPublishEvt(&event.Message{Name: event.StartMonitorDone}, 3)

					log.Debug("sensor: monitor.worker - waiting to stop monitoring...")
				}

			case *command.StopMonitor:
				log.Info("sensor: 'stop' monitor command")

				monitorCtxCancel()

				log.Info("sensor: waiting for monitor to finish...")

				processReports(
					defaultArtifactDirName,
					startCmd,
					mountPoint,
					monRes.peReport(),
					monRes.fanReport(),
					monRes.ptReport(),
				)

				log.Info("sensor: monitor stopped...")
				ipcServer.TryPublishEvt(&event.Message{Name: event.StopMonitorDone}, 3)

			case *command.ShutdownSensor:
				log.Info("sensor: 'shutdown' command")
				ipcServer.TryPublishEvt(&event.Message{Name: event.ShutdownSensorDone}, 3)
				return // We're done!

			default:
				log.Info("sensor: ignoring unknown command => ", cmd)
			}

		case <-time.After(time.Second * 5):
			log.Debug(".")
		}
	}
}

func runStandalone(
	sensorCtx context.Context,
	dirName,
	mountPoint string,
	stopSignal os.Signal,
	stopGracePeriod time.Duration,
) {
	monitorCtx, monitorCtxCancel := context.WithCancel(sensorCtx)

	startCmd, err := readCommandFile()
	errutil.FailOn(err)

	errorChan := make(chan error)
	go func() {
		for {
			log.Debug("sensor: error collector - waiting for errors...")
			select {
			case <-sensorCtx.Done():
				log.Debug("sensor: error collector - done...")
				return
			case err := <-errorChan:
				log.Infof("sensor: error collector - forwarding error = %+v", err)
				log.Infof("sensor: error: %v", event.Message{Name: event.Error, Data: err})
			}
		}
	}()

	signalChan := initSignalForwardingChannel(monitorCtx, stopSignal, stopGracePeriod)
	monRes, err := startMonitor(monitorCtx, &startCmd, true, dirName, mountPoint, signalChan, errorChan)
	if err != nil {
		log.Info("sensor: monitor not started...")
		monitorCtxCancel()
		log.Infof("sensor: error: %v", event.Message{Name: event.StartMonitorFailed})
		return
	}

	// Wait until the monitored app is terminated.
	ptReport := monRes.ptReport()

	// Make other monitors stop by canceling their context(s).
	monitorCtxCancel()

	log.Info("sensor: target app is done.")

	processReports(
		defaultArtifactDirName,
		&startCmd,
		mountPoint,
		monRes.peReport(),
		monRes.fanReport(),
		ptReport,
	)
}

type monResult struct {
	peReportChan  <-chan *report.PeMonitorReport
	fanReportChan <-chan *report.FanMonitorReport
	ptReportChan  <-chan *report.PtMonitorReport
}

func (r *monResult) peReport() *report.PeMonitorReport {
	if r.peReportChan == nil {
		return nil
	}
	return <-r.peReportChan
}

func (r *monResult) fanReport() *report.FanMonitorReport {
	return <-r.fanReportChan
}

func (r *monResult) ptReport() *report.PtMonitorReport {
	return <-r.ptReportChan
}

func startMonitor(
	ctx context.Context,
	cmd *command.StartMonitor,
	standalone bool,
	dirName string,
	mountPoint string,
	signalChan <-chan os.Signal,
	errorChan chan<- error,
) (monResult, error) {
	res := monResult{}
	origPaths, err := getCurrentPaths("/")
	if err != nil {
		return res, err
	}

	log.Info("sensor: monitor starting...")

	//tmp: disable PEVENTs (due to problems with the new boot2docker host OS)
	usePEMon, err := system.DefaultKernelFeatures.IsCompiled("CONFIG_PROC_EVENTS")
	usePEMon = false
	if (err == nil) && usePEMon {
		log.Info("sensor: proc events are available!")
		res.peReportChan = pevent.Run(ctx.Done())
		//ProcEvents are not enabled in the default boot2docker kernel
	}

	prepareEnv(defaultArtifactDirName, cmd)

	res.fanReportChan = fanotify.Run(ctx, errorChan, mountPoint, cmd.IncludeNew, origPaths) //cmd.AppName, cmd.AppArgs
	if res.fanReportChan == nil {
		log.Info("sensor: startMonitor - FAN failed to start running...")
		return res, errors.New("FAN failed to start running")
	}

	log.Debugf("sensor: starting target app => %v %#v", cmd.AppName, cmd.AppArgs)

	appStartAckChan := make(chan bool, 3)
	res.ptReportChan = ptrace.Run(
		ctx,
		cmd.RTASourcePT,
		standalone,
		errorChan,
		appStartAckChan,
		signalChan,
		cmd.AppName,
		cmd.AppArgs,
		dirName,
		cmd.AppUser,
		cmd.RunTargetAsUser,
		cmd.IncludeNew,
		origPaths)
	if res.ptReportChan == nil {
		log.Info("sensor: startMonitor - PTAN failed to start running...")
		return res, errors.New("PTAN failed to start running")
	}

	log.Info("sensor: waiting for monitor to complete startup...")

	if !<-appStartAckChan {
		log.Info("sensor: startMonitor - PTAN failed to ack running...")
		return res, errors.New("PTAN failed to ack running")
	}

	return res, nil
}

func initSignalForwardingChannel(
	ctx context.Context,
	stopSignal os.Signal,
	stopGracePeriod time.Duration,
) <-chan os.Signal {
	signalChan := make(chan os.Signal, 10)

	go func() {
		log.Debug("sensor: starting forwarding signals to target app...")

		ch := make(chan os.Signal)
		signal.Notify(ch)

		for {
			select {
			case <-ctx.Done():
				log.Debug("sensor: forwarding signal to target app no more - monitor is done")
				return
			case s := <-ch:
				log.WithField("signal", s).Debug("sensor: forwarding signal to target app")
				signalChan <- s

				if s == stopSignal {
					log.Debug("sensor: recieved stop signal - starting grace period")

					// Starting the grace period
					select {
					case <-ctx.Done():
						log.Debug("sensor: monitor finished before grace period expired - dismantling SIGKILL")
					case <-time.After(stopGracePeriod):
						log.Debug("sensor: grace period expired - sending SIGKILL to target app")
						signalChan <- syscall.SIGKILL
					}
					return
				}
			}
		}
	}()

	return signalChan
}

func getCurrentPaths(root string) (map[string]interface{}, error) {
	pathMap := map[string]interface{}{}
	err := filepath.Walk(root,
		func(pth string, info os.FileInfo, err error) error {
			if strings.HasPrefix(pth, "/proc/") {
				log.Debugf("getCurrentPaths: skipping /proc file system objects...")
				return filepath.SkipDir
			}

			if strings.HasPrefix(pth, "/sys/") {
				log.Debugf("getCurrentPaths: skipping /sys file system objects...")
				return filepath.SkipDir
			}

			if strings.HasPrefix(pth, "/dev/") {
				log.Debugf("getCurrentPaths: skipping /dev file system objects...")
				return filepath.SkipDir
			}

			if info.Mode().IsRegular() &&
				!strings.HasPrefix(pth, "/proc/") &&
				!strings.HasPrefix(pth, "/sys/") &&
				!strings.HasPrefix(pth, "/dev/") {
				pth, err := filepath.Abs(pth)
				if err == nil {
					pathMap[pth] = nil
				}
			}
			return nil
		})

	if err != nil {
		return nil, err
	}

	return pathMap, nil
}

func signalFromString(s string) syscall.Signal {
	if !strings.HasPrefix(s, "SIG") {
		s = "SIG" + s
	}
	return unix.SignalNum(s)
}

// TODO: Make this function return a list of commands.
func readCommandFile() (command.StartMonitor, error) {
	var cmd command.StartMonitor

	file, err := os.Open(*commandFile)
	if err != nil {
		return cmd, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return cmd, errors.New("empty command file")
	}

	jsonCmd := scanner.Text()
	if err := scanner.Err(); err != nil {
		return cmd, fmt.Errorf("failed to read command file: %w", err)
	}

	if err := json.Unmarshal([]byte(jsonCmd), &cmd); err != nil {
		return cmd, err
	}

	return cmd, nil
}
