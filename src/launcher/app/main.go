package main

import (
	"os"
	"time"

	"internal/report"
	"internal/utils"
	"launcher/ipc"
	"launcher/monitors/fanotify"
	"launcher/monitors/pevent"
	"launcher/monitors/ptrace"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/system"
)

var doneChan chan struct{}

///////////////////////////////////////////////////////////////////////////////

func monitor(stopWork chan bool,
	stopWorkAck chan bool,
	pids chan []int,
	ptmonStartChan chan int,
	appName string,
	appArgs []string,
	dirName string) {
	log.Info("launcher: monitor starting...")
	mountPoint := "/"

	stopMonitor := make(chan struct{})

	var peReportChan <-chan *report.PeMonitorReport
	var peReport *report.PeMonitorReport
	usePEMon, err := system.DefaultKernelFeatures.IsCompiled("CONFIG_PROC_EVENTS")
	if (err == nil) && usePEMon {
		log.Info("launcher: proc events are available!")
		peReportChan = pevent.Run(stopMonitor)
		//ProcEvents are not enabled in the default boot2docker kernel
	}

	fanReportChan := fanotify.Run(mountPoint, stopMonitor)
	ptReportChan := ptrace.Run(ptmonStartChan, stopMonitor, appName, appArgs, dirName)

	go func() {
		log.Debug("launcher: monitor - waiting to stop monitoring...")
		<-stopWork
		log.Debug("launcher: monitor - stop message...")

		close(stopMonitor)

		log.Debug("launcher: monitor - processing data...")

		fanReport := <-fanReportChan
		ptReport := <-ptReportChan

		if peReportChan != nil {
			peReport = <-peReportChan
			//TODO: when peReport is available filter file events from fanReport
		}

		processReports(mountPoint, fanReport, ptReport, peReport)
		stopWorkAck <- true
	}()
}

/////////

func main() {
	//log.SetLevel(log.DebugLevel)

	log.Infof("launcher: args => %#v\n", os.Args)
	utils.FailWhen(len(os.Args) < 2, "missing app information")

	dirName, err := os.Getwd()
	utils.FailOn(err)
	log.Debugf("launcher: cwd => %#v\n", dirName)

	appName := os.Args[1]
	var appArgs []string
	if len(os.Args) > 2 {
		appArgs = os.Args[2:]
	}

	initSignalHandlers()
	defer func() {
		log.Debug("defered cleanup on shutdown...")
		cleanupOnShutdown()
	}()

	monDoneChan := make(chan bool, 1)
	monDoneAckChan := make(chan bool)
	pidsChan := make(chan []int, 1)
	ptmonStartChan := make(chan int, 1)
	monitor(monDoneChan, monDoneAckChan, pidsChan, ptmonStartChan, appName, appArgs, dirName)

	//target app started by ptmon... (long story :-))
	//TODO: need to get the target app pid to pemon, so it can filter process events
	log.Debugf("launcher: target app started => %v %#v\n", appName, appArgs)
	time.Sleep(3 * time.Second)

	log.Debug("launcher: setting up channels...")
	doneChan = make(chan struct{})

	err = ipc.InitChannels()
	utils.FailOn(err)

	cmdChan, err := ipc.RunCmdServer(doneChan)
	utils.FailOn(err)
	log.Info("launcher: waiting for commands...")
doneRunning:
	for {
		select {
		case cmd := <-cmdChan:
			log.Debugln("\nlauncher: command =>", cmd)
			switch cmd {
			case "monitor.finish":
				log.Debug("launcher: 'monitor.finish' command - stopping monitor...")
				break doneRunning
			default:
				log.Debugln("launcher: ignoring command =>", cmd)
			}

		case <-time.After(time.Second * 5):
			log.Debug(".")
		}
	}

	monDoneChan <- true
	log.Info("launcher: waiting for monitor to finish...")
	<-monDoneAckChan

	ipc.TryPublishEvt(3, "monitor.finish.completed")

	log.Info("launcher: done!")
}
