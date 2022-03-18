package ptrace

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/armon/go-radix"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/launcher"
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
	rtaSourcePT bool,
	cmd string,
	args []string,
	dir string,
	user string,
	runAsUser bool,
	reportCh chan *report.PtMonitorReport,
	errorCh chan error,
	stateCh chan AppState,
	stopCh chan struct{},
	includeNew bool,
	origPaths map[string]interface{},
) (*App, error) {
	log.Debug("ptrace.Run")
	app, err := newApp(rtaSourcePT, cmd, args, dir, user, runAsUser, reportCh, errorCh, stateCh, stopCh, includeNew, origPaths)
	if err != nil {
		app.StateCh <- AppFailed
		return nil, err
	}

	if rtaSourcePT {
		log.Debug("ptrace.Run - tracing target app")
		app.Report.Enabled = true
		go app.process()
		go app.trace()
	} else {
		log.Debug("ptrace.Run - not tracing target app...")
		go func() {
			log.Debug("ptrace.Run - not tracing target app - start app")
			err := app.start()
			if err != nil {
				log.Debugf("ptrace.Run - not tracing target app - start app error - %v", err)
				app.StateCh <- AppFailed
				app.ErrorCh <- errors.SE("ptrace.App.trace.app.start", "call.error", err)
				return
			}

			app.StateCh <- AppStarted

			time.Sleep(2 * time.Second)
			log.Debug("ptrace.Run - not tracing target app - state<-AppDone")
			app.StateCh <- AppDone
			app.ReportCh <- &app.Report
		}()
	}

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
	pathParam    string
	pathParamErr error
}

type App struct {
	RTASourcePT     bool
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
	fsActivity      map[string]*report.FSActivityInfo
	syscallActivity map[uint32]uint64
	//syscallResolver system.NumberResolverFunc
	cmd             *exec.Cmd
	pgid            int
	eventCh         chan syscallEvent
	collectorDoneCh chan int
	includeNew      bool
	origPaths       map[string]interface{}
}

func (a *App) MainPID() int {
	return a.cmd.Process.Pid
}

func (a *App) PGID() int {
	return a.pgid
}

const eventBufSize = 2000

type syscallEvent struct {
	pid       int
	callNum   uint32
	retVal    uint64
	pathParam string
}

func newApp(
	rtaSourcePT bool,
	cmd string,
	args []string,
	dir string,
	user string,
	runAsUser bool,
	reportCh chan *report.PtMonitorReport,
	errorCh chan error,
	stateCh chan AppState,
	stopCh chan struct{},
	includeNew bool,
	origPaths map[string]interface{}) (*App, error) {
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
		RTASourcePT:     rtaSourcePT,
		Cmd:             cmd,
		Args:            args,
		Dir:             dir,
		User:            user,
		RunAsUser:       runAsUser,
		ReportCh:        reportCh,
		ErrorCh:         errorCh,
		StateCh:         stateCh,
		StopCh:          stopCh,
		fsActivity:      map[string]*report.FSActivityInfo{},
		syscallActivity: map[uint32]uint64{},
		eventCh:         make(chan syscallEvent, eventBufSize),
		collectorDoneCh: make(chan int, 1),
		//syscallResolver: system.CallNumberResolver(archName),
		Report: report.PtMonitorReport{
			ArchName:     string(archName),
			SyscallStats: map[string]report.SyscallStatInfo{},
			FSActivity:   map[string]*report.FSActivityInfo{},
		},
		includeNew: includeNew,
		origPaths:  origPaths,
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

func (app *App) processSyscallActivity(e *syscallEvent) {
	app.syscallActivity[e.callNum]++
}

func (app *App) processFileActivity(e *syscallEvent) {
	if e.pathParam != "" {
		p, found := syscallProcessors[int(e.callNum)]
		if !found {
			log.Debug("ptrace.App.processFileActivity - no syscall processor - %#v", e)
			//shouldn't happen
			return
		}

		if (p.SyscallType() == CheckFileType ||
			p.SyscallType() == OpenFileType) &&
			!p.FailedReturnStatus(e.retVal) {
			//todo: filter "/proc/", "/sys/", "/dev/" externally
			if e.pathParam != "." &&
				e.pathParam != "/proc" &&
				!strings.HasPrefix(e.pathParam, "/proc/") &&
				!strings.HasPrefix(e.pathParam, "/sys/") &&
				!strings.HasPrefix(e.pathParam, "/dev/") {
				if fsa, ok := app.fsActivity[e.pathParam]; ok {
					fsa.OpsAll++
					fsa.Pids[e.pid] = struct{}{}
					fsa.Syscalls[int(e.callNum)] = struct{}{}

					if processor, found := syscallProcessors[int(e.callNum)]; found {
						switch processor.SyscallType() {
						case CheckFileType:
							fsa.OpsCheckFile++
						}
					}
				} else {
					fsa := &report.FSActivityInfo{
						OpsAll:       1,
						OpsCheckFile: 1,
						Pids:         map[int]struct{}{},
						Syscalls:     map[int]struct{}{},
					}

					fsa.Pids[e.pid] = struct{}{}
					fsa.Syscalls[int(e.callNum)] = struct{}{}

					app.fsActivity[e.pathParam] = fsa
				}
			}
		}
	}
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

			app.processSyscallActivity(&e)
			app.processFileActivity(&e)
		}
	}

	log.Debugf("ptrace.App.process: - executed syscall count = %d", app.Report.SyscallCount)
	log.Debugf("ptrace.App.process: - number of syscalls: %v", len(app.syscallActivity))

	for scNum, scCount := range app.syscallActivity {
		//syscallName := app.syscallResolver(scNum)
		syscallName := system.LookupCallName(scNum)
		log.Debugf("[%v] %v = %v", scNum, syscallName, scCount)
		scKey := strconv.FormatInt(int64(scNum), 10)
		app.Report.SyscallStats[scKey] = report.SyscallStatInfo{
			Number: scNum,
			Name:   syscallName,
			Count:  scCount,
		}
	}

	app.Report.SyscallNum = uint32(len(app.Report.SyscallStats))
	app.Report.FSActivity = app.FileActivity()

	app.StateCh <- state
	app.ReportCh <- &app.Report
}

func (app *App) FileActivity() map[string]*report.FSActivityInfo {
	log.Debugf("ptrace.App.FileActivity [all records - %d]", len(app.fsActivity))
	//get the file activity info (ignore intermediate directories)
	t := radix.New()
	for k, v := range app.fsActivity {
		t.Insert(k, v)
	}

	walk := func(wkey string, wv interface{}) bool {
		wdata, ok := wv.(*report.FSActivityInfo)
		if !ok {
			return false
		}

		walkAfter := func(akey string, av interface{}) bool {
			//adata, ok := av.(*report.FSActivityInfo)
			//if !ok {
			//    return false
			//}

			if wkey == akey {
				return false
			}

			wdata.IsSubdir = true
			return true
		}

		t.WalkPrefix(wkey, walkAfter)
		return false
	}

	t.Walk(walk)

	result := map[string]*report.FSActivityInfo{}
	for k, v := range app.fsActivity {
		if v.IsSubdir {
			continue
		}

		result[k] = v
	}

	log.Debugf("ptrace.App.FileActivity [file records - %d]", len(result))
	return result
}

func (app *App) start() error {
	log.Debug("ptrace.App.start")
	var err error
	app.cmd, err = launcher.Start(app.Cmd, app.Args, app.Dir, app.User, app.RunAsUser, app.RTASourcePT)
	if err != nil {
		log.Errorf("ptrace.App.start: cmd='%v' args='%+v' dir='%v' error=%v\n",
			app.Cmd, app.Args, app.Dir, err)
		return err
	}

	if !app.RTASourcePT {
		app.pgid, err = syscall.Getpgid(app.cmd.Process.Pid)
		if err != nil {
			return err
		}

		return nil
	}

	err = app.cmd.Wait()
	log.Debugf("ptrace.App.start: app.cmd.Wait err - %v", err)
	log.Debugf("ptrace.App.start: Target process state info - Exited=%v ExitCode=%v SysWaitStatus=%v",
		app.cmd.ProcessState.Exited(),
		app.cmd.ProcessState.ExitCode(),
		app.cmd.ProcessState.Sys())

	waitStatus, ok := app.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ok {
		log.Debugf("ptrace.App.start: Target process wait status - %v (Exited=%v Signaled=%v Signal='%v' Stopped=%v StopSignal='%v' TrapCause=%v)",
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

	log.Debugf("ptrace.App.collect: trace syscall mainPID=%v", callPid)

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
				log.Debugf("ptrace.App.collect[%d/%d]: unknown process is terminated (pid=%v)",
					app.cmd.Process.Pid, app.pgid, wpid)
			} else {
				if !pidSyscallState[wpid].exiting {
					log.Debugf("ptrace.App.collect[%d/%d]: unexpected process termination (pid=%v)",
						app.cmd.Process.Pid, app.pgid, wpid)
				}
			}

			delete(pidSyscallState, wpid)
			if app.MainPID() == wpid {
				log.Debugf("ptrace.App.collect[%d/%d]: wpid(%v) is main PID and terminated...",
					app.cmd.Process.Pid, app.pgid, wpid)
				if !mainExiting {
					log.Debug("ptrace.App.collect: unexpected main PID termination...")
				}
			}

			if len(pidSyscallState) == 0 {
				log.Debugf("ptrace.App.collect[%d/%d]: all processes terminated...", app.cmd.Process.Pid, app.pgid)
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
				log.Debugf("ptrace.App.collect[%d/%d]: collector loop - new pid - mainPid=%v pid=%v (prevPid=%v) - add state",
					app.cmd.Process.Pid, app.pgid, app.MainPID(), wpid, prevPid)
				//TODO: create new process records from clones/forks
				cstate = &syscallState{pid: wpid}
				pidSyscallState[wpid] = cstate
			}

			if !cstate.expectReturn {
				genEvent, err := onSyscall(wpid, cstate)
				if err != nil {
					log.Debugf("ptrace.App.collect[%d/%d]: wpid=%v onSyscall error - %v",
						app.cmd.Process.Pid, app.pgid, wpid, err)
					continue
				}

				if genEvent {
					evt := syscallEvent{
						pid:       wpid,
						callNum:   uint32(cstate.callNum),
						pathParam: cstate.pathParam,
					}

					log.Debugf("ptrace.App.collect[%d/%d]: event.onSyscall - wpid=%v (evt=%#v)",
						app.cmd.Process.Pid, app.pgid, wpid, evt)
					select {
					case app.eventCh <- evt:
					default:
						log.Debugf("ptrace.App.collect[%d/%d]: event.onSyscall - wpid=%v app.eventCh send error (evt=%#v)",
							app.cmd.Process.Pid, app.pgid, wpid, evt)
					}
				}
			} else {
				if err := onSyscallReturn(wpid, cstate); err != nil {
					log.Debugf("ptrace.App.collect[%d/%d]: wpid=%v onSyscallReturn error - %v",
						app.cmd.Process.Pid, app.pgid, wpid, err)
					continue
				}
			}

			if cstate.gotCallNum && cstate.gotRetVal {
				evt := syscallEvent{
					pid:       wpid,
					callNum:   uint32(cstate.callNum),
					retVal:    cstate.retVal,
					pathParam: cstate.pathParam,
				}

				cstate.gotCallNum = false
				cstate.gotRetVal = false
				cstate.pathParam = ""
				cstate.pathParamErr = nil

				_, ok := app.origPaths[evt.pathParam]
				if app.includeNew {
					ok = true
				}

				if ok {
					select {
					case app.eventCh <- evt:
					default:
						log.Debugf("ptrace.App.collect[%d/%d]: wpid=%v app.eventCh send error (evt=%#v)",
							app.cmd.Process.Pid, app.pgid, wpid, evt)
					}
				}
			}
		}

		if eventStop {
			log.Debugf("ptrace.App.collect[%d/%d]: eventStop eventCode=%d(0x%04x)",
				app.cmd.Process.Pid, app.pgid, eventCode, eventCode)

			switch eventCode {
			case syscall.PTRACE_EVENT_CLONE,
				syscall.PTRACE_EVENT_FORK,
				syscall.PTRACE_EVENT_VFORK,
				syscall.PTRACE_EVENT_VFORK_DONE:
				newPid, err := syscall.PtraceGetEventMsg(wpid)
				if err != nil {
					log.Debugf("ptrace.App.collect[%d/%d]: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - error getting cloned pid - %v",
						app.cmd.Process.Pid, app.pgid, err)
				} else {
					log.Debugf("ptrace.App.collect[%d/%d]: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - cloned pid - %v",
						app.cmd.Process.Pid, app.pgid, newPid)
					if _, ok := pidSyscallState[int(newPid)]; ok {
						log.Debugf("ptrace.App.collect[%d/%d]: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - pid already exists - %v",
							app.cmd.Process.Pid, app.pgid, newPid)
						pidSyscallState[int(newPid)].started = true
					} else {
						pidSyscallState[int(newPid)] = &syscallState{pid: int(newPid), started: true}
					}
				}

			case syscall.PTRACE_EVENT_EXEC:
				oldPid, err := syscall.PtraceGetEventMsg(wpid)
				if err != nil {
					log.Debugf("ptrace.App.collect[%d/%d]: PTRACE_EVENT_EXEC - error getting old pid - %v",
						app.cmd.Process.Pid, app.pgid, err)
				} else {
					log.Debugf("ptrace.App.collect[%d/%d]: PTRACE_EVENT_EXEC - old pid - %v",
						app.cmd.Process.Pid, app.pgid, oldPid)
				}

			case syscall.PTRACE_EVENT_EXIT:
				log.Debugf("ptrace.App.collect[%d/%d]: PTRACE_EVENT_EXIT - process exiting pid=%v",
					app.cmd.Process.Pid, app.pgid, wpid)
				if app.MainPID() == wpid {
					mainExiting = true
					log.Debugf("ptrace.App.collect[%d/%d]: main process is exiting (%v)",
						app.cmd.Process.Pid, app.pgid, wpid)
				}

				if _, ok := pidSyscallState[wpid]; ok {
					pidSyscallState[wpid].exiting = true
				} else {
					log.Debugf("ptrace.App.collect[%d/%d]: unknown process is exiting (pid=%v)",
						app.cmd.Process.Pid, app.pgid, wpid)
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

func onSyscall(pid int, cstate *syscallState) (bool, error) {
	var regs syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
		return false, err
	}

	cstate.callNum = system.CallNumber(regs)
	cstate.expectReturn = true
	cstate.gotCallNum = true

	if processor, found := syscallProcessors[int(cstate.callNum)]; found && processor != nil {
		processor.OnCall(pid, regs, cstate)
		genEvent := processor.EventOnCall()
		return genEvent, nil
	}

	return false, nil
}

func onSyscallReturn(pid int, cstate *syscallState) error {
	var regs syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
		return err
	}

	cstate.retVal = system.CallReturnValue(regs)
	cstate.expectReturn = false
	cstate.gotRetVal = true

	if processor, found := syscallProcessors[int(cstate.callNum)]; found {
		processor.OnReturn(pid, regs, cstate)
	}

	return nil
}

///////////////////////////////////

func getIntParam(pid int, ptr uint64) int {
	return int(int32(ptr))
}

func getStringParam(pid int, ptr uint64) string {
	var out [256]byte
	var data []byte
	for {
		count, err := syscall.PtracePeekData(pid, uintptr(ptr), out[:])
		if err != nil && err != syscall.EIO {
			fmt.Printf("readString: syscall.PtracePeekData error - '%v'\v", err)
		}

		idx := bytes.IndexByte(out[:count], 0)
		var foundNull bool
		if idx == -1 {
			idx = count
			ptr += uint64(count)
		} else {
			foundNull = true
		}

		data = append(data, out[:idx]...)
		if foundNull {
			return string(data)
		}
	}
}

type SyscallTypeName string

const (
	CheckFileType SyscallTypeName = "type.checkfile"
	OpenFileType  SyscallTypeName = "type.openfile"
	ExecType      SyscallTypeName = "type.exec"
)

type SyscallProcessor interface {
	SyscallNumber() uint64
	SetSyscallNumber(uint64)
	SyscallType() SyscallTypeName
	SyscallName() string
	EventOnCall() bool
	OnCall(pid int, regs syscall.PtraceRegs, cstate *syscallState)
	OnReturn(pid int, regs syscall.PtraceRegs, cstate *syscallState)
	FailedCall(cstate *syscallState) bool
	FailedReturnStatus(retVal uint64) bool
}

type StringParamPos int

type syscallProcessorCore struct {
	Num         uint64
	Name        string
	Type        SyscallTypeName
	StringParam StringParamPos
}

const (
	SPPNo  StringParamPos = 0
	SPPOne StringParamPos = 1
	SPPTwo StringParamPos = 2
)

func (ref *syscallProcessorCore) SyscallNumber() uint64 {
	return ref.Num
}

func (ref *syscallProcessorCore) SetSyscallNumber(num uint64) {
	ref.Num = num
}

func (ref *syscallProcessorCore) SyscallType() SyscallTypeName {
	return ref.Type
}

func (ref *syscallProcessorCore) SyscallName() string {
	return ref.Name
}

func (ref *syscallProcessorCore) OnCall(pid int, regs syscall.PtraceRegs, cstate *syscallState) {
	pth := ""
	dir := ""
	cwd, _ := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))

	switch ref.StringParam {
	case SPPNo:
		log.Tracef("syscallProcessorCore.OnCall[%s/%d]: pid=%d - no string param", ref.Name, ref.Num, pid)
		return
	case SPPOne:
		pth = getStringParam(pid, system.CallFirstParam(regs))
	case SPPTwo:
		fd := getIntParam(pid, system.CallFirstParam(regs))
		if fd > 0 {
			dir, _ = os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", pid, fd))
		}
		pth = getStringParam(pid, system.CallSecondParam(regs))
	default:
		panic("unreachable")
	}
	if len(pth) > 0 && pth[0] != '/' {
		if dir != "" {
			pth = path.Join(dir, pth)
		} else if cwd != "" {
			pth = path.Join(cwd, pth)
		}
	}

	cstate.pathParam = pth
}

func (ref *syscallProcessorCore) OnReturn(pid int, regs syscall.PtraceRegs, cstate *syscallState) {
	//for stat/check syscalls:
	//-2 => -ENOENT (No such file or directory)
	//TODO: get/use error code enums

	if cstate.pathParamErr == nil {
		log.Debugf("checkFileSyscallProcessor.OnReturn: [%d] {%d}%s('%s') = %d", pid, cstate.callNum, ref.Name, cstate.pathParam, int(cstate.retVal))
	} else {
		log.Debugf("checkFileSyscallProcessor.OnReturn: [%d] {%d}%s(<unknown>/'%s') = %d [pp err => %v]", pid, cstate.callNum, ref.Name, cstate.pathParam, int(cstate.retVal), cstate.pathParamErr)
	}
}

type checkFileSyscallProcessor struct {
	*syscallProcessorCore
}

func (ref *checkFileSyscallProcessor) FailedCall(cstate *syscallState) bool {
	return cstate.retVal != 0
}

func (ref *checkFileSyscallProcessor) FailedReturnStatus(retVal uint64) bool {
	return retVal != 0
}

func (ref *checkFileSyscallProcessor) EventOnCall() bool {
	return false
}

type openFileSyscallProcessor struct {
	*syscallProcessorCore
}

func (ref *openFileSyscallProcessor) FailedCall(cstate *syscallState) bool {
	return cstate.retVal < 0
}

func (ref *openFileSyscallProcessor) FailedReturnStatus(retVal uint64) bool {
	return retVal < 0
}

func (ref *openFileSyscallProcessor) EventOnCall() bool {
	return false
}

type execSyscallProcessor struct {
	*syscallProcessorCore
}

func (ref *execSyscallProcessor) FailedCall(cstate *syscallState) bool {
	return cstate.retVal < 0
}

func (ref *execSyscallProcessor) FailedReturnStatus(retVal uint64) bool {
	return retVal < 0
}

func (ref *execSyscallProcessor) EventOnCall() bool {
	return true
}

//TODO: introduce syscall num and name consts to use instead of liternal values
var syscallProcessors = map[int]SyscallProcessor{}

func init() {
	//stat(const char *filename,struct stat *statbuf)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "stat",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//lstat(fconst char *filename, struct stat *statbuf)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "lstat",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//access(const char *filename, int mode)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "access",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//statfs(const char *pathname, struct statfs *buf)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "statfs",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//readlink(const char *path, char *buf, int bufsiz)
	//on success, returns the number of bytes placed in buf. On error, returns -1
	//reusing "open" processor because the call error return values are similar
	addSyscallProcessor(&openFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "readlink",
			Type:        OpenFileType,
			StringParam: SPPOne,
		},
	})
	//utime(char *filename, struct utimbuf *times)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "utime",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//utimes(char *filename, struct timeval *utimes)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "utimes",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//chdir(const char *filename)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "chdir",
			Type:        CheckFileType,
			StringParam: SPPOne,
		},
	})
	//open(const char *filename, int flags, int mode)
	addSyscallProcessor(&openFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "open",
			Type:        OpenFileType,
			StringParam: SPPOne,
		},
	})
	//readlinkat(int dfd, const char *pathname, char *buf, int bufsiz)
	//on success, returns the number of bytes placed in buf. On error, returns -1
	//reusing "open" processor because the call error return values are similar
	addSyscallProcessor(&openFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "readlinkat",
			Type:        OpenFileType,
			StringParam: SPPTwo,
		},
	})
	//faccessat(int dfd, const char *filename, int mode)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "faccessat",
			Type:        CheckFileType,
			StringParam: SPPTwo,
		},
	})
	//openat(int dfd, const char *filename, int flags, int mode)
	addSyscallProcessor(&openFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "openat",
			Type:        OpenFileType,
			StringParam: SPPTwo,
		},
	})
	//futimesat(int dfd, const char *filename, struct timeval *utimes)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "futimesat",
			Type:        CheckFileType,
			StringParam: SPPTwo,
		},
	})
	//statx(int dirfd, const char *pathname, int flags, unsigned int mask, struct statx *statxbuf)
	//dirfd: AT_FDCWD
	//flags: AT_EMPTY_PATH
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "statx",
			Type:        CheckFileType,
			StringParam: SPPTwo,
		},
	})

	//execve(const char *pathname, char *const argv[], char *const envp[])
	addSyscallProcessor(&execSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "execve",
			Type:        ExecType,
			StringParam: SPPOne,
		},
	})
	//execveat(int dirfd, const char *pathname, const char *const argv[], const char *const envp[], int flags)
	addSyscallProcessor(&execSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "execveat",
			Type:        ExecType,
			StringParam: SPPTwo,
		},
	})
}

func addSyscallProcessor(p SyscallProcessor) {
	num, found := system.LookupCallNumber(p.SyscallName())
	if !found {
		log.Errorf("addSyscallProcessor: unknown syscall='%s'", p.SyscallName())
		return
	}

	p.SetSyscallNumber(uint64(num))
	syscallProcessors[int(p.SyscallNumber())] = p
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
