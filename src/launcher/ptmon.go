package main

import (
	"log"
	//"os"
	"os/exec"
	"runtime"
	"syscall"
	"strconv"
)

type syscallStatInfo struct {
	Number uint64 `json:"num"`
	Name   string `json:"name"`
	Count  uint64 `json:"count"`
}

type ptMonitorReport struct {
	SyscallCount uint64                     `json:"syscall_count"`
	SyscallNum   uint32                     `json:"syscall_num"`
	SyscallStats map[string]syscallStatInfo `json:"syscall_stats"`
}

type syscallEvent struct {
	callNum uint64
	retVal uint64
}


func ptRunMonitor(startChan <-chan int, 
	stopChan chan struct{},
	appName string,
	appArgs []string,
	dirName string) <-chan *ptMonitorReport {
	reportChan := make(chan *ptMonitorReport, 1)

	go func() {
		report := &ptMonitorReport{
			SyscallStats: map[string]syscallStatInfo{},
		}

		//log.Println("ptmon: waiting for process...")
		//targetPid, ok := <-startChan
		//app, err := startTargetApp(appName, appArgs, dirName, true)
		//failOnError(err)
		//targetPid := app.Process.Pid

		//log.Printf("ptmon: target PID => %d\n", targetPid)
		//if !ok {
		//	reportChan <- report
		//	return
		//}

		syscallStats := map[uint64]uint64{}
		eventChan := make(chan syscallEvent)
		doneMonitoring := make(chan int)

		var app *exec.Cmd

		go func() {
			//IMPORTANT NOTE:
			//Ptrace is not pretty... and it requires that you do all ptrace calls from the same thread
			runtime.LockOSThread()

			var err error
			app, err = startTargetApp(appName, appArgs, dirName, true)
			failOnError(err)
			targetPid := app.Process.Pid

			log.Printf("ptmon: target PID ==> %d\n", targetPid)

			var wstat syscall.WaitStatus
			//log.Println("TMP: ptmon: get initial process status...")
			_, err = syscall.Wait4(targetPid, &wstat, 0, nil)
			if err != nil {
				log.Printf("ptmon: error waiting for %d: %v\n", targetPid, err)
				doneMonitoring <- 1
			}

			//log.Println("TMP: ptmon: initial process status =>",wstat)

			if wstat.Exited() {
				log.Println("ptmon: app exited unexpectedly")
				doneMonitoring <- 2
			}

			if wstat.Signaled() {
				log.Println("ptmon: app signalled unexpectedly")
				doneMonitoring <- 3
			}

			syscallReturn := false
			gotCallNum := false
			gotRetVal := false
			var callNum uint64
			var retVal uint64
			for wstat.Stopped() {
				var regs syscall.PtraceRegs

				//log.Println("TMP: ptmon: get syscall info...")
				switch syscallReturn {
				case false:
					if err := syscall.PtraceGetRegs(targetPid, &regs); err != nil {
						log.Fatalf("PtraceGetRegs(call): %v", err)
					}

					callNum = regs.Orig_rax
					//log.Printf("[syscall]: %v\n", callNum)
					syscallReturn = true
					gotCallNum = true
				case true:
					if err := syscall.PtraceGetRegs(targetPid, &regs); err != nil {
						log.Fatalf("PtraceGetRegs(return): %v", err)
					}

					retVal = regs.Rax
					//log.Printf("[syscall return]: %v\n", retVal)
					syscallReturn = false
					gotRetVal = true
				}

				err = syscall.PtraceSyscall(targetPid, 0)
				if err != nil {
					log.Printf("PtraceSyscall error: %v\n", err)
					break
				}
				_, err = syscall.Wait4(targetPid, &wstat, 0, nil)
				if err != nil {
					log.Printf("error waiting 4 %d: %v\n", targetPid, err)
					break
				}

				if gotCallNum && gotRetVal {
					gotCallNum = false
					gotRetVal = false
					//log.Printf("TMP: ptmon: sending event data: %v %v\n",callNum,retVal)
					eventChan <- syscallEvent{
						callNum: callNum,
						retVal: retVal,
					}
				}
			}

			log.Println("ptmon: monitor is exiting... status=",wstat)
			doneMonitoring <- 0
		}()	

		done_monitoring:
		for {
			select {
				case rc := <-doneMonitoring:
					log.Println("monitor.ptRunMonitor done =>",rc)
					break done_monitoring
				case <-stopChan:
					log.Println("monitor.ptRunMonitor stopping...")
					//NOTE: need a better way to stop the target app...
					if err := app.Process.Signal(syscall.SIGTERM); err != nil {
						log.Println("monitor.ptRunMonitor - error stopping target app =>",err)
						if err := app.Process.Kill(); err != nil {
							log.Println("monitor.ptRunMonitor - error killing target app =>",err)
						}
					}
					break done_monitoring
				case e := <- eventChan:
					report.SyscallCount++
					//log.Printf("TMP: monitor.ptRunMonitor: event %v => %v\n",report.SyscallCount,e)

					if _, ok := syscallStats[e.callNum]; ok {
						//log.Printf("TMP: monitor.ptRunMonitor - updating syscall => %v / %v\n",e.callNum,syscallName64(e.callNum))
						syscallStats[e.callNum]++
					} else {
						//log.Printf("TMP: monitor.ptRunMonitor - first seen syscall => %v / %v\n",e.callNum,syscallName64(e.callNum))
						syscallStats[e.callNum] = 1
					}
			}
		}

		//log.Printf("TMP: monitor.ptRunMonitor: summary - syscall executions = %d\n", report.SyscallCount)
		//log.Printf("TMP: monitor.ptRunMonitor: summary - number of syscalls: %v\n", len(syscallStats))
		for scNum, scCount := range syscallStats {
			log.Printf("[%v] %v = %v", scNum, syscallName64(scNum), scCount)
			report.SyscallStats[strconv.FormatUint(scNum,10)] = syscallStatInfo{
				Number: scNum,
				Name:   syscallName64(scNum),
				Count:  scCount,
			}
		}

		report.SyscallNum = uint32(len(report.SyscallStats))
		reportChan <- report
	}()

	return reportChan
}
