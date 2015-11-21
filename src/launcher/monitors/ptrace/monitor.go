package ptrace

import (
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	"internal/report"
	"internal/utils"
	"launcher/target"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/system"
)

type syscallEvent struct {
	callNum int16
	retVal  uint64
}

func Run(startChan <-chan int,
	stopChan chan struct{},
	appName string,
	appArgs []string,
	dirName string) <-chan *report.PtMonitorReport {
	log.Info("ptmon: starting...")

	sysInfo := system.GetSystemInfo()
	archName := system.MachineToArchName(sysInfo.Machine)
	syscallResolver := system.CallNumberResolver(archName)

	reportChan := make(chan *report.PtMonitorReport, 1)

	go func() {
		ptReport := &report.PtMonitorReport{
			ArchName:     string(archName),
			SyscallStats: map[string]report.SyscallStatInfo{},
		}

		syscallStats := map[int16]uint64{}
		eventChan := make(chan syscallEvent)
		doneMonitoring := make(chan int)

		var app *exec.Cmd

		go func() {
			//Ptrace is not pretty... and it requires that you do all ptrace calls from the same thread
			runtime.LockOSThread()

			var err error
			app, err = target.Start(appName, appArgs, dirName, true)
			utils.FailOn(err)
			targetPid := app.Process.Pid

			log.Debugf("ptmon: target PID ==> %d\n", targetPid)

			var wstat syscall.WaitStatus
			_, err = syscall.Wait4(targetPid, &wstat, 0, nil)
			if err != nil {
				log.Warnf("ptmon: error waiting for %d: %v\n", targetPid, err)
				doneMonitoring <- 1
			}

			log.Debugln("ptmon: initial process status =>", wstat)

			if wstat.Exited() {
				log.Warn("ptmon: app exited (unexpected)")
				doneMonitoring <- 2
			}

			if wstat.Signaled() {
				log.Warn("ptmon: app signalled (unexpected)")
				doneMonitoring <- 3
			}

			syscallReturn := false
			gotCallNum := false
			gotRetVal := false
			var callNum uint64
			var retVal uint64
			for wstat.Stopped() {
				var regs syscall.PtraceRegs

				switch syscallReturn {
				case false:
					if err := syscall.PtraceGetRegs(targetPid, &regs); err != nil {
						log.Fatalf("ptmon: PtraceGetRegs(call): %v", err)
					}

					callNum = regs.Orig_rax
					syscallReturn = true
					gotCallNum = true
				case true:
					if err := syscall.PtraceGetRegs(targetPid, &regs); err != nil {
						log.Fatalf("ptmon: PtraceGetRegs(return): %v", err)
					}

					retVal = regs.Rax
					syscallReturn = false
					gotRetVal = true
				}

				err = syscall.PtraceSyscall(targetPid, 0)
				if err != nil {
					log.Warnf("ptmon: PtraceSyscall error: %v\n", err)
					break
				}
				_, err = syscall.Wait4(targetPid, &wstat, 0, nil)
				if err != nil {
					log.Warnf("ptmon: error waiting 4 %d: %v\n", targetPid, err)
					break
				}

				if gotCallNum && gotRetVal {
					gotCallNum = false
					gotRetVal = false

					eventChan <- syscallEvent{
						callNum: int16(callNum),
						retVal:  retVal,
					}
				}
			}

			log.Infoln("ptmon: monitor is exiting... status=", wstat)
			doneMonitoring <- 0
		}()

	done:
		for {
			select {
			case rc := <-doneMonitoring:
				log.Info("ptmon: done =>", rc)
				break done
			case <-stopChan:
				log.Info("ptmon: stopping...")
				//NOTE: need a better way to stop the target app...
				if err := app.Process.Signal(syscall.SIGTERM); err != nil {
					log.Warnln("ptmon: error stopping target app =>", err)
					if err := app.Process.Kill(); err != nil {
						log.Warnln("ptmon: error killing target app =>", err)
					}
				}
				break done
			case e := <-eventChan:
				ptReport.SyscallCount++

				if _, ok := syscallStats[e.callNum]; ok {
					syscallStats[e.callNum]++
				} else {
					syscallStats[e.callNum] = 1
				}
			}
		}

		log.Debugf("ptmon: executed syscall count = %d\n", ptReport.SyscallCount)
		log.Debugf("ptmon: number of syscalls: %v\n", len(syscallStats))
		for scNum, scCount := range syscallStats {
			log.Debugf("[%v] %v = %v", scNum, syscallResolver(scNum), scCount)
			ptReport.SyscallStats[strconv.FormatInt(int64(scNum), 10)] = report.SyscallStatInfo{
				Number: scNum,
				Name:   syscallResolver(scNum),
				Count:  scCount,
			}
		}

		ptReport.SyscallNum = uint32(len(ptReport.SyscallStats))
		reportChan <- ptReport
	}()

	return reportChan
}
