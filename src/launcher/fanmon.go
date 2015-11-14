package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"internal/utils"

	"bitbucket.org/madmo/fanotify"
	log "github.com/Sirupsen/logrus"
)

type fanMonitorReport struct {
	MonitorPid       int                             `json:"monitor_pid"`
	MonitorParentPid int                             `json:"monitor_ppid"`
	EventCount       uint32                          `json:"event_count"`
	MainProcess      *processInfo                    `json:"main_process"`
	Processes        map[string]*processInfo         `json:"processes"`
	ProcessFiles     map[string]map[string]*fileInfo `json:"process_files"`
}

type event struct {
	ID      uint32
	Pid     int32
	File    string
	IsRead  bool
	IsWrite bool
}

func fanRunMonitor(mountPoint string, stopChan chan struct{}) <-chan *fanMonitorReport {
	log.Info("fanmon: starting...")

	nd, err := fanotify.Initialize(fanotify.FAN_CLASS_NOTIF, os.O_RDONLY)
	utils.FailOn(err)
	err = nd.Mark(fanotify.FAN_MARK_ADD|fanotify.FAN_MARK_MOUNT,
		fanotify.FAN_MODIFY|fanotify.FAN_ACCESS|fanotify.FAN_OPEN, -1, mountPoint)
	utils.FailOn(err)

	eventsChan := make(chan *fanMonitorReport, 1)

	go func() {
		log.Debug("fanmon: fanRunMonitor worker starting")

		report := &fanMonitorReport{
			MonitorPid:       os.Getpid(),
			MonitorParentPid: os.Getppid(),
			ProcessFiles:     make(map[string]map[string]*fileInfo),
		}

		eventChan := make(chan event)
		go func() {
			log.Debug("fanmon: fanRunMonitor worker (monitor) starting")
			var eventID uint32

			for {
				data, err := nd.GetEvent()
				utils.FailOn(err)
				log.Debugf("fanmon: data.Mask =>%x\n", data.Mask)

				if (data.Mask & fanotify.FAN_Q_OVERFLOW) == fanotify.FAN_Q_OVERFLOW {
					log.Debug("fanmon: overflow event")
					continue
				}

				doNotify := false
				isRead := false
				isWrite := false

				if (data.Mask & fanotify.FAN_OPEN) == fanotify.FAN_OPEN {
					log.Debug("fanmon: file open")
					doNotify = true
				}

				if (data.Mask & fanotify.FAN_ACCESS) == fanotify.FAN_ACCESS {
					log.Debug("fanmon: file read")
					isRead = true
					doNotify = true
				}

				if (data.Mask & fanotify.FAN_MODIFY) == fanotify.FAN_MODIFY {
					log.Debug("fanmon: file write")
					isWrite = true
					doNotify = true
				}

				path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", data.File.Fd()))
				utils.FailOn(err)
				log.Debug("fanmon: file path =>", path)

				data.File.Close()
				if doNotify {
					eventID++
					e := event{ID: eventID, Pid: data.Pid, File: path, IsRead: isRead, IsWrite: isWrite}
					eventChan <- e
				}
			}
		}()

	done:
		for {
			select {
			case <-stopChan:
				log.Info("fanmon: stopping...")
				break done
			case e := <-eventChan:
				report.EventCount++
				log.Debug("fanmon: event ", report.EventCount)

				if e.ID == 1 {
					//first event represents the main process
					if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
						report.MainProcess = pinfo
						report.Processes = make(map[string]*processInfo)
						report.Processes[strconv.Itoa(int(e.Pid))] = pinfo
					}
				} else {
					if _, ok := report.Processes[strconv.Itoa(int(e.Pid))]; !ok {
						if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
							report.Processes[strconv.Itoa(int(e.Pid))] = pinfo
						}
					}
				}

				if _, ok := report.ProcessFiles[strconv.Itoa(int(e.Pid))]; !ok {
					report.ProcessFiles[strconv.Itoa(int(e.Pid))] = make(map[string]*fileInfo)
				}

				if existingFi, ok := report.ProcessFiles[strconv.Itoa(int(e.Pid))][e.File]; !ok {
					fi := &fileInfo{
						EventCount: 1,
						Name:       e.File,
					}

					if e.IsRead {
						fi.ReadCount = 1
					}

					if e.IsWrite {
						fi.WriteCount = 1
					}

					if pi, ok := report.Processes[strconv.Itoa(int(e.Pid))]; ok && (e.File == pi.Path) {
						fi.ExeCount = 1
					}

					report.ProcessFiles[strconv.Itoa(int(e.Pid))][e.File] = fi
				} else {
					existingFi.EventCount++

					if e.IsRead {
						existingFi.ReadCount++
					}

					if e.IsWrite {
						existingFi.WriteCount++
					}

					if pi, ok := report.Processes[strconv.Itoa(int(e.Pid))]; ok && (e.File == pi.Path) {
						existingFi.ExeCount++
					}
				}
			}
		}

		log.Debugf("fanmon: sending report (processed %v events)...\n", report.EventCount)
		eventsChan <- report
	}()

	return eventsChan
}

func procFilePath(pid int, key string) string {
	return fmt.Sprintf("/proc/%v/%v", pid, key)
}

func getProcessInfo(pid int32) (*processInfo, error) {
	info := &processInfo{Pid: pid}
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

	stat, err := ioutil.ReadFile(procFilePath(int(pid), "stat"))
	var procPid int
	var procName string
	var procStatus string
	var procPpid int
	fmt.Sscanf(string(stat), "%d %s %s %d", &procPid, &procName, &procStatus, &procPpid)

	info.Name = procName[1 : len(procName)-1]
	info.ParentPid = int32(procPpid)

	return info, nil
}
