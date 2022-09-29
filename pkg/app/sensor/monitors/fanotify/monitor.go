//go:build linux
// +build linux

package fanotify

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/report"
	fanapi "github.com/docker-slim/docker-slim/pkg/third_party/madmo/fanotify"
)

const (
	errorBufSize   = 10
	eventBufSize   = 1000
	procFsFdInfo   = "/proc/self/fd/%d"
	procFsFilePath = "/proc/%v/%v"
)

// Event is file operation event
type Event struct {
	ID      uint32
	Pid     int32
	File    string
	IsRead  bool
	IsWrite bool
}

type Monitor interface {
	// Starts the long running monitoring. The method itself is not
	// blocking and not reentrant!
	Start() error

	// Cancels the underlying ptrace execution context but doesn't
	// make the current monitor done immediately. You still need to await
	// the final cleanup with <-mon.Done() before accessing the status.
	Cancel()

	// With Done clients can await for the monitoring completion.
	// The method is reentrant - every invocation returns the same
	// instance of the channel.
	Done() <-chan struct{}

	Status() (*report.FanMonitorReport, error)
}

type status struct {
	report *report.FanMonitorReport
	err    error
}

type monitor struct {
	ctx    context.Context
	cancel context.CancelFunc

	mountPoint string

	// TODO: Move the logic behind these two fields to the artifact processig stage.
	includeNew bool
	origPaths  map[string]struct{}

	status status
	doneCh chan struct{}
}

func NewMonitor(
	ctx context.Context,
	mountPoint string,
	includeNew bool,
	origPaths map[string]struct{},
) Monitor {
	ctx, cancel := context.WithCancel(ctx)
	return &monitor{
		ctx:    ctx,
		cancel: cancel,

		mountPoint: mountPoint,
		includeNew: includeNew,
		origPaths:  origPaths,

		doneCh: make(chan struct{}),
	}
}

func (m *monitor) Start() error {
	log.Info("fanmon: Start")

	nd, err := fanapi.Initialize(fanapi.FAN_CLASS_NOTIF, os.O_RDONLY)
	if err != nil {
		return errors.SE("sensor.fanotify.Run/fanapi.Initialize", "call.error", err)
	}

	if err = nd.Mark(
		fanapi.FAN_MARK_ADD|fanapi.FAN_MARK_MOUNT,
		fanapi.FAN_MODIFY|fanapi.FAN_ACCESS|fanapi.FAN_OPEN,
		-1, m.mountPoint,
	); err != nil {
		return errors.SE("sensor.fanotify.Run/nd.Mark", "call.error", err)
	}

	// Sync part of the start was successful.

	// Tracking the completetion of the monitor....
	go func() {
		log.Debug("fanmon: processor - starting...")

		fanReport := &report.FanMonitorReport{
			MonitorPid:       os.Getpid(),
			MonitorParentPid: os.Getppid(),
			ProcessFiles:     map[string]map[string]*report.FileInfo{},
			Processes:        map[string]*report.ProcessInfo{},
		}

		eventChan := make(chan Event, eventBufSize)
		go func() {
			log.Debug("fanmon: collector - starting...")
			var eventID uint32

			for {
				//TODO: enhance FA Notify to return the original file handle too
				data, err := nd.GetEvent()
				if err != nil {
					m.status.err = errors.SE("sensor.fanotify.Run/nd.GetEvent", "call.error", err)
					close(m.doneCh)
					return
				}
				log.Debugf("fanmon: collector - data.Mask => %x", data.Mask)

				if (data.Mask & fanapi.FAN_Q_OVERFLOW) == fanapi.FAN_Q_OVERFLOW {
					log.Debug("fanmon: collector - overflow event")
					continue
				}

				doNotify := false
				isRead := false
				isWrite := false

				if (data.Mask & fanapi.FAN_OPEN) == fanapi.FAN_OPEN {
					log.Debug("fanmon: collector - file open")
					doNotify = true
				}

				if (data.Mask & fanapi.FAN_ACCESS) == fanapi.FAN_ACCESS {
					log.Debug("fanmon: collector - file read")
					isRead = true
					doNotify = true
				}

				if (data.Mask & fanapi.FAN_MODIFY) == fanapi.FAN_MODIFY {
					log.Debug("fanmon: collector - file write")
					isWrite = true
					doNotify = true
				}

				path, err := os.Readlink(fmt.Sprintf(procFsFdInfo, data.File.Fd()))
				if err != nil {
					m.status.err = errors.SE("sensor.fanotify.Run/os.Readlink", "call.error", err)
					close(m.doneCh)
					return
				}

				log.Debugf("fanmon: collector - file path => %v", path)

				data.File.Close()
				if doNotify {
					eventID++
					e := Event{ID: eventID, Pid: data.Pid, File: path, IsRead: isRead, IsWrite: isWrite}

					select {
					case eventChan <- e:
					case <-m.ctx.Done():
						log.Info("fanmon: collector - stopping....")
						return
					}
				}
			}
		}()

	done:
		for {
			select {
			case <-m.ctx.Done():
				log.Info("fanmon: processor - stopping...")
				break done
			case e := <-eventChan:
				fanReport.EventCount++
				log.Debugf("fanmon: processor - [%v] handling event %v", fanReport.EventCount, e)

				_, ok := m.origPaths[e.File]
				if m.includeNew {
					ok = true
				}

				if !ok {
					continue done
				}

				if e.ID == 1 {
					//first event represents the main process
					if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
						fanReport.MainProcess = pinfo
						fanReport.Processes[strconv.Itoa(int(e.Pid))] = pinfo
					}
				} else {
					if _, ok := fanReport.Processes[strconv.Itoa(int(e.Pid))]; !ok {
						if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
							fanReport.Processes[strconv.Itoa(int(e.Pid))] = pinfo
						}
					}
				}

				if _, ok := fanReport.ProcessFiles[strconv.Itoa(int(e.Pid))]; !ok {
					fanReport.ProcessFiles[strconv.Itoa(int(e.Pid))] = map[string]*report.FileInfo{}
				}

				if existingFi, ok := fanReport.ProcessFiles[strconv.Itoa(int(e.Pid))][e.File]; !ok {
					fi := &report.FileInfo{
						EventCount:   1,
						Name:         e.File,
						FirstEventID: e.ID,
					}

					if e.IsRead {
						fi.ReadCount = 1
					}

					if e.IsWrite {
						fi.WriteCount = 1
					}

					if pi, ok := fanReport.Processes[strconv.Itoa(int(e.Pid))]; ok && (e.File == pi.Path) {
						fi.ExeCount = 1
					}

					fanReport.ProcessFiles[strconv.Itoa(int(e.Pid))][e.File] = fi
				} else {
					existingFi.EventCount++

					if e.IsRead {
						existingFi.ReadCount++
					}

					if e.IsWrite {
						existingFi.WriteCount++
					}

					if pi, ok := fanReport.Processes[strconv.Itoa(int(e.Pid))]; ok && (e.File == pi.Path) {
						existingFi.ExeCount++
					}
				}
			}
		}

		log.Debugf("fanmon: processor - sending report (processed %v events)...", fanReport.EventCount)
		m.status.report = fanReport
		close(m.doneCh)
	}()

	return nil
}

func (m *monitor) Cancel() {
	m.cancel()
}

func (m *monitor) Done() <-chan struct{} {
	return m.doneCh
}

func (m *monitor) Status() (*report.FanMonitorReport, error) {
	return m.status.report, m.status.err
}

func procFilePath(pid int, key string) string {
	return fmt.Sprintf(procFsFilePath, pid, key)
}

func getProcessInfo(pid int32) (*report.ProcessInfo, error) {
	info := &report.ProcessInfo{Pid: pid}
	var err error

	info.Path, err = os.Readlink(procFilePath(int(pid), "exe"))
	if err != nil {
		return nil, err
	}

	info.Cwd, err = os.Readlink(procFilePath(int(pid), "cwd"))
	if err != nil {
		return nil, err
	}

	info.Root, err = os.Readlink(procFilePath(int(pid), "root"))
	if err != nil {
		return nil, err
	}

	rawCmdline, err := ioutil.ReadFile(procFilePath(int(pid), "cmdline"))
	if err != nil {
		return nil, err
	}

	if len(rawCmdline) > 0 {
		rawCmdline = bytes.TrimRight(rawCmdline, "\x00")
		//NOTE: later/future (when we do more app analytics)
		//split rawCmdline and resolve the "entry point" (exe or cmd param)
		info.Cmd = string(bytes.Replace(rawCmdline, []byte("\x00"), []byte(" "), -1))
	}

	//note: will need to get "environ" at some point :)
	//rawEnviron, err := ioutil.ReadFile(procFilePath(int(pid), "environ"))
	//if err != nil {
	//	return nil, err
	//}
	//if len(rawEnviron) > 0 {
	//	rawEnviron = bytes.TrimRight(rawEnviron,"\x00")
	//	info.Env = strings.Split(string(rawEnviron),"\x00")
	//}

	info.Name = "unknown"
	info.ParentPid = -1

	stat, err := ioutil.ReadFile(procFilePath(int(pid), "stat"))
	if err == nil {
		var procPid int
		var procName string
		var procStatus string
		var procPpid int
		fmt.Sscanf(string(stat), "%d %s %s %d", &procPid, &procName, &procStatus, &procPpid)

		info.Name = procName[1 : len(procName)-1]
		info.ParentPid = int32(procPpid)
	}

	return info, nil
}
