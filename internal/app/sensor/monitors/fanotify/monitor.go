package fanotify

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/docker-slim/docker-slim/pkg/errors"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/Sirupsen/logrus"
	fanapi "github.com/docker-slim/docker-slim/pkg/third_party/madmo/fanotify"
)

// Event is file operation event
type Event struct {
	ID      uint32
	Pid     int32
	File    string
	IsRead  bool
	IsWrite bool
}

const (
	eventBufSize   = 1000
	procFsFdInfo   = "/proc/self/fd/%d"
	procFsFilePath = "/proc/%v/%v"
)

// Run starts the FANOTIFY monitor
func Run(errorCh chan error, mountPoint string, stopChan chan struct{}) <-chan *report.FanMonitorReport {
	log.Info("fanmon: Run")

	nd, err := fanapi.Initialize(fanapi.FAN_CLASS_NOTIF, os.O_RDONLY)
	//TODO: need to propagate the FANOTIFY init failure back to the master instead of just crashing the sensor!
	//errutil.FailOn(err)
	if err != nil {
		sensorErr := errors.SE("sensor.fanotify.Run/fanapi.Initialize", "call.error", err)
		errorCh <- sensorErr
		return nil
	}

	err = nd.Mark(fanapi.FAN_MARK_ADD|fanapi.FAN_MARK_MOUNT,
		fanapi.FAN_MODIFY|fanapi.FAN_ACCESS|fanapi.FAN_OPEN, -1, mountPoint)
	//errutil.FailOn(err)
	if err != nil {
		sensorErr := errors.SE("sensor.fanotify.Run/nd.Mark", "call.error", err)
		errorCh <- sensorErr
		return nil
	}

	resultChan := make(chan *report.FanMonitorReport, 1)

	go func() {
		log.Debug("fanmon: processor - starting...")

		fanReport := &report.FanMonitorReport{
			MonitorPid:       os.Getpid(),
			MonitorParentPid: os.Getppid(),
			ProcessFiles:     make(map[string]map[string]*report.FileInfo),
		}

		eventChan := make(chan Event, eventBufSize)
		go func() {
			log.Debug("fanmon: collector - starting...")
			var eventID uint32

			for {
				//TODO: enhance FA Notify to return the original file handle too
				data, err := nd.GetEvent()
				errutil.FailOn(err)
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
				errutil.FailOn(err)

				log.Debugf("fanmon: collector - file path => %v", path)

				data.File.Close()
				if doNotify {
					eventID++
					e := Event{ID: eventID, Pid: data.Pid, File: path, IsRead: isRead, IsWrite: isWrite}

					select {
					case eventChan <- e:
					case <-stopChan:
						log.Info("fanmon: collector - stopping....")
						return
					}
				}
			}
		}()

	done:
		for {
			select {
			case <-stopChan:
				log.Info("fanmon: processor - stopping...")
				break done
			case e := <-eventChan:
				fanReport.EventCount++
				log.Debugf("fanmon: processor - [%v] handling event %v", fanReport.EventCount, e)

				if e.ID == 1 {
					//first event represents the main process
					if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
						fanReport.MainProcess = pinfo
						fanReport.Processes = make(map[string]*report.ProcessInfo)
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
					fanReport.ProcessFiles[strconv.Itoa(int(e.Pid))] = make(map[string]*report.FileInfo)
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
		resultChan <- fanReport
	}()

	return resultChan
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

	info.Name = "uknown"
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
