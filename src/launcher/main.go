package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
)

var doneChan chan struct{}

///////////////////////////////////////////////////////////////////////////////

func cleanupOnStartup() {
	if _, err := os.Stat("/tmp/docker-slim-launcher.cmds.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-launcher.cmds.ipc"); err != nil {
			fmt.Printf("Error removing unix socket %s: %s", "/tmp/docker-slim-launcher.cmds.ipc", err.Error())
		}
	}

	if _, err := os.Stat("/tmp/docker-slim-launcher.events.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-launcher.events.ipc"); err != nil {
			fmt.Printf("Error removing unix socket %s: %s", "/tmp/docker-slim-launcher.events.ipc", err.Error())
		}
	}
}

func cleanupOnShutdown() {
	log.Debug("cleanupOnShutdown()")

	if doneChan != nil {
		close(doneChan)
		doneChan = nil
	}

	shutdownCmdChannel()
	shutdownEvtChannel()
}

//////////////

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
	syscall.SIGSTOP,
	syscall.SIGCONT,
}

func initSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, signals...)
	go func() {
		sig := <-sigChan
		fmt.Printf(" launcher: cleanup on signal (%v)...\n", sig)
		cleanupOnShutdown()
		os.Exit(0)
	}()
}

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

	fanReportChan := fanRunMonitor(mountPoint, stopMonitor)
	ptReportChan := ptRunMonitor(ptmonStartChan, stopMonitor, appName, appArgs, dirName)
	//NOTE:
	//Disabled until linux-kernel module is added to check if the ProcEvents are enabled in the kernel
	//ProcEvents are not enabled in the boot2docker kernel
	//peReportChan := peRunMonitor(stopMonitor)

	go func() {
		log.Debug("launcher: monitor - waiting to stop monitoring...")
		<-stopWork
		log.Debug("launcher: monitor - stop message...")

		close(stopMonitor)

		log.Debug("launcher: monitor - processing data...")

		fanReport := <-fanReportChan
		ptReport := <-ptReportChan

		//peReport := <-peReportChan
		//TODO: when peReport is available filter file events from fanReport

		fileCount := 0
		for _, processFileMap := range fanReport.ProcessFiles {
			fileCount += len(processFileMap)
		}
		fileList := make([]string, 0, fileCount)
		for _, processFileMap := range fanReport.ProcessFiles {
			for fpath := range processFileMap {
				fileList = append(fileList, fpath)
			}
		}

		allFilesMap := findSymlinks(fileList, mountPoint)
		saveResults(fanReport, allFilesMap, ptReport)
		stopWorkAck <- true
	}()
}

/////////

func main() {
	//log.SetLevel(log.DebugLevel)

	log.Infof("launcher: args => %#v\n", os.Args)
	failWhen(len(os.Args) < 2, "missing app information")

	dirName, err := os.Getwd()
	failOnError(err)
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
	evtChannel, err = newEvtPublisher(evtChannelAddr)
	failOnError(err)
	cmdChannel, err = newCmdServer(cmdChannelAddr)
	failOnError(err)

	cmdChan, err := runCmdServer(cmdChannel, doneChan)
	failOnError(err)
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

	tryPublishEvt(3, "monitor.finish.completed")

	log.Info("launcher: done!")
}
