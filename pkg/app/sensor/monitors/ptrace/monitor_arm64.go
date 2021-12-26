// +build arm64

package ptrace

import (
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/launcher"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type syscallEvent struct {
	callNum uint32
	retVal  uint64
}

const (
	eventBufSize = 500
	ptOptions    = unix.PTRACE_O_TRACECLONE | unix.PTRACE_O_TRACEFORK | unix.PTRACE_O_TRACEVFORK
)

/*
	unix.PTRACE_O_EXITKILL|
	unix.PTRACE_O_TRACESECCOMP|
	syscall.PTRACE_O_TRACESYSGOOD|
	syscall.PTRACE_O_TRACEEXEC|
	syscall.PTRACE_O_TRACECLONE|
	syscall.PTRACE_O_TRACEFORK|
	syscall.PTRACE_O_TRACEVFORK

	syscall.PTRACE_O_TRACECLONE|syscall.PTRACE_O_TRACEEXIT
*/

// Run starts the PTRACE monitor
func Run(
	errorCh chan error,
	ackChan chan<- bool,
	startChan <-chan int,
	stopChan chan struct{},
	appName string,
	appArgs []string,
	dirName string,
	appUser string,
	runTargetAsUser bool,
	origPaths map[string]interface{}) <-chan *report.PtMonitorReport {
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

		syscallStats := map[uint32]uint64{}
		eventChan := make(chan syscallEvent, eventBufSize)
		collectorDoneChan := make(chan int, 1)

		var app *exec.Cmd

		go func() {
			log.Debug("ptmon: collector - starting...")
			//Ptrace is not pretty... and it requires that you do all ptrace calls from the same thread
			runtime.LockOSThread()

			var err error
			app, err = launcher.Start(appName, appArgs, dirName, appUser, runTargetAsUser, true)
			started := true
			if err != nil {
				started = false
			}
			ackChan <- started

			if err != nil {
				sensorErr := errors.SE("sensor.ptrace.Run/launcher.Start", "call.error", err)
				errorCh <- sensorErr
				time.Sleep(3 * time.Second)
			}
			errutil.FailOn(err)

			targetPid := app.Process.Pid

			//pgid, err := syscall.Getpgid(targetPid)
			//if err != nil {
			//	log.Warnf("ptmon: collector - getpgid error %d: %v", targetPid, err)
			//	collectorDoneChan <- 1
			//	return
			//}

			log.Debugf("ptmon: collector - target PID ==> %d", targetPid)

			var wstat unix.WaitStatus

			//pid, err := syscall.Wait4(-1, &wstat, syscall.WALL, nil) - WIP
			pid, err := unix.Wait4(targetPid, &wstat, 0, nil)
			if err != nil {
				log.Warnf("ptmon: collector - error waiting for %d: %v", targetPid, err)
				collectorDoneChan <- 2
				return
			}

			//err = syscall.PtraceSetOptions(targetPid, ptOptions)
			//if err != nil {
			//	log.Warnf("ptmon: collector - error setting trace options %d: %v", targetPid, err)
			//	collectorDoneChan <- 3
			//	return
			//}

			log.Debugf("ptmon: initial process status = %v (pid=%d)\n", wstat, pid)

			if wstat.Exited() {
				log.Warn("ptmon: collector - app exited (unexpected)")
				collectorDoneChan <- 4
				return
			}

			if wstat.Signaled() {
				log.Warn("ptmon: collector - app signalled (unexpected)")
				collectorDoneChan <- 5
				return
			}

			syscallReturn := false
			gotCallNum := false
			gotRetVal := false
			var callNum uint64
			var retVal uint64
			for wstat.Stopped() {
				var regs unix.PtraceRegsArm64

				switch syscallReturn {
				case false:
					log.Infof("target pid is %d", targetPid)
					if err := unix.PtraceGetRegSetArm64(targetPid, 1, &regs); err != nil {
						//if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
						log.Fatalf("ptmon: collector - PtraceGetRegsArm64(call): %v", err)
					}

					callNum = system.CallNumber(regs)
					syscallReturn = true
					gotCallNum = true

				case true:
					if err := unix.PtraceGetRegSetArm64(targetPid, 1, &regs); err != nil {
						//if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
						log.Fatalf("ptmon: collector - PtraceGetRegsArm64(return): %v", err)
					}

					retVal = system.CallReturnValue(regs)
					syscallReturn = false
					gotRetVal = true

				}

				//err = syscall.PtraceSyscall(pid, 0)
				err = unix.PtraceSyscall(targetPid, 0)
				if err != nil {
					log.Warnf("ptmon: collector - PtraceSyscall error: %v", err)
					break
				}

				//pid, err = syscall.Wait4(-1, &wstat, syscall.WALL, nil)
				pid, err = unix.Wait4(targetPid, &wstat, 0, nil)
				if err != nil {
					log.Warnf("ptmon: collector - error waiting 4 %d: %v", targetPid, err)
					break
				}

				if gotCallNum && gotRetVal {
					gotCallNum = false
					gotRetVal = false

					select {
					case eventChan <- syscallEvent{
						callNum: uint32(callNum),
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
				if err := app.Process.Signal(unix.SIGTERM); err != nil {
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
