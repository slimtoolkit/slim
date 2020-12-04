package ptrace

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/docker-slim/docker-slim/pkg/app/launcher"
	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/system"
)

type AppState string

const (
	AppStarted AppState = "app.started"
	AppFailed  AppState = "app.failed"
	AppDone    AppState = "app.done"
	AppExited  AppState = "app.exited"
)

func Run(
	cmd string,
	args []string,
	dir string,
	user string,
	runAsUser bool,
	reportCh chan *report.PtMonitorReport,
	errorCh chan error,
	stateCh chan AppState,
	stopCh chan struct{},
) (*App, error) {
	log.Debug("ptrace.Run")
	app, err := newApp(cmd, args, dir, user, runAsUser, reportCh, errorCh, stateCh, stopCh)
	if err != nil {
		app.StateCh <- AppFailed
		return nil, err
	}

	go app.process()
	go app.trace()

	return app, nil
}

const ptOptions = syscall.PTRACE_O_TRACECLONE |
	syscall.PTRACE_O_TRACEFORK |
	syscall.PTRACE_O_TRACEVFORK |
	syscall.PTRACE_O_TRACEEXEC |
	syscall.PTRACE_O_TRACESYSGOOD |
	syscall.PTRACE_O_TRACEEXIT |
	unix.PTRACE_O_EXITKILL

type syscallState struct {
	pid          int
	callNum      uint64
	retVal       uint64
	expectReturn bool
	gotCallNum   bool
	gotRetVal    bool
	started      bool
	exiting      bool
}

type App struct {
	Cmd             string
	Args            []string
	Dir             string
	User            string
	RunAsUser       bool
	Report          report.PtMonitorReport
	ReportCh        chan *report.PtMonitorReport
	ErrorCh         chan error
	StateCh         chan AppState
	StopCh          chan struct{}
	syscallCounters map[uint32]uint64
	syscallResolver system.NumberResolverFunc
	cmd             *exec.Cmd
	pgid            int
	eventCh         chan syscallEvent
	collectorDoneCh chan int
}

func (a *App) MainPID() int {
	return a.cmd.Process.Pid
}

func (a *App) PGID() int {
	return a.pgid
}

const eventBufSize = 2000

type syscallEvent struct {
	pid     int
	callNum uint32
	retVal  uint64
}

func newApp(cmd string,
	args []string,
	dir string,
	user string,
	runAsUser bool,
	reportCh chan *report.PtMonitorReport,
	errorCh chan error,
	stateCh chan AppState,
	stopCh chan struct{}) (*App, error) {
	log.Debug("ptrace.newApp")
	if reportCh == nil {
		reportCh = make(chan *report.PtMonitorReport, 1)
	}

	if errorCh == nil {
		errorCh = make(chan error, 100)
	}

	if stateCh == nil {
		stateCh = make(chan AppState, 10)
	}

	if stopCh == nil {
		stopCh = make(chan struct{})
	}

	sysInfo := system.GetSystemInfo()
	archName := system.MachineToArchName(sysInfo.Machine)

	a := App{
		Cmd:             cmd,
		Args:            args,
		Dir:             dir,
		User:            user,
		RunAsUser:       runAsUser,
		ReportCh:        reportCh,
		ErrorCh:         errorCh,
		StateCh:         stateCh,
		StopCh:          stopCh,
		syscallCounters: map[uint32]uint64{},
		eventCh:         make(chan syscallEvent, eventBufSize),
		collectorDoneCh: make(chan int, 1),
		syscallResolver: system.CallNumberResolver(archName),
		Report: report.PtMonitorReport{
			ArchName:     string(archName),
			SyscallStats: map[string]report.SyscallStatInfo{},
		},
	}

	return &a, nil
}

func (app *App) Stop() {
	close(app.StopCh)
}

func (app *App) trace() {
	log.Debug("ptrace.App.trace")
	runtime.LockOSThread()

	err := app.start()
	if err != nil {
		app.collectorDoneCh <- 1
		app.StateCh <- AppFailed
		app.ErrorCh <- errors.SE("ptrace.App.trace.app.start", "call.error", err)
		return
	}

	app.StateCh <- AppStarted
	app.collect()
}

func (app *App) process() {
	log.Debug("ptrace.App.process")
	state := AppDone

done:
	for {
		select {
		case rc := <-app.collectorDoneCh:
			log.Debugf("ptrace.App.process: collector finished => %v", rc)
			if rc > 0 {
				state = AppFailed
			}
			break done
		case <-app.StopCh:
			log.Debug("ptrace.App.process: stopping...")
			//NOTE: need a better way to stop the target app...
			//"os: process already finished" error is ok
			if err := app.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Debug("ptrace.App.process: error stopping target app => ", err)
				if err := app.cmd.Process.Kill(); err != nil {
					log.Debug("ptrace.App.process: error killing target app => ", err)
				}
			}
			break done
		case e := <-app.eventCh:
			app.Report.SyscallCount++
			log.Debugf("ptrace.App.process: event ==> {pid=%v cn=%d}", e.pid, e.callNum)

			if _, ok := app.syscallCounters[e.callNum]; ok {
				app.syscallCounters[e.callNum]++
			} else {
				app.syscallCounters[e.callNum] = 1
			}
		}
	}

	log.Debugf("ptrace.App.process: - executed syscall count = %d", app.Report.SyscallCount)
	log.Debugf("ptrace.App.process: - number of syscalls: %v", len(app.syscallCounters))

	for scNum, scCount := range app.syscallCounters {
		syscallName := app.syscallResolver(scNum)
		log.Debugf("[%v] %v = %v", scNum, syscallName, scCount)
		scKey := strconv.FormatInt(int64(scNum), 10)
		app.Report.SyscallStats[scKey] = report.SyscallStatInfo{
			Number: scNum,
			Name:   syscallName,
			Count:  scCount,
		}
	}

	app.Report.SyscallNum = uint32(len(app.Report.SyscallStats))

	app.StateCh <- state
	app.ReportCh <- &app.Report
}

func (app *App) start() error {
	log.Debug("ptrace.App.start")
	var err error
	app.cmd, err = launcher.Start(app.Cmd, app.Args, app.Dir, app.User, app.RunAsUser, true)
	if err != nil {
		log.Errorf("ptrace.App.start: cmd='%v' args='%+v' dir='%v' error=%v\n",
			app.Cmd, app.Args, app.Dir, err)
		return err
	}

	err = app.cmd.Wait()
	log.Debugf("ptrace.App.start: app.cmd.Wait err - %v", err)
	log.Debugf("ptrace.App.start: Process state info - Exited=%v ExitCode=%v SysWaitStatus=%v",
		app.cmd.ProcessState.Exited(),
		app.cmd.ProcessState.ExitCode(),
		app.cmd.ProcessState.Sys())

	waitStatus, ok := app.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ok {
		log.Debugf("ptrace.App.start: Process wait status - %v (Exited=%v Signaled=%v Signal='%v' Stopped=%v StopSignal='%v' TrapCause=%v)",
			waitStatus,
			waitStatus.Exited(),
			waitStatus.Signaled(),
			waitStatus.Signal(),
			waitStatus.Stopped(),
			waitStatus.StopSignal(),
			waitStatus.TrapCause())

		if waitStatus.Exited() {
			log.Debug("ptrace.App.start: unexpected app exit")
			return fmt.Errorf("unexpected app exit")
		}

		if waitStatus.Signaled() {
			log.Debug("ptrace.App.start: unexpected app signalled")
			return fmt.Errorf("unexpected app signalled")
		}

		//we should be in the Stopped state
		if waitStatus.Stopped() {
			sigEnum := SignalEnum(int(waitStatus.StopSignal()))
			log.Debugf("ptrace.App.start: Process Stop Signal - code=%d enum=%s str=%s",
				waitStatus.StopSignal(), sigEnum, waitStatus.StopSignal())
		} else {
			//TODO:
			//check for Exited or Signaled process state (shouldn't happen)
			//do it for context indicating that we are in a failed state
		}
	} else {
		return fmt.Errorf("process status error")
	}

	app.pgid, err = syscall.Getpgid(app.cmd.Process.Pid)
	if err != nil {
		return err
	}

	log.Debugf("ptrace.App.start: started target app --> PID=%d PGID=%d",
		app.cmd.Process.Pid, app.pgid)

	err = syscall.PtraceSetOptions(app.cmd.Process.Pid, ptOptions)
	if err != nil {
		return err
	}

	return nil
}

const traceSysGoodStatusBit = 0x80

func StopSignalInfo(sig syscall.Signal) string {
	sigNum := int(sig)
	if sigNum == -1 {
		return fmt.Sprintf("(code=%d)", sigNum)
	}

	sigEnum := SignalEnum(sigNum)
	sigStr := sig.String()
	if sig&traceSysGoodStatusBit == traceSysGoodStatusBit {
		msig := sig &^ traceSysGoodStatusBit
		sigEnum = fmt.Sprintf("%s|0x%04x", SignalEnum(int(msig)), traceSysGoodStatusBit)
		sigStr = fmt.Sprintf("%s|0x%04x", msig, traceSysGoodStatusBit)
	}

	info := fmt.Sprintf("(code=%d/0x%04x enum='%s' str='%s')",
		sigNum, sigNum, sigEnum, sigStr)

	return info
}

func SigTrapCauseInfo(cause int) string {
	if cause == -1 {
		return fmt.Sprintf("(code=%d)", cause)
	}

	causeEnum := PtraceEvenEnum(cause)
	info := fmt.Sprintf("(code=%d enum=%s)", cause, causeEnum)

	return info
}

func PtraceEvenEnum(data int) string {
	if enum, ok := ptEventMap[data]; ok {
		return enum
	} else {
		return fmt.Sprintf("(%d)", data)
	}
}

var ptEventMap = map[int]string{
	syscall.PTRACE_EVENT_CLONE:      "PTRACE_EVENT_CLONE",
	syscall.PTRACE_EVENT_EXEC:       "PTRACE_EVENT_EXEC",
	syscall.PTRACE_EVENT_EXIT:       "PTRACE_EVENT_EXIT",
	syscall.PTRACE_EVENT_FORK:       "PTRACE_EVENT_FORK",
	unix.PTRACE_EVENT_SECCOMP:       "PTRACE_EVENT_SECCOMP",
	unix.PTRACE_EVENT_STOP:          "PTRACE_EVENT_STOP",
	syscall.PTRACE_EVENT_VFORK:      "PTRACE_EVENT_VFORK",
	syscall.PTRACE_EVENT_VFORK_DONE: "PTRACE_EVENT_VFORK_DONE",
}

func (app *App) collect() {
	log.Debug("ptrace.App.collect")
	callPid := app.MainPID()
	prevPid := callPid

	log.Tracef("ptrace.App.collect: trace syscall mainPID=%v", callPid)

	pidSyscallState := map[int]*syscallState{}
	pidSyscallState[callPid] = &syscallState{pid: callPid}

	mainExiting := false
	waitFor := -1
	doSyscall := true
	for {
		var callSig int

		select {
		case <-app.StopCh:
			log.Debug("ptrace.App.collect: stop (exiting)")
			return
		default:
		}

		if doSyscall {
			log.Tracef("ptrace.App.collect: trace syscall (pid=%v sig=%v)", callPid, callSig)
			err := syscall.PtraceSyscall(callPid, callSig)
			if err != nil {
				log.Errorf("ptrace.App.collect: trace syscall pid=%v sig=%v error - %v (errno=%d)", callPid, callSig, err, err.(syscall.Errno))
				app.ErrorCh <- errors.SE("ptrace.App.collect.ptsyscall", "call.error", err)
				//keep waiting for other syscalls
			}
		}

		log.Trace("ptrace.App.collect: waiting for syscall...")
		var ws syscall.WaitStatus
		wpid, err := syscall.Wait4(waitFor, &ws, syscall.WALL, nil)
		if err != nil {
			if err.(syscall.Errno) == syscall.ECHILD {
				log.Debug("ptrace.App.collect: wait4 ECHILD error (ignoring)")
				doSyscall = false
				continue
			}

			log.Debugf("ptrace.App.collect: wait4 error - %v (errno=%d)", err, err.(syscall.Errno))
			app.ErrorCh <- errors.SE("ptrace.App.collect.wait4", "call.error", err)
			app.StateCh <- AppFailed
			app.collectorDoneCh <- 2
			return
		}

		log.Tracef("ptrace.App.collect: wait4 -> wpid=%v wstatus=%v (Exited=%v Signaled=%v Signal='%v' Stopped=%v StopSignalInfo=%s TrapCause=%s)",
			wpid,
			ws,
			ws.Exited(),
			ws.Signaled(),
			ws.Signal(),
			ws.Stopped(),
			StopSignalInfo(ws.StopSignal()),
			SigTrapCauseInfo(ws.TrapCause()))

		if wpid == -1 {
			log.Error("ptrace.App.collect: wpid = -1")
			app.StateCh <- AppFailed
			app.ErrorCh <- errors.SE("ptrace.App.collect.wpid", "call.error", fmt.Errorf("wpid is -1"))
			return
		}

		terminated := false
		stopped := false
		eventStop := false
		handleCall := false
		eventCode := 0
		statusCode := 0
		switch {
		case ws.Exited():
			terminated = true
			statusCode = ws.ExitStatus()
		case ws.Signaled():
			terminated = true
			statusCode = int(ws.Signal())
		case ws.Stopped():
			stopped = true
			statusCode = int(ws.StopSignal())
			if statusCode == int(syscall.SIGTRAP|traceSysGoodStatusBit) {
				handleCall = true
			} else if statusCode == int(syscall.SIGTRAP) {
				eventStop = true
				eventCode = ws.TrapCause()
			} else {
				callSig = statusCode
			}
		}

		if terminated {
			if _, ok := pidSyscallState[wpid]; !ok {
				log.Debugf("ptrace.App.collect: unknown process is terminated (%v)", wpid)
			} else {
				if !pidSyscallState[wpid].exiting {
					log.Debugf("ptrace.App.collect: unexpected process termination (%v)", wpid)
				}
			}

			delete(pidSyscallState, wpid)
			if app.MainPID() == wpid {
				log.Debug("ptrace.App.collect: wpid is main PID and terminated...")
				if !mainExiting {
					log.Debug("ptrace.App.collect: unexpected main PID termination...")
				}
			}

			if len(pidSyscallState) == 0 {
				log.Debug("ptrace.App.collect: all processes terminated...")
				app.collectorDoneCh <- 0
				return
			}

			doSyscall = false
			continue
		}

		if handleCall {
			var cstate *syscallState
			if _, ok := pidSyscallState[wpid]; ok {
				cstate = pidSyscallState[wpid]
			} else {
				log.Debugf("ptrace.App.collect: collector loop - new pid - mainPid=%v pid=%v (prevPid=%v) - add state", app.MainPID(), wpid, prevPid)
				//TODO: create new process records from clones/forks
				cstate = &syscallState{pid: wpid}
				pidSyscallState[wpid] = cstate
			}

			if !cstate.expectReturn {
				if err := onSyscall(wpid, cstate); err != nil {
					log.Debugf("ptrace.App.collect: onSyscall error - %v", err)
					continue
				}
			} else {
				if err := onSyscallReturn(wpid, cstate); err != nil {
					log.Debugf("ptrace.App.collect: onSyscallReturn error - %v", err)
					continue
				}
			}

			if cstate.gotCallNum && cstate.gotRetVal {
				cstate.gotCallNum = false
				cstate.gotRetVal = false

				evt := syscallEvent{
					pid:     wpid,
					callNum: uint32(cstate.callNum),
					retVal:  cstate.retVal,
				}

				select {
				case app.eventCh <- evt:
				default:
					log.Debugf("ptrace.App.collect: app.eventCh send error (%#v)", evt)
				}
			}
		}

		if eventStop {
			log.Debugf("ptrace.App.collect: eventStop eventCode=%d(0x%04x)", eventCode, eventCode)

			switch eventCode {
			case syscall.PTRACE_EVENT_CLONE,
				syscall.PTRACE_EVENT_FORK,
				syscall.PTRACE_EVENT_VFORK,
				syscall.PTRACE_EVENT_VFORK_DONE:
				newPid, err := syscall.PtraceGetEventMsg(wpid)
				if err != nil {
					log.Debugf("ptrace.App.collect: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - error getting cloned pid - %v", err)
				} else {
					log.Debugf("ptrace.App.collect: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - cloned pid - %v", newPid)
					if _, ok := pidSyscallState[int(newPid)]; ok {
						log.Debugf("ptrace.App.collect: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - pid already exists - %v", newPid)
						pidSyscallState[int(newPid)].started = true
					} else {
						pidSyscallState[int(newPid)] = &syscallState{pid: int(newPid), started: true}
					}
				}

			case syscall.PTRACE_EVENT_EXEC:
				oldPid, err := syscall.PtraceGetEventMsg(wpid)
				if err != nil {
					log.Debugf("ptrace.App.collect: PTRACE_EVENT_EXEC - error getting old pid - %v", err)
				} else {
					log.Debugf("ptrace.App.collect: PTRACE_EVENT_EXEC - old pid - %v", oldPid)
				}

			case syscall.PTRACE_EVENT_EXIT:
				log.Debugf("ptrace.App.collect: PTRACE_EVENT_EXIT - process exiting pid=%v", wpid)
				if app.MainPID() == wpid {
					mainExiting = true
					log.Debugf("ptrace.App.collect: main process is exiting (%v)", wpid)
				}

				if _, ok := pidSyscallState[wpid]; ok {
					pidSyscallState[wpid].exiting = true
				} else {
					log.Debugf("ptrace.App.collect: unknown process is exiting (%v)", wpid)
				}
			}
		}

		if stopped {
			callSig = statusCode
		}

		doSyscall = true
		callPid = wpid
	}

}

func onSyscall(pid int, cstate *syscallState) error {
	var regs syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
		return err
	}

	cstate.callNum = system.CallNumber(regs)
	cstate.expectReturn = true
	cstate.gotCallNum = true
	return nil
}

func onSyscallReturn(pid int, cstate *syscallState) error {
	var regs syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
		return err
	}

	cstate.retVal = system.CallReturnValue(regs)
	cstate.expectReturn = false
	cstate.gotRetVal = true
	return nil
}

///////////////////////////////////

func SignalEnum(sigNum int) string {
	if sigNum >= len(sigEnums) || sigNum < 0 {
		return fmt.Sprintf("BAD(%d)", sigNum)
	}

	e := sigEnums[sigNum]
	if e == "" {
		e = fmt.Sprintf("UNKNOWN(%d)", sigNum)
	}

	return e
}

var sigEnums = [...]string{
	0:                 "(NOSIGNAL)",
	syscall.SIGABRT:   "SIGABRT/SIGIOT",
	syscall.SIGALRM:   "SIGALRM",
	syscall.SIGBUS:    "SIGBUS",
	syscall.SIGCHLD:   "SIGCHLD",
	syscall.SIGCONT:   "SIGCONT",
	syscall.SIGFPE:    "SIGFPE",
	syscall.SIGHUP:    "SIGHUP",
	syscall.SIGILL:    "SIGILL",
	syscall.SIGINT:    "SIGINT",
	syscall.SIGKILL:   "SIGKILL",
	syscall.SIGPIPE:   "SIGPIPE",
	syscall.SIGPOLL:   "SIGIO/SIGPOLL",
	syscall.SIGPROF:   "SIGPROF",
	syscall.SIGPWR:    "SIGPWR",
	syscall.SIGQUIT:   "SIGQUIT",
	syscall.SIGSEGV:   "SIGSEGV",
	syscall.SIGSTKFLT: "SIGSTKFLT",
	syscall.SIGSTOP:   "SIGSTOP",
	syscall.SIGSYS:    "SIGSYS",
	syscall.SIGTERM:   "SIGTERM",
	syscall.SIGTRAP:   "SIGTRAP",
	syscall.SIGTSTP:   "SIGTSTP",
	syscall.SIGTTIN:   "SIGTTIN",
	syscall.SIGTTOU:   "SIGTTOU",
	syscall.SIGURG:    "SIGURG",
	syscall.SIGUSR1:   "SIGUSR1",
	syscall.SIGUSR2:   "SIGUSR2",
	syscall.SIGVTALRM: "SIGVTALRM",
	syscall.SIGWINCH:  "SIGWINCH",
	syscall.SIGXCPU:   "SIGXCPU",
	syscall.SIGXFSZ:   "SIGXFSZ",
}
