package ptrace

import (
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/sensor/target"
	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/Sirupsen/logrus"
)

type syscallEvent struct {
	callNum int16
	retVal  uint64
}

const (
	eventBufSize = 500
)

// Run starts the PTRACE monitor
func Run(
	errorCh chan error,
	ackChan chan<- bool,
	startChan <-chan int,
	stopChan chan struct{},
	appName string,
	appArgs []string,
	dirName string) <-chan *report.PtMonitorReport {
	log.Info("ptmon: Run")

	sysInfo := system.GetSystemInfo()
	archName := system.MachineToArchName(sysInfo.Machine)
	syscallResolver := system.CallNumberResolver(archName)

	resultChan := make(chan *report.PtMonitorReport, 1)

	go func() {
		log.Debug("ptmon: processor - starting...")

		ptReport := &report.PtMonitorReport{
			ArchName:     string(archName),
			SyscallStats: map[string]report.SyscallStatInfo{},
		}

		syscallStats := map[int16]uint64{}
		eventChan := make(chan syscallEvent, eventBufSize)
		collectorDoneChan := make(chan int, 1)

		var app *exec.Cmd

		go func() {
			log.Debug("ptmon: collector - starting...")
			//Ptrace is not pretty... and it requires that you do all ptrace calls from the same thread
			runtime.LockOSThread()

			var err error
			app, err = target.Start(appName, appArgs, dirName, true)
			started := true
			if err != nil {
				started = false
			}
			ackChan <- started

			if err != nil {
				sensorErr := errors.SE("sensor.ptrace.Run/target.Start", "call.error", err)
				errorCh <- sensorErr
				time.Sleep(3 * time.Second)
			}
			errutil.FailOn(err)

			targetPid := app.Process.Pid

			log.Debugf("ptmon: collector - target PID ==> %d", targetPid)

			var wstat syscall.WaitStatus
			_, err = syscall.Wait4(targetPid, &wstat, 0, nil)
			if err != nil {
				log.Warnf("ptmon: collector - error waiting for %d: %v", targetPid, err)
				collectorDoneChan <- 1
				return
			}

			log.Debugln("ptmon: initial process status =>", wstat)

			if wstat.Exited() {
				log.Warn("ptmon: collector - app exited (unexpected)")
				collectorDoneChan <- 2
				return
			}

			if wstat.Signaled() {
				log.Warn("ptmon: collector - app signalled (unexpected)")
				collectorDoneChan <- 3
				return
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
						log.Fatalf("ptmon: collector - PtraceGetRegs(call): %v", err)
					}

					callNum = system.CallNumber(regs)
					syscallReturn = true
					gotCallNum = true
				case true:
					if err := syscall.PtraceGetRegs(targetPid, &regs); err != nil {
						log.Fatalf("ptmon: collector - PtraceGetRegs(return): %v", err)
					}

					retVal = system.CallReturnValue(regs)
					syscallReturn = false
					gotRetVal = true
				}

				err = syscall.PtraceSyscall(targetPid, 0)
				if err != nil {
					log.Warnf("ptmon: collector - PtraceSyscall error: %v", err)
					break
				}
				_, err = syscall.Wait4(targetPid, &wstat, 0, nil)
				if err != nil {
					log.Warnf("ptmon: collector - error waiting 4 %d: %v", targetPid, err)
					break
				}

				if gotCallNum && gotRetVal {
					gotCallNum = false
					gotRetVal = false

					select {
					case eventChan <- syscallEvent{
						callNum: int16(callNum),
						retVal:  retVal,
					}:
					case <-stopChan:
						log.Info("ptmon: collector - stopping...")
						return
					}
				}
			}

			log.Infoln("ptmon: collector - exiting... status=", wstat)
			collectorDoneChan <- 0
		}()

	done:
		for {
			select {
			case rc := <-collectorDoneChan:
				log.Info("ptmon: processor - collector finished =>", rc)
				break done
			case <-stopChan:
				log.Info("ptmon: processor - stopping...")
				//NOTE: need a better way to stop the target app...
				if err := app.Process.Signal(syscall.SIGTERM); err != nil {
					log.Warnln("ptmon: processor - error stopping target app =>", err)
					if err := app.Process.Kill(); err != nil {
						log.Warnln("ptmon: processor - error killing target app =>", err)
					}
				}
				break done
			case e := <-eventChan:
				ptReport.SyscallCount++
				log.Debugf("ptmon: syscall ==> %d", e.callNum)

				if _, ok := syscallStats[e.callNum]; ok {
					syscallStats[e.callNum]++
				} else {
					syscallStats[e.callNum] = 1
				}
			}
		}

		log.Debugf("ptmon: processor - executed syscall count = %d", ptReport.SyscallCount)
		log.Debugf("ptmon: processor - number of syscalls: %v", len(syscallStats))
		for scNum, scCount := range syscallStats {
			log.Debugf("%v", syscallResolver(scNum))
			log.Debugf("[%v] %v = %v", scNum, syscallResolver(scNum), scCount)
			ptReport.SyscallStats[strconv.FormatInt(int64(scNum), 10)] = report.SyscallStatInfo{
				Number: scNum,
				Name:   syscallResolver(scNum),
				Count:  scCount,
			}
		}

		ptReport.SyscallNum = uint32(len(ptReport.SyscallStats))
		resultChan <- ptReport
	}()

	return resultChan
}
