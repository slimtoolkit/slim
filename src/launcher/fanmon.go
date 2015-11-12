package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"bitbucket.org/madmo/fanotify"
)

type fanMonitorReport struct {
	MonitorPid       int                             `json:"monitor_pid"`
	MonitorParentPid int                             `json:"monitor_ppid"`
	EventCount       uint32                          `json:"event_count"`
	MainProcess      *processInfo                    `json:"main_process"`
	Processes        map[string]*processInfo         `json:"processes"`
	ProcessFiles     map[string]map[string]*fileInfo `json:"process_files"`
}

//func listenEvents(mountPoint string, stopChan chan bool) chan map[event]bool {
func fanRunMonitor(mountPoint string, stopChan chan struct{}) <-chan *fanMonitorReport {
	log.Println("monitor: fanRunMonitor start")

	nd, err := fanotify.Initialize(fanotify.FAN_CLASS_NOTIF, os.O_RDONLY)
	check(err)
	err = nd.Mark(fanotify.FAN_MARK_ADD|fanotify.FAN_MARK_MOUNT,
		fanotify.FAN_MODIFY|fanotify.FAN_ACCESS|fanotify.FAN_OPEN, -1, mountPoint)
	check(err)

	//eventsChan := make(chan map[event]bool, 1)
	eventsChan := make(chan *fanMonitorReport, 1)

	go func() {
		log.Println("monitor: fanRunMonitor worker starting")
		//events := make(map[event]bool, 1)
		report := &fanMonitorReport{
			MonitorPid:       os.Getpid(),
			MonitorParentPid: os.Getppid(),
			ProcessFiles:     make(map[string]map[string]*fileInfo),
		}

		eventChan := make(chan event)
		go func() {
			log.Println("monitor: fanRunMonitor worker (monitor) starting")
			var eventID uint32

			for {
				data, err := nd.GetEvent()
				check(err)
				log.Printf("TMP: monitor: fanRunMonitor: data.Mask =>%x\n",data.Mask)

				if (data.Mask & fanotify.FAN_Q_OVERFLOW) == fanotify.FAN_Q_OVERFLOW {
					log.Println("monitor: fanRunMonitor: overflow event")
					continue
				}

				doNotify := false
				isRead := false
				isWrite := false

				if (data.Mask & fanotify.FAN_OPEN) == fanotify.FAN_OPEN {
					//log.Println("TMP: monitor: fanRunMonitor: file open")
					doNotify = true
				}

				if (data.Mask & fanotify.FAN_ACCESS) == fanotify.FAN_ACCESS {
					//log.Println("TMP: monitor: fanRunMonitor: file read")
					isRead = true
					doNotify = true
				}

				if (data.Mask & fanotify.FAN_MODIFY) == fanotify.FAN_MODIFY {
					//log.Println("TMP: monitor: fanRunMonitor: file write")
					isWrite = true
					doNotify = true
				}

				path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", data.File.Fd()))
				check(err)
				//log.Println("TMP: monitor: fanRunMonitor: file path =>",path)

				data.File.Close()
				if doNotify {
					eventID++
					e := event{ID: eventID, Pid: data.Pid, File: path, IsRead: isRead, IsWrite: isWrite}
					eventChan <- e
				}
			}
		}()

		s := false
		for !s {
			select {
			//case <-time.After(110 * time.Second):
			//	log.Println("monitor: fanRunMonitor - event timeout...")
			//	s = true
			case <-stopChan:
				log.Println("monitor: fanRunMonitor stopping...")
				s = true
			case e := <-eventChan:
				report.EventCount++
				log.Println("TMP: monitor: fanRunMonitor: event ",report.EventCount)

				if e.ID == 1 {
					//first event represents the main process
					if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
						report.MainProcess = pinfo
						report.Processes = make(map[string]*processInfo)
						report.Processes[strconv.Itoa(int(e.Pid))] = pinfo
						//log.Println("TMP: monitor: listenEvents: (1) adding pi for ",
						//	e.Pid,"info:",report.Processes[strconv.Itoa(int(e.Pid))])
					}
				} else {
					if _, ok := report.Processes[strconv.Itoa(int(e.Pid))]; !ok {
						if pinfo, err := getProcessInfo(e.Pid); (err == nil) && (pinfo != nil) {
							report.Processes[strconv.Itoa(int(e.Pid))] = pinfo
							//log.Println("TMP: monitor: fanRunMonitor: (2) adding pi for ",
							//	e.Pid,"info:",report.Processes[strconv.Itoa(int(e.Pid))])
						}
					}
				}

				if _, ok := report.ProcessFiles[strconv.Itoa(int(e.Pid))]; !ok {
					report.ProcessFiles[strconv.Itoa(int(e.Pid))] = make(map[string]*fileInfo)
					//log.Println("TMP: monitor: fanRunMonitor: adding pf for ",e.Pid)
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

				//log.Printf("monitor: listenEvents event => %#v\n", e)
			}
		}

		log.Printf("monitor: listenEvents - sending report (processed %v events)...\n", report.EventCount)
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
