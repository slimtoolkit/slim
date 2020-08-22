package ptrace

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	//"os"

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
	stateCh chan AppState, //was ackChan chan<- bool
	stopCh chan struct{}, //was stopChan
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

/* same as report.SyscallStatInfo:
type CallInfo struct {
    Number int
    Name   string
    Count  uint64
}
*/

/*
type AppReport struct {
    ArchName  string        //ArchName
    SyscallTypeCount uint32 //SyscallNum
    CallStats map[string]CallInfo //SyscallStats
    SyscallCount uint64     //SyscallCount
}
*/

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
			//CallStats: map[string]CallInfo{},
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

/*
type NumberResolverFunc func(uint32) string
func CallNumberResolver() NumberResolverFunc {
    return callNameX86Family64
}
*/

func (app *App) process() {
	log.Debug("ptrace.App.process")
	//syscallResolver := CallNumberResolver()
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
		log.Errorf("ptrace.App.start: error=%v\n", err)
		return err
	}

	err = app.cmd.Wait()
	log.Debugf("ptrace.App.start: app.cmd.Wait err - %v", err)
	//State: stop signal: trace/breakpoint trap
	//app.Wait() does state, err := c.Process.Wait()
	//c.ProcessState = state
	//if !state.Success()
	//&ExitError{ProcessState: state}
	//Process *os.Process
	//ProcessState *os.ProcessState
	//(p *ProcessState) Sys() interface{}
	//system-dependent exit information / syscall.WaitStatus
	log.Debugf("ptrace.App.start: Process state info - Exited=%v ExitCode=%v SysWaitStatus=%v",
		app.cmd.ProcessState.Exited(),
		app.cmd.ProcessState.ExitCode(),
		app.cmd.ProcessState.Sys())
	//Process state info: Exited=false ExitCode=-1 SysWaitStatus=1407
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
		//Process wait status: 1407 (
		//	Signaled=false
		//	Signal=signal -1
		//	Stopped=true
		//	StopSignal=trace/breakpoint trap
		//	TrapCause=0)

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
			//Process Stop Signal: code=5 enum=SIGTRAP str=trace/breakpoint trap
			//SIGTRAP	5	Trace/Breakpoint Trap
			//used from within debuggers and program tracers
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
	waitFor := -1 // -1 * app.pgid
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

/*
func getCallNumber(regs syscall.PtraceRegs) uint64 {
    return regs.Orig_rax
}

func getCallReturnValue(regs syscall.PtraceRegs) uint64 {
    return regs.Rax
}
*/

///////////////////////////////////

/*
func New() (*Engine, error) {
	e := Engine{}

	return &e, nil
}

func (e *Engine) StartApp(argv []string, dir string) (*App, error) {
	fmt.Printf("ptrace.Engine.StartApp(%v,%v)\n", argv, dir)
	app, err := newApp(dir, argv[0],argv[1:])
	if err != nil {
		return nil, err
	}

	err = app.Start()
	if err != nil {
		return nil, err
	}

	//app := exec.Command(argv[0],argv[1:]...)

	//app.SysProcAttr = &syscall.SysProcAttr{
	//	Ptrace:    true, //process will stop and send SIGSTOP signal to its parent before start.
	//	Setpgid:   true,
	//	Pdeathsig: syscall.SIGKILL,
	//}

	//app.Dir = dir
	//app.Stdout = os.Stdout
	//app.Stderr = os.Stderr
	//app.Stdin = os.Stdin

	//err := app.Start()
	//if err != nil {
	//	fmt.Printf("ptrace.Engine.StartApp: error=%v\n", err)
	//	return nil, err
	//}



    //PTRACE_O_TRACECLONE:
    //Thanks to that our debugger knows when new thread has been started
    //
    //http://man7.org/linux/man-pages/man2/ptrace.2.html
    //PTRACE_O_TRACECLONE (since Linux 2.5.46)
    //Stop the tracee at the next clone(2) and automatically start tracing
    //the newly cloned process, which will
    //start with a SIGSTOP, or PTRACE_EVENT_STOP if PTRACE_SEIZE was used
    //A waitpid(2) by the tracer will return a "status" value such that:
    // status>>8 == (SIGTRAP | (PTRACE_EVENT_CLONE<<8))
    //The PID of the new process can be retrieved with PTRACE_GETEVENTMSG.
    //
    //This option may not catch clone(2) calls in all cases.
    //If the tracee calls clone(2) with the CLONE_VFORK flag,
    //PTRACE_EVENT_VFORK will be delivered instead if
    //PTRACE_O_TRACEVFORK is set; otherwise if the tracee
    //calls clone(2) with the exit signal set to SIGCHLD,
    //PTRACE_EVENT_FORK will be delivered if PTRACE_O_TRACE‐FORK is set.

	return app, nil
}
*/

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

/*
const SyscallX86UnknownName = "unknown_syscall"

func callNameX86Family64(num uint32) string {
    if num > SyscallX86MaxNum64 {
        return SyscallX86UnknownName
    }

    return syscallNumTableX86Family64[num]
}

const (
    SyscallX86MaxNum64   = 322
    SyscallX86LastName64 = "execveat"
)

var syscallNumTableX86Family64 = [...]string{
    "read",
    "write",
    "open",
    "close",
    "stat",
    "fstat",
    "lstat",
    "poll",
    "lseek",
    "mmap",
    "mprotect",
    "munmap",
    "brk",
    "rt_sigaction",
    "rt_sigprocmask",
    "rt_sigreturn",
    "ioctl",
    "pread64",
    "pwrite64",
    "readv",
    "writev",
    "access",
    "pipe",
    "select",
    "sched_yield",
    "mremap",
    "msync",
    "mincore",
    "madvise",
    "shmget",
    "shmat",
    "shmctl",
    "dup",
    "dup2",
    "pause",
    "nanosleep",
    "getitimer",
    "alarm",
    "setitimer",
    "getpid",
    "sendfile",
    "socket",
    "connect",
    "accept",
    "sendto",
    "recvfrom",
    "sendmsg",
    "recvmsg",
    "shutdown",
    "bind",
    "listen",
    "getsockname",
    "getpeername",
    "socketpair",
    "setsockopt",
    "getsockopt",
    "clone",
    "fork",
    "vfork",
    "execve",
    "exit",
    "wait4",
    "kill",
    "uname",
    "semget",
    "semop",
    "semctl",
    "shmdt",
    "msgget",
    "msgsnd",
    "msgrcv",
    "msgctl",
    "fcntl",
    "flock",
    "fsync",
    "fdatasync",
    "truncate",
    "ftruncate",
    "getdents",
    "getcwd",
    "chdir",
    "fchdir",
    "rename",
    "mkdir",
    "rmdir",
    "creat",
    "link",
    "unlink",
    "symlink",
    "readlink",
    "chmod",
    "fchmod",
    "chown",
    "fchown",
    "lchown",
    "umask",
    "gettimeofday",
    "getrlimit",
    "getrusage",
    "sysinfo",
    "times",
    "ptrace",
    "getuid",
    "syslog",
    "getgid",
    "setuid",
    "setgid",
    "geteuid",
    "getegid",
    "setpgid",
    "getppid",
    "getpgrp",
    "setsid",
    "setreuid",
    "setregid",
    "getgroups",
    "setgroups",
    "setresuid",
    "getresuid",
    "setresgid",
    "getresgid",
    "getpgid",
    "setfsuid",
    "setfsgid",
    "getsid",
    "capget",
    "capset",
    "rt_sigpending",
    "rt_sigtimedwait",
    "rt_sigqueueinfo",
    "rt_sigsuspend",
    "sigaltstack",
    "utime",
    "mknod",
    "uselib",
    "personality",
    "ustat",
    "statfs",
    "fstatfs",
    "sysfs",
    "getpriority",
    "setpriority",
    "sched_setparam",
    "sched_getparam",
    "sched_setscheduler",
    "sched_getscheduler",
    "sched_get_priority_max",
    "sched_get_priority_min",
    "sched_rr_get_interval",
    "mlock",
    "munlock",
    "mlockall",
    "munlockall",
    "vhangup",
    "modify_ldt",
    "pivot_root",
    "_sysctl",
    "prctl",
    "arch_prctl",
    "adjtimex",
    "setrlimit",
    "chroot",
    "sync",
    "acct",
    "settimeofday",
    "mount",
    "umount2",
    "swapon",
    "swapoff",
    "reboot",
    "sethostname",
    "setdomainname",
    "iopl",
    "ioperm",
    "create_module",
    "init_module",
    "delete_module",
    "get_kernel_syms",
    "query_module",
    "quotactl",
    "nfsservctl",
    "getpmsg",
    "putpmsg",
    "afs_syscall",
    "tuxcall",
    "security",
    "gettid",
    "readahead",
    "setxattr",
    "lsetxattr",
    "fsetxattr",
    "getxattr",
    "lgetxattr",
    "fgetxattr",
    "listxattr",
    "llistxattr",
    "flistxattr",
    "removexattr",
    "lremovexattr",
    "fremovexattr",
    "tkill",
    "time",
    "futex",
    "sched_setaffinity",
    "sched_getaffinity",
    "set_thread_area",
    "io_setup",
    "io_destroy",
    "io_getevents",
    "io_submit",
    "io_cancel",
    "get_thread_area",
    "lookup_dcookie",
    "epoll_create",
    "epoll_ctl_old",
    "epoll_wait_old",
    "remap_file_pages",
    "getdents64",
    "set_tid_address",
    "restart_syscall",
    "semtimedop",
    "fadvise64",
    "timer_create",
    "timer_settime",
    "timer_gettime",
    "timer_getoverrun",
    "timer_delete",
    "clock_settime",
    "clock_gettime",
    "clock_getres",
    "clock_nanosleep",
    "exit_group",
    "epoll_wait",
    "epoll_ctl",
    "tgkill",
    "utimes",
    "vserver",
    "mbind",
    "set_mempolicy",
    "get_mempolicy",
    "mq_open",
    "mq_unlink",
    "mq_timedsend",
    "mq_timedreceive",
    "mq_notify",
    "mq_getsetattr",
    "kexec_load",
    "waitid",
    "add_key",
    "request_key",
    "keyctl",
    "ioprio_set",
    "ioprio_get",
    "inotify_init",
    "inotify_add_watch",
    "inotify_rm_watch",
    "migrate_pages",
    "openat",
    "mkdirat",
    "mknodat",
    "fchownat",
    "futimesat",
    "newfstatat",
    "unlinkat",
    "renameat",
    "linkat",
    "symlinkat",
    "readlinkat",
    "fchmodat",
    "faccessat",
    "pselect6",
    "ppoll",
    "unshare",
    "set_robust_list",
    "get_robust_list",
    "splice",
    "tee",
    "sync_file_range",
    "vmsplice",
    "move_pages",
    "utimensat",
    "epoll_pwait",
    "signalfd",
    "timerfd_create",
    "eventfd",
    "fallocate",
    "timerfd_settime",
    "timerfd_gettime",
    "accept4",
    "signalfd4",
    "eventfd2",
    "epoll_create1",
    "dup3",
    "pipe2",
    "inotify_init1",
    "preadv",
    "pwritev",
    "rt_tgsigqueueinfo",
    "perf_event_open",
    "recvmmsg",
    "fanotify_init",
    "fanotify_mark",
    "prlimit64",
    "name_to_handle_at",
    "open_by_handle_at",
    "clock_adjtime",
    "syncfs",
    "sendmmsg",
    "setns",
    "getcpu",
    "process_vm_readv",
    "process_vm_writev",
    "kcmp",
    "finit_module",
    "sched_setattr",
    "sched_getattr",
    "renameat2",
    "seccomp",
    "getrandom",
    "memfd_create",
    "kexec_file_load",
    "bpf",
    "execveat",
    "userfaultfd",
    "membarrier",
    "mlock2",
    "copy_file_range",
    "preadv2",
    "pwritev2",
    "pkey_mprotect",
    "pkey_alloc",
    "pkey_free",
    "statx",
    "io_pgetevents",
    "rseq",
    "reserved.335",
    "reserved.336",
    "reserved.337",
    "reserved.338",
    "reserved.339",
    "reserved.340",
    "reserved.341",
    "reserved.342",
    "reserved.343",
    "reserved.344",
    "reserved.345",
    "reserved.346",
    "reserved.347",
    "reserved.348",
    "reserved.349",
    "reserved.350",
    "reserved.351",
    "reserved.352",
    "reserved.353",
    "reserved.354",
    "reserved.355",
    "reserved.356",
    "reserved.357",
    "reserved.358",
    "reserved.359",
    "reserved.360",
    "reserved.361",
    "reserved.362",
    "reserved.363",
    "reserved.364",
    "reserved.365",
    "reserved.366",
    "reserved.367",
    "reserved.368",
    "reserved.369",
    "reserved.370",
    "reserved.371",
    "reserved.372",
    "reserved.373",
    "reserved.374",
    "reserved.375",
    "reserved.376",
    "reserved.377",
    "reserved.378",
    "reserved.379",
    "reserved.380",
    "reserved.381",
    "reserved.382",
    "reserved.383",
    "reserved.384",
    "reserved.385",
    "reserved.386",
    "reserved.387",
    "reserved.388",
    "reserved.389",
    "reserved.390",
    "reserved.391",
    "reserved.392",
    "reserved.393",
    "reserved.394",
    "reserved.395",
    "reserved.396",
    "reserved.397",
    "reserved.398",
    "reserved.399",
    "reserved.400",
    "reserved.401",
    "reserved.402",
    "reserved.403",
    "reserved.404",
    "reserved.405",
    "reserved.406",
    "reserved.407",
    "reserved.408",
    "reserved.409",
    "reserved.410",
    "reserved.411",
    "reserved.412",
    "reserved.413",
    "reserved.414",
    "reserved.415",
    "reserved.416",
    "reserved.417",
    "reserved.418",
    "reserved.419",
    "reserved.420",
    "reserved.421",
    "reserved.422",
    "reserved.423",
    "pidfd_send_signal",
    "io_uring_setup",
    "io_uring_enter",
    "io_uring_register",
    "open_tree",
    "move_mount",
    "fsopen",
    "fsconfig",
    "fsmount",
    "fspick",
    "pidfd_open",
    "clone3", //435
}
*/

/*
https://medium.com/golangspec/making-debugger-in-golang-part-ii-d2b8eb2f19e0
*basic info

https://go.googlesource.com/debug/+/d6f6c5dad7f1bd8eb4e0c83eda26c972299d76db/ogle/demo/ptrace-linux-amd64/main.go
*LOOKS LIKE A GOOD REFERENCE PTRACE LOOP CODE
*TOO BAD IT'S SINGLE STEP CODE...

MORE NICE SNIPPETS:
https://golang.hotexamples.com/examples/syscall/SysProcAttr/Ptrace/golang-sysprocattr-ptrace-method-examples.html

https://github.com/subgraph/oz
Oz is a sandboxing system targeting everyday workstation applications.

https://github.com/subgraph/oz/blob/master/oz-seccomp/tracer.go
*main ptrace code / nice
https://github.com/subgraph/oz/blob/master/oz-seccomp/syscalls_args_amd64.go
https://github.com/subgraph/oz/blob/master/oz-seccomp/util.go
https://github.com/subgraph/oz/blob/master/oz-seccomp/syscall_util_amd64.go

*/
