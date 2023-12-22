//go:build !arm64
// +build !arm64

package ptrace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/armon/go-radix"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/slimtoolkit/slim/pkg/errors"
	"github.com/slimtoolkit/slim/pkg/launcher"
	"github.com/slimtoolkit/slim/pkg/mondel"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/system"
)

type AppState string

const (
	AppStarted AppState = "app.started"
	AppFailed  AppState = "app.failed"
	AppDone    AppState = "app.done"
)

const (
	cwdFD = unix.AT_FDCWD // AT_FDCWD = -0x64
)

func Run(
	ctx context.Context,
	del mondel.Publisher,
	runOpt AppRunOpt,
	// TODO(ivan): includeNew & origPaths logic should be applied at the artifact dumping stage.
	includeNew bool,
	origPaths map[string]struct{},
	signalCh <-chan os.Signal,
	errorCh chan<- error,
) (*App, error) {
	logger := log.WithFields(log.Fields{
		"com": "pt",
		"op":  "Run",
	})

	logger.Debug("call")
	defer logger.Debug("exit")

	app, err := newApp(ctx, del, runOpt, includeNew, origPaths, signalCh, errorCh)
	if err != nil {
		app.StateCh <- AppFailed
		return nil, err
	}

	if runOpt.RTASourcePT {
		logger.Debug("tracing target app")
		app.Report.Enabled = true
		go app.process()
		go app.trace()
	} else {
		logger.Debug("not tracing target app...")
		go func() {
			logger.Debug("not tracing target app - start app")
			if err := app.start(); err != nil {
				logger.Debugf("not tracing target app - start app error - %v", err)
				app.errorCh <- errors.SE("ptrace.App.trace.app.start", "call.error", err)
				app.StateCh <- AppFailed
				return
			}

			cancelSignalForwarding := app.startSignalForwarding()
			defer cancelSignalForwarding()

			app.StateCh <- AppStarted

			if err := app.cmd.Wait(); err != nil {
				logger.WithError(err).Debug("not tracing target app - state<-AppFailed")
				app.StateCh <- AppFailed
			} else {
				logger.Debug("not tracing target app - state<-AppDone")
				app.StateCh <- AppDone
			}

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
	ctx       context.Context
	Cmd       string
	Args      []string
	WorkDir   string
	User      string
	RunAsUser bool

	del mondel.Publisher

	appStdout io.Writer
	appStderr io.Writer

	RTASourcePT         bool
	reportOnMainPidExit bool

	includeNew bool
	origPaths  map[string]struct{}

	signalCh <-chan os.Signal
	errorCh  chan<- error

	StateCh  chan AppState
	ReportCh chan *report.PtMonitorReport

	cmd    *exec.Cmd
	pgid   int
	Report report.PtMonitorReport

	fsActivity      map[string]*report.FSActivityInfo
	syscallActivity map[uint32]uint64
	//syscallResolver system.NumberResolverFunc

	eventCh         chan syscallEvent
	collectorDoneCh chan int

	logger *log.Entry
}

func (a *App) MainPID() int {
	return a.cmd.Process.Pid
}

func (a *App) PGID() int {
	return a.pgid
}

const eventBufSize = 2000

type syscallEvent struct {
	returned  bool
	pid       int
	callNum   uint32
	retVal    uint64
	pathParam string
}

func newApp(
	ctx context.Context,
	del mondel.Publisher,
	runOpt AppRunOpt,
	includeNew bool,
	origPaths map[string]struct{},
	signalCh <-chan os.Signal,
	errorCh chan<- error,
) (*App, error) {
	logger := log.WithFields(log.Fields{
		"com": "pt.App",
	})

	sysInfo := system.GetSystemInfo()
	archName := system.MachineToArchName(sysInfo.Machine)

	a := App{
		ctx:       ctx,
		del:       del,
		Cmd:       runOpt.Cmd,
		Args:      runOpt.Args,
		WorkDir:   runOpt.WorkDir,
		User:      runOpt.User,
		RunAsUser: runOpt.RunAsUser,

		appStdout: runOpt.AppStdout,
		appStderr: runOpt.AppStderr,

		RTASourcePT:         runOpt.RTASourcePT,
		reportOnMainPidExit: runOpt.ReportOnMainPidExit,

		includeNew: includeNew,
		origPaths:  origPaths,

		signalCh: signalCh,
		errorCh:  errorCh,

		StateCh:  make(chan AppState, 5),
		ReportCh: make(chan *report.PtMonitorReport),

		Report: report.PtMonitorReport{
			ArchName:     string(archName),
			SyscallStats: map[string]report.SyscallStatInfo{},
			FSActivity:   map[string]*report.FSActivityInfo{},
		},

		fsActivity:      map[string]*report.FSActivityInfo{},
		syscallActivity: map[uint32]uint64{},

		eventCh:         make(chan syscallEvent, eventBufSize),
		collectorDoneCh: make(chan int, 2),

		logger: logger,
	}

	return &a, nil
}

func (app *App) trace() {
	logger := app.logger.WithField("op", "trace")
	logger.Debug("call")
	defer logger.Debug("exit")

	runtime.LockOSThread()

	if err := app.start(); err != nil {
		app.collectorDoneCh <- 1
		app.errorCh <- errors.SE("ptrace.App.trace.app.start", "call.error", err)
		app.StateCh <- AppFailed
		return
	}

	cancelSignalForwarding := app.startSignalForwarding()
	defer cancelSignalForwarding()

	app.StateCh <- AppStarted
	app.collect()
}

func (app *App) processSyscallActivity(e *syscallEvent) {
	app.syscallActivity[e.callNum]++
}

func (app *App) processFileActivity(e *syscallEvent) {
	if e.pathParam != "" {
		logger := app.logger.WithField("op", "processFileActivity")
		p, found := syscallProcessors[int(e.callNum)]
		if !found {
			logger.Debugf("no syscall processor - %#v", e)
			//shouldn't happen
			return
		}

		if (p.SyscallType() == CheckFileType ||
			p.SyscallType() == OpenFileType) &&
			p.OKReturnStatus(e.retVal) {
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

				if app.del != nil {
					//NOTE:
					//not capturing the 'dirfd' syscall params necessary
					//to reconstruct relative paths for some syscalls (todo: improve later)
					delEvent := &report.MonitorDataEvent{
						Source:   report.MDESourcePT,
						Type:     report.MDETypeArtifact,
						Pid:      int32(e.pid),
						Artifact: e.pathParam, //note: might not be full path
						OpNum:    e.callNum,
						Op:       p.SyscallName(),
					}

					switch p.SyscallType() {
					case CheckFileType:
						delEvent.OpType = report.OpTypeCheck
					case OpenFileType:
						delEvent.OpType = report.OpTypeRead
					}

					if err := app.del.Publish(delEvent); err != nil {
						logger.Errorf(
							"mondel publish event failed - source=%v type=%v: %v",
							delEvent.Source, delEvent.Type, err,
						)
					}
				}
			}
		}

		if p.SyscallType() == ExecType &&
			(!e.returned || //need to catch exec calls that haven't completed yet
				(e.returned && p.OKReturnStatus(e.retVal))) {
			if fsa, ok := app.fsActivity[e.pathParam]; ok {
				fsa.OpsAll++
				fsa.Pids[e.pid] = struct{}{}
				fsa.Syscalls[int(e.callNum)] = struct{}{}
			} else {
				fsa := &report.FSActivityInfo{
					OpsAll:       1,
					OpsCheckFile: 0,
					Pids:         map[int]struct{}{},
					Syscalls:     map[int]struct{}{},
				}

				fsa.Pids[e.pid] = struct{}{}
				fsa.Syscalls[int(e.callNum)] = struct{}{}

				app.fsActivity[e.pathParam] = fsa
			}

			if app.del != nil {
				//NOTE:
				//not capturing the 'dirfd' syscall params necessary
				//to reconstruct relative paths for some syscalls (todo: improve later)
				delEvent := &report.MonitorDataEvent{
					Source:   report.MDESourcePT,
					Type:     report.MDETypeArtifact,
					Pid:      int32(e.pid),
					Artifact: e.pathParam, //note: might not be full path
					OpNum:    e.callNum,
					Op:       p.SyscallName(),
					OpType:   report.OpTypeExec,
				}

				if err := app.del.Publish(delEvent); err != nil {
					logger.Errorf(
						"mondel publish event failed - source=%v type=%v op_type=%v: %v",
						delEvent.Source, delEvent.Type, delEvent.OpType, err,
					)
				}
			}
		}
	}
}

func (app *App) process() {
	logger := app.logger.WithField("op", "process")
	logger.Debug("call")
	defer logger.Debug("exit")

	state := AppDone

done:
	for {
		select {
		// Highest priority - monitor's context has been cancelled from the outside.
		case <-app.ctx.Done():
			logger.Debug("done - stopping...")
			//NOTE: need a better way to stop the target app...
			//"os: process already finished" error is ok
			if err := app.cmd.Process.Signal(syscall.SIGTERM); err != nil {
				logger.Debug("error stopping target app => ", err)
				if err := app.cmd.Process.Kill(); err != nil {
					logger.Debug("error killing target app => ", err)
				}
			}
			break done

		case e := <-app.eventCh:
			app.Report.SyscallCount++
			logger.Tracef("event ==> {pid=%v cn=%d}", e.pid, e.callNum)

			app.processSyscallActivity(&e)
			app.processFileActivity(&e)

		case rc := <-app.collectorDoneCh:
			logger.Debugf("collector finished => %v", rc)
			if rc > 0 {
				state = AppFailed
			}
			break done
		}
	}

	// Drainign the remaining events after the collection is done.
	// Note that it likely introduces a race when ReportOnMainPidExit is true.
	// Events might be generated by the child processes (indefinitely) long after
	// the main process' exit, and this could make the sensor hang. However, the
	// alternative race (when events aren't drained) is even worse - it leads to
	// loosing files in the report.
drain:
	for {
		select {
		case e := <-app.eventCh:
			app.Report.SyscallCount++
			logger.Tracef("event (drained) ==> {pid=%v cn=%d}", e.pid, e.callNum)

			app.processSyscallActivity(&e)
			app.processFileActivity(&e)

		default:
			logger.Trace("event draining is finished")
			break drain
		}
	}

	logger.Tracef("executed syscall count = %d", app.Report.SyscallCount)
	logger.Tracef("number of syscalls: %v", len(app.syscallActivity))

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
	logger := app.logger.WithField("op", "FileActivity")
	logger.Debugf("call - [all records - %d]", len(app.fsActivity))

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

	logger.Debugf("exit - [file records - %d]", len(result))
	return result
}

func (app *App) start() error {
	logger := app.logger.WithField("op", "start")
	logger.Debug("call")
	defer logger.Debug("exit")

	var err error
	app.cmd, err = launcher.Start(
		app.Cmd,
		app.Args,
		app.WorkDir,
		app.User,
		app.RunAsUser,
		app.RTASourcePT,
		app.appStdout,
		app.appStderr,
	)
	if err != nil {
		logger.WithError(err).Errorf(
			"cmd='%v' args='%+v' dir='%v'",
			app.Cmd, app.Args, app.WorkDir,
		)
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
	logger.Debugf("app.cmd.Wait err - %v", err)
	logger.Debugf("Target process state info - Exited=%v ExitCode=%v SysWaitStatus=%v",
		app.cmd.ProcessState.Exited(),
		app.cmd.ProcessState.ExitCode(),
		app.cmd.ProcessState.Sys())

	waitStatus, ok := app.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ok {
		logger.Debugf("Target process wait status - %v (Exited=%v Signaled=%v Signal='%v' Stopped=%v StopSignal='%v' TrapCause=%v)",
			waitStatus,
			waitStatus.Exited(),
			waitStatus.Signaled(),
			waitStatus.Signal(),
			waitStatus.Stopped(),
			waitStatus.StopSignal(),
			waitStatus.TrapCause())

		if waitStatus.Exited() {
			logger.Debug("unexpected app exit")
			return fmt.Errorf("unexpected app exit")
		}

		if waitStatus.Signaled() {
			logger.Debug("unexpected app signalled")
			return fmt.Errorf("unexpected app signalled")
		}

		//we should be in the Stopped state
		if waitStatus.Stopped() {
			sigEnum := SignalEnum(int(waitStatus.StopSignal()))
			logger.Debugf("Process Stop Signal - code=%d enum=%s str=%s",
				waitStatus.StopSignal(), sigEnum, waitStatus.StopSignal())
		} else {
			//TODO:
			//check for Exited or Signaled process state (shouldn't happen)
			//do it for context indicating that we are in a failed state
		}
	} else {
		logger.WithError(err).Error("process status error")
		return fmt.Errorf("process status error")
	}

	app.pgid, err = syscall.Getpgid(app.cmd.Process.Pid)
	if err != nil {
		return err
	}

	logger.Debugf("started target app --> PID=%d PGID=%d",
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
	logger := app.logger.WithField("op", "collect")
	logger.Debug("call")
	defer logger.Debug("exit")

	callPid := app.MainPID()
	prevPid := callPid

	logger.Debugf("trace syscall mainPID=%v", callPid)

	pidSyscallState := map[int]*syscallState{}
	pidSyscallState[callPid] = &syscallState{pid: callPid}

	mainExiting := false
	waitFor := -1
	doSyscall := true
	callSig := 0
	for {
		select {
		case <-app.ctx.Done():
			logger.Debug("done - stop (exiting)")
			return
		default:
		}

		if doSyscall {
			logger.Tracef("trace syscall (pid=%v sig=%v)", callPid, callSig)
			err := syscall.PtraceSyscall(callPid, callSig)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "no such process") {
					// This is kinda-sorta normal situtaion when ptrace-ing:
					//   - The tracee process might have been KILL-led
					//   - A group-stop event in a multi-threaded program can have
					//     a similar effect (see also https://linux.die.net/man/2/ptrace).
					//
					// We'd been observing this behavior a lot with short&fast Go programs,
					// and regular `strace -f ./app` would produce similar results.
					// Sending this error event back to the master process would make
					// the `slim build` command fail with the exit code -124.
					logger.Debugf("trace syscall - (likely) tracee terminated pid=%v sig=%v error - %v (errno=%d)", callPid, callSig, err, err.(syscall.Errno))
				} else {
					logger.Errorf("trace syscall pid=%v sig=%v error - %v (errno=%d)", callPid, callSig, err, err.(syscall.Errno))
					app.errorCh <- errors.SE("ptrace.App.collect.ptsyscall", "call.error", err)
				}
				//keep waiting for other syscalls
			}
		}

		logger.Trace("waiting for syscall...")
		var ws syscall.WaitStatus
		wpid, err := syscall.Wait4(waitFor, &ws, syscall.WALL, nil)
		if err != nil {
			if err.(syscall.Errno) == syscall.ECHILD {
				logger.Debug("wait4 ECHILD error (ignoring)")
				doSyscall = false
				continue
			}

			logger.Debugf("wait4 error - %v (errno=%d)", err, err.(syscall.Errno))
			app.errorCh <- errors.SE("ptrace.App.collect.wait4", "call.error", err)
			app.StateCh <- AppFailed
			app.collectorDoneCh <- 2
			return
		}

		logger.Tracef("wait4 -> wpid=%v wstatus=%v (Exited=%v Signaled=%v Signal='%v' Stopped=%v StopSignalInfo=%s TrapCause=%s)",
			wpid,
			ws,
			ws.Exited(),
			ws.Signaled(),
			ws.Signal(),
			ws.Stopped(),
			StopSignalInfo(ws.StopSignal()),
			SigTrapCauseInfo(ws.TrapCause()))

		if wpid == -1 {
			logger.Error("wpid = -1")
			app.StateCh <- AppFailed
			app.errorCh <- errors.SE("ptrace.App.collect.wpid", "call.error", fmt.Errorf("wpid is -1"))
			// TODO(ivan): Investigate if this code branch leads to sensor becoming stuck.
			//             Should we collectorDoneCh <- 42?
			return
		}

		callSig = 0 // reset
		terminated := false
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
				logger.Debugf("[%d/%d]: unknown process is terminated (pid=%v)",
					app.cmd.Process.Pid, app.pgid, wpid)
			} else {
				if !pidSyscallState[wpid].exiting {
					logger.Debugf("[%d/%d]: unexpected process termination (pid=%v)",
						app.cmd.Process.Pid, app.pgid, wpid)
				}
			}

			delete(pidSyscallState, wpid)
			if app.MainPID() == wpid {
				logger.Debugf("[%d/%d]: wpid(%v) is main PID and terminated...",
					app.cmd.Process.Pid, app.pgid, wpid)
				if !mainExiting {
					logger.Debug("unexpected main PID termination...")
				}

				if len(pidSyscallState) > 0 && app.reportOnMainPidExit {
					// Announce the end of event collection but don't stop the tracing loop.
					//
					// This makes the monitor report the tracing results unblocking the sensor
					// and allowing it to start dumping the artifacts (while giving an extra
					// chance for the child processes to terminate gracefully).
					app.collectorDoneCh <- 0
				}
			}

			if len(pidSyscallState) == 0 {
				logger.Debugf("[%d/%d]: all processes terminated...", app.cmd.Process.Pid, app.pgid)
				app.collectorDoneCh <- 0 // TODO(ivan): Should it be collectorDoneCh <- statusCode instead?
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
				logger.Debugf("[%d/%d]: collector loop - new pid - mainPid=%v pid=%v (prevPid=%v) - add state",
					app.cmd.Process.Pid, app.pgid, app.MainPID(), wpid, prevPid)
				//TODO: create new process records from clones/forks
				cstate = &syscallState{pid: wpid}
				pidSyscallState[wpid] = cstate
			}

			if !cstate.expectReturn {
				genEvent, err := onSyscall(wpid, cstate)
				if err != nil {
					logger.Debugf("[%d/%d]: wpid=%v onSyscall error - %v",
						app.cmd.Process.Pid, app.pgid, wpid, err)
					continue
				}

				if genEvent {
					//not waiting for the return call for the 'exec' syscalls (mostly)
					evt := syscallEvent{
						returned:  false,
						pid:       wpid,
						callNum:   uint32(cstate.callNum),
						pathParam: cstate.pathParam,
					}

					logger.Debugf("[%d/%d]: event.onSyscall - wpid=%v (evt=%#v)",
						app.cmd.Process.Pid, app.pgid, wpid, evt)
					select {
					case app.eventCh <- evt:
					default:
						logger.Warnf("[%d/%d]: event.onSyscall - wpid=%v app.eventCh send error (evt=%#v)",
							app.cmd.Process.Pid, app.pgid, wpid, evt)
					}
				}
			} else {
				if err := onSyscallReturn(wpid, cstate); err != nil {
					logger.Debugf("[%d/%d]: wpid=%v onSyscallReturn error - %v",
						app.cmd.Process.Pid, app.pgid, wpid, err)
					continue
				}
			}

			if cstate.gotCallNum && cstate.gotRetVal {
				evt := syscallEvent{
					returned:  true,
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
						logger.Warnf("[%d/%d]: wpid=%v app.eventCh send error (evt=%#v)",
							app.cmd.Process.Pid, app.pgid, wpid, evt)
					}
				}
			}
		}

		if eventStop {
			logger.Debugf("[%d/%d]: eventStop eventCode=%d(0x%04x)",
				app.cmd.Process.Pid, app.pgid, eventCode, eventCode)

			switch eventCode {
			case syscall.PTRACE_EVENT_CLONE,
				syscall.PTRACE_EVENT_FORK,
				syscall.PTRACE_EVENT_VFORK,
				syscall.PTRACE_EVENT_VFORK_DONE:
				newPid, err := syscall.PtraceGetEventMsg(wpid)
				if err != nil {
					logger.Debugf("[%d/%d]: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - error getting cloned pid - %v",
						app.cmd.Process.Pid, app.pgid, err)
				} else {
					logger.Debugf("[%d/%d]: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - cloned pid - %v",
						app.cmd.Process.Pid, app.pgid, newPid)
					if _, ok := pidSyscallState[int(newPid)]; ok {
						logger.Debugf("[%d/%d]: PTRACE_EVENT_CLONE/[V]FORK[_DONE] - pid already exists - %v",
							app.cmd.Process.Pid, app.pgid, newPid)
						pidSyscallState[int(newPid)].started = true
					} else {
						pidSyscallState[int(newPid)] = &syscallState{pid: int(newPid), started: true}
					}
				}

			case syscall.PTRACE_EVENT_EXEC:
				oldPid, err := syscall.PtraceGetEventMsg(wpid)
				if err != nil {
					logger.Debugf("[%d/%d]: PTRACE_EVENT_EXEC - error getting old pid - %v",
						app.cmd.Process.Pid, app.pgid, err)
				} else {
					logger.Debugf("[%d/%d]: PTRACE_EVENT_EXEC - old pid - %v",
						app.cmd.Process.Pid, app.pgid, oldPid)
				}

			case syscall.PTRACE_EVENT_EXIT:
				logger.Debugf("[%d/%d]: PTRACE_EVENT_EXIT - process exiting pid=%v",
					app.cmd.Process.Pid, app.pgid, wpid)
				if app.MainPID() == wpid {
					mainExiting = true
					logger.Debugf("[%d/%d]: main process is exiting (%v)",
						app.cmd.Process.Pid, app.pgid, wpid)
				}

				if _, ok := pidSyscallState[wpid]; ok {
					pidSyscallState[wpid].exiting = true
				} else {
					logger.Debugf("[%d/%d]: unknown process is exiting (pid=%v)",
						app.cmd.Process.Pid, app.pgid, wpid)
				}
			}
		}

		doSyscall = true
		callPid = wpid
	} // eof: main for loop
}

func (app *App) startSignalForwarding() context.CancelFunc {
	logger := app.logger.WithField("op", "startSignalForwarding")
	logger.Debug("call")
	defer logger.Debug("exit")

	ctx, cancel := context.WithCancel(app.ctx)

	go func() {
		logger := app.logger.WithField("op", "startSignalForwarding.worker")
		logger.Debug("call")
		defer logger.Debug("exit")

		for {
			select {
			case <-ctx.Done():
				return

			case s := <-app.signalCh:
				if s == syscall.SIGCHLD {
					continue
				}

				logger.WithField("signal", s).Debug("forwarding signal")

				ss, ok := s.(syscall.Signal)
				if !ok {
					logger.WithField("signal", s).Debug("unsupported signal type")
					continue
				}

				// app.cmd.Process.Signal(s) can't be used because due to ptrace-ing
				// the target processes' status becomes set before the actual
				// process termination making Signal() think the process exited and
				// not even trying to deliver the signal.
				if err := syscall.Kill(app.cmd.Process.Pid, ss); err != nil {
					logger.
						WithError(err).
						WithField("signal", ss).
						Debug("failed to signal target app")
				}
			}
		}
	}()

	return cancel
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
		doGenEvent := processor.EventOnCall()
		return doGenEvent, nil
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

func fdName(fd int) string {
	switch fd {
	case 0:
		return "STDIN"
	case 1:
		return "STDOUT"
	case 2:
		return "STDERR"
	case cwdFD:
		return "AT_FDCWD"
	}

	return ""
}

func getIntVal(ptr uint64) int {
	return int(int32(ptr))
}

func errnoName(ptr uint64) string {
	val := int(int32(ptr))
	if val >= 0 {
		return ""
	}

	val *= -1
	return unix.ErrnoName(syscall.Errno(val))
}

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
	OKCall(cstate *syscallState) bool
	OKReturnStatus(retVal uint64) bool
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
	var fd int

	switch ref.StringParam {
	case SPPNo:
		log.Tracef("syscallProcessorCore.OnCall[%s/%d]: pid=%d - no string param", ref.Name, ref.Num, pid)
		return
	case SPPOne:
		pth = getStringParam(pid, system.CallFirstParam(regs))
	case SPPTwo:
		fd = getIntParam(pid, system.CallFirstParam(regs))
		//cwdFD < 0 / (stdin=0/stdout=1/stderr=2)
		if fd > 2 {
			dir, _ = os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", pid, fd))
		}
		pth = getStringParam(pid, system.CallSecondParam(regs))
	default:
		panic("unreachable")
	}

	//fmt.Printf("[pid=%d][1]ptrace.SPC.OnCall[%v/%v](fd=[%x/%d/%s]/pth:'%s')\n",
	//	pid, ref.Num, ref.Name, fd, fd, fdName(fd), pth)

	if len(pth) == 0 || (len(pth) > 0 && pth[0] != '/') {
		if dir != "" {
			pth = path.Join(dir, pth)
		} else if fd == cwdFD {
			//todo: track cwd with process/pid (so we don't need to look it up later for each call)
			cwd, _ := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
			if cwd != "" {
				pth = path.Join(cwd, pth)
			}
		}
	}

	cstate.pathParam = filepath.Clean(pth)
	//fmt.Printf("[pid=%d][2]ptrace.SPC.OnCall[%v/%v](fd=[%x/%d/%s]/pp:'%s')\n",
	//	pid, ref.Num, ref.Name, fd, fd, fdName(fd), cstate.pathParam)
}

func (ref *syscallProcessorCore) OnReturn(pid int, regs syscall.PtraceRegs, cstate *syscallState) {
	//for stat/check syscalls:
	//-2 => -ENOENT (No such file or directory)
	intRetVal := getIntVal(cstate.retVal)
	errnoStr := errnoName(cstate.retVal)

	if cstate.pathParamErr == nil {
		//fmt.Printf("[pid=%d][3]ptrace.SPC.OnReturn[%v/%v](pp:'%s') = %x/%d/%s\n",
		//	pid, cstate.callNum, ref.Name, cstate.pathParam, cstate.retVal, intRetVal, errnoStr)

		log.Tracef("checkFileSyscallProcessor.OnReturn: [%d] {%d}%s('%s') = %x/%d/%s", pid, cstate.callNum, ref.Name, cstate.pathParam, cstate.retVal, intRetVal, errnoStr)
	} else {
		log.Debugf("checkFileSyscallProcessor.OnReturn: [%d] {%d}%s(<unknown>/'%s') = %x/%d/%s [pp err => %v]", pid, cstate.callNum, ref.Name, cstate.pathParam, cstate.retVal, intRetVal, errnoStr, cstate.pathParamErr)

		//fmt.Printf("[pid=%d][3err]ptrace.SPC.OnReturn[%v/%v](pp:'%s'/ppe: '%v') = %x/%d/%s\n",
		//	pid, cstate.callNum, ref.Name, cstate.pathParam, cstate.pathParamErr, cstate.retVal, intRetVal, errnoStr)
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

func (ref *checkFileSyscallProcessor) OKCall(cstate *syscallState) bool {
	return cstate.retVal == 0
}

func (ref *checkFileSyscallProcessor) OKReturnStatus(retVal uint64) bool {
	return retVal == 0
}

func (ref *checkFileSyscallProcessor) EventOnCall() bool {
	return false
}

type openFileSyscallProcessor struct {
	*syscallProcessorCore
}

func (ref *openFileSyscallProcessor) FailedCall(cstate *syscallState) bool {
	fd := getIntVal(cstate.retVal)
	return fd < 0
}

func (ref *openFileSyscallProcessor) FailedReturnStatus(retVal uint64) bool {
	fd := getIntVal(retVal)
	return fd < 0
}

func (ref *openFileSyscallProcessor) OKCall(cstate *syscallState) bool {
	fd := getIntVal(cstate.retVal)
	//need to make sure it works for readlink which returns the number of read bytes
	//(later: has its own processor)
	return fd >= 0
}

func (ref *openFileSyscallProcessor) OKReturnStatus(retVal uint64) bool {
	fd := getIntVal(retVal)
	//need to make sure it works for readlink which returns the number of read bytes
	//(later: has its own processor)
	return fd >= 0
}

func (ref *openFileSyscallProcessor) EventOnCall() bool {
	return false
}

type execSyscallProcessor struct {
	*syscallProcessorCore
}

func (ref *execSyscallProcessor) FailedCall(cstate *syscallState) bool {
	fd := getIntVal(cstate.retVal)
	return fd < 0
}

func (ref *execSyscallProcessor) FailedReturnStatus(retVal uint64) bool {
	fd := getIntVal(retVal)
	return fd < 0
}

func (ref *execSyscallProcessor) OKCall(cstate *syscallState) bool {
	fd := getIntVal(cstate.retVal)
	return fd >= 0
}

func (ref *execSyscallProcessor) OKReturnStatus(retVal uint64) bool {
	fd := getIntVal(retVal)
	return fd >= 0
}

func (ref *execSyscallProcessor) EventOnCall() bool {
	return true
}

// TODO: introduce syscall num and name consts to use instead of liternal values
var syscallProcessors = map[int]SyscallProcessor{}

func init() {
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

	//"check" system calls with file paths:

	//access(const char *filename, int mode)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "access",
			Type:        CheckFileType,
			StringParam: SPPOne,
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
	//statfs(const char *pathname, struct statfs *buf)
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "statfs",
			Type:        CheckFileType,
			StringParam: SPPOne,
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
	//newfstatat/fstatat(int dirfd, const char *restrict pathname, struct stat *restrict statbuf, int flags)
	//dirfd: AT_FDCWD
	addSyscallProcessor(&checkFileSyscallProcessor{
		syscallProcessorCore: &syscallProcessorCore{
			Name:        "newfstatat",
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
