package main

import (
	"flag"
	"os"
	"time"

	"github.com/cloudimmunity/docker-slim/messages"
	"github.com/cloudimmunity/docker-slim/report"
	"github.com/cloudimmunity/docker-slim/sensor/ipc"
	"github.com/cloudimmunity/docker-slim/sensor/monitors/fanotify"
	"github.com/cloudimmunity/docker-slim/sensor/monitors/pevent"
	"github.com/cloudimmunity/docker-slim/sensor/monitors/ptrace"
	"github.com/cloudimmunity/docker-slim/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/system"
)

var doneChan chan struct{}

///////////////////////////////////////////////////////////////////////////////

func monitor(stopWork chan bool,
	stopWorkAck chan bool,
	pids chan []int,
	ptmonStartChan chan int,
	cmd *messages.StartMonitor,
	dirName string) {
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

	fanReportChan := fanotify.Run(mountPoint, stopMonitor) //data.AppName, data.AppArgs
	ptReportChan := ptrace.Run(ptmonStartChan, stopMonitor, cmd.AppName, cmd.AppArgs, dirName)

	go func() {
		log.Debug("sensor: monitor - waiting to stop monitoring...")
		<-stopWork
		log.Debug("sensor: monitor - stop message...")

		close(stopMonitor)

		log.Debug("sensor: monitor - processing data...")

		fanReport := <-fanReportChan
		ptReport := <-ptReportChan

		if peReportChan != nil {
			peReport = <-peReportChan
			//TODO: when peReport is available filter file events from fanReport
		}

		processReports(mountPoint, fanReport, ptReport, peReport, cmd)
		stopWorkAck <- true
	}()
}

/////////

var enableDebug bool

func init() {
	flag.BoolVar(&enableDebug, "d", false, "enable debug logging")
}

/////////

func main() {
	flag.Parse()

	if enableDebug {
		log.SetLevel(log.DebugLevel)
	}

	log.Infof("sensor: args => %#v\n", os.Args)

	dirName, err := os.Getwd()
	utils.WarnOn(err)
	log.Debugf("sensor: cwd => %#v\n", dirName)

	initSignalHandlers()
	defer func() {
		log.Debug("defered cleanup on shutdown...")
		cleanupOnShutdown()
	}()

	log.Debug("sensor: setting up channels...")
	doneChan = make(chan struct{})

	err = ipc.InitChannels()
	utils.FailOn(err)

	cmdChan, err := ipc.RunCmdServer(doneChan)
	utils.FailOn(err)

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
			case *messages.StartMonitor:
				if data == nil {
					log.Info("sensor: 'start' command - no data...")
					break
				}

				log.Debugf("sensor: 'start' command (%#v) - starting monitor...\n", data)
				monitor(monDoneChan, monDoneAckChan, pidsChan, ptmonStartChan, data, dirName)

				//target app started by ptmon... (long story :-))
				//TODO: need to get the target app pid to pemon, so it can filter process events
				log.Debugf("sensor: target app started => %v %#v\n", data.AppName, data.AppArgs)
				time.Sleep(3 * time.Second)

			case *messages.StopMonitor:
				log.Debug("sensor: 'stop' command - stopping monitor...")
				break doneRunning
			default:
				log.Debug("sensor: ignoring unknown command => ", cmd)
			}

		case <-time.After(time.Second * 5):
			log.Debug(".")
		}
	}

	monDoneChan <- true
	log.Info("sensor: waiting for monitor to finish...")
	<-monDoneAckChan

	ipc.TryPublishEvt(3, "monitor.finish.completed")

	log.Info("sensor: done!")
}
