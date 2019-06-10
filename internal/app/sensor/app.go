package app

import (
	"flag"
	"os"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/sensor/ipc"
	"github.com/docker-slim/docker-slim/internal/app/sensor/monitors/fanotify"
	"github.com/docker-slim/docker-slim/internal/app/sensor/monitors/pevent"
	"github.com/docker-slim/docker-slim/internal/app/sensor/monitors/ptrace"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/Sirupsen/logrus"
)

var doneChan chan struct{}

///////////////////////////////////////////////////////////////////////////////

func startMonitor(errorCh chan error,
	startAckChan chan bool,
	stopWork chan bool,
	stopWorkAck chan bool,
	pids chan []int,
	ptmonStartChan chan int,
	cmd *command.StartMonitor,
	dirName string) bool {
	log.Info("sensor: monitor starting...")
	mountPoint := "/"

	stopMonitor := make(chan struct{})

	var peReportChan <-chan *report.PeMonitorReport
	var peReport *report.PeMonitorReport
	usePEMon, err := system.DefaultKernelFeatures.IsCompiled("CONFIG_PROC_EVENTS")
	//tmp: disalbe PEVENTs (due to problems with the new boot2docker host OS)
	usePEMon = false
	if (err == nil) && usePEMon {
		log.Info("sensor: proc events are available!")
		peReportChan = pevent.Run(stopMonitor)
		//ProcEvents are not enabled in the default boot2docker kernel
	}

	fanReportChan := fanotify.Run(errorCh, mountPoint, stopMonitor) //data.AppName, data.AppArgs
	if fanReportChan == nil {
		log.Info("sensor: startMonitor - FAN failed to start running...")
		return false
	}

	ptReportChan := ptrace.Run(errorCh, startAckChan, ptmonStartChan, stopMonitor, cmd.AppName, cmd.AppArgs, dirName)
	if ptReportChan == nil {
		log.Info("sensor: startMonitor - PTAN failed to start running...")
		close(stopMonitor)
		return false
	}

	go func() {
		log.Debug("sensor: monitor.worker - waiting to stop monitoring...")
		<-stopWork
		log.Debug("sensor: monitor.worker - stop message...")

		close(stopMonitor)

		log.Debug("sensor: monitor.worker - processing data...")

		fanReport := <-fanReportChan
		ptReport := <-ptReportChan

		if peReportChan != nil {
			peReport = <-peReportChan
			//TODO: when peReport is available filter file events from fanReport
		}

		processReports(mountPoint, fanReport, ptReport, peReport, cmd)
		stopWorkAck <- true
	}()

	return true
}

/////////

var enableDebug bool

func init() {
	flag.BoolVar(&enableDebug, "d", false, "enable debug logging")
}

/////////

// Run starts the sensor app
func Run() {
	flag.Parse()

	if enableDebug {
		log.SetLevel(log.DebugLevel)
	}

	log.Debugf("sensor: uid=%v euid=%v", os.Getuid(), os.Geteuid())
	log.Debugf("sensor: sysinfo => %#v", system.GetSystemInfo())
	log.Debugf("sensor: kernel flags => %#v", system.DefaultKernelFeatures.Raw)

	log.Infof("sensor: args => %#v", os.Args)

	dirName, err := os.Getwd()
	errutil.WarnOn(err)
	log.Debugf("sensor: cwd => %#v", dirName)

	initSignalHandlers()
	defer func() {
		log.Debug("defered cleanup on shutdown...")
		cleanupOnShutdown()
	}()

	log.Debug("sensor: setting up channels...")
	doneChan = make(chan struct{})

	err = ipc.InitChannels()
	errutil.FailOn(err)

	cmdChan, err := ipc.RunCmdServer(doneChan)
	errutil.FailOn(err)

	errorCh := make(chan error)
	go func() {
		for {
			log.Debug("sensor: error collector - waiting for errors...")
			select {
			case <-doneChan:
				log.Debug("sensor: error collector - done...")
				return
			case err := <-errorCh:
				log.Infof("sensor: error collector - forwarding error = %+v", err)
				ipc.TryPublishEvt(3, &event.Message{Name: event.Error, Data: err})
			}
		}
	}()

	monStartAckChan := make(chan bool, 1)
	monDoneChan := make(chan bool, 1)
	monDoneAckChan := make(chan bool)
	pidsChan := make(chan []int, 1)
	ptmonStartChan := make(chan int, 1)

	log.Info("sensor: waiting for commands...")
doneRunning:
	for {
		select {
		case cmd := <-cmdChan:
			log.Debug("\nsensor: command => ", cmd)
			switch data := cmd.(type) {
			case *command.StartMonitor:
				if data == nil {
					log.Info("sensor: 'start' monitor command - no data...")
					break
				}

				log.Debugf("sensor: 'start' monitor command (%#v)", data)
				if data.AppUser != "" {
					log.Debugf("sensor: 'start' monitor command - run app as user='%s'", data.AppUser)
				}

				started := startMonitor(errorCh, monStartAckChan, monDoneChan, monDoneAckChan, pidsChan, ptmonStartChan, data, dirName)
				if !started {
					log.Info("sensor: monitor not started...")
					time.Sleep(3 * time.Second) //give error event time to get sent
					ipc.TryPublishEvt(3, &event.Message{Name: event.StartMonitorFailed})
					break
				}

				//target app started by ptmon... (long story :-))
				//TODO: need to get the target app pid to pemon, so it can filter process events
				log.Debugf("sensor: starting target app => %v %#v", data.AppName, data.AppArgs)
				time.Sleep(3 * time.Second)

				log.Info("sensor: waiting for monitor to complete startup...")
				started = <-monStartAckChan
				log.Infof("sensor: monitor started (%v)...", started)
				msg := &event.Message{Name: event.StartMonitorDone}
				if !started {
					msg.Name = event.StartMonitorFailed
				}
				ipc.TryPublishEvt(3, msg)

			case *command.StopMonitor:
				log.Info("sensor: 'stop' monitor command")

				monDoneChan <- true
				log.Info("sensor: waiting for monitor to finish...")
				<-monDoneAckChan
				log.Info("sensor: monitor stopped...")
				ipc.TryPublishEvt(3, &event.Message{Name: event.StopMonitorDone})

			case *command.ShutdownSensor:
				log.Info("sensor: 'shutdown' command")
				close(doneChan)
				doneChan = nil
				break doneRunning
			default:
				log.Info("sensor: ignoring unknown command => ", cmd)
			}

		case <-time.After(time.Second * 5):
			log.Debug(".")
		}
	}

	ipc.TryPublishEvt(3, &event.Message{Name: event.ShutdownSensorDone})

	log.Info("sensor: done!")
}
