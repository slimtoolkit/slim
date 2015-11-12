package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/pub"
	"github.com/gdamore/mangos/protocol/rep"
	//"github.com/gdamore/mangos/transport/ipc"
	"github.com/cloudimmunity/pdiscover"
	"github.com/gdamore/mangos/transport/tcp"
)

func failOnError(err error) {
	if err != nil {
		log.Fatalln("launcher: ERROR =>", err)
	}
}

func failWhen(cond bool, msg string) {
	if cond {
		log.Fatalln("launcher: ERROR =>", msg)
	}
}

func myFileDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	failOnError(err)
	return dirName
}

func fileDir(fileName string) string {
	dirName, err := filepath.Abs(filepath.Dir(fileName))
	failOnError(err)
	return dirName
}

///////////////////////////////////////////////////////////////////////////////

var doneChan chan struct{}

var cmdChannelAddr = "tcp://0.0.0.0:65501"

//var cmdChannelAddr = "ipc:///tmp/docker-slim-launcher.cmds.ipc"
//var cmdChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-launcher.cmds.ipc"
var cmdChannel mangos.Socket

func newCmdServer(addr string) (mangos.Socket, error) {
	log.Println("launcher: creating cmd server...")
	socket, err := rep.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Listen(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}

func runCmdServer(channel mangos.Socket, done <-chan struct{}) (<-chan string, error) {
	cmdChan := make(chan string)
	go func() {
		for {
			// Could also use sock.RecvMsg to get header
			log.Println("launcher: cmd server - waiting for a command...")
			select {
			case <-done:
				log.Println("launcher: cmd server - done...")
				return
			default:
				if rawCmd, err := channel.Recv(); err != nil {
					switch err {
					case mangos.ErrRecvTimeout:
						log.Println("launcher: cmd server - timeout... ok")
					default:
						log.Println("launcher: cmd server - error =>", err)
					}
				} else {
					cmd := string(rawCmd)
					log.Println("launcher: cmd server - got a command =>", cmd)
					cmdChan <- cmd
					//for now just ack the command and process the command asynchronously
					//NOTE:
					//must reply before receiving the next message
					//otherwise nanomsg/mangos will be confused :-)
					monitorFinishReply := "ok"
					err = channel.Send([]byte(monitorFinishReply))
					if err != nil {
						log.Println("launcher: cmd server - fail to send monitor.finish reply =>", err)
					}
				}
			}
		}
	}()

	return cmdChan, nil
}

func shutdownCmdChannel() {
	if cmdChannel != nil {
		cmdChannel.Close()
		cmdChannel = nil
	}
}

var evtChannelAddr = "tcp://0.0.0.0:65502"

//var evtChannelAddr = "ipc:///tmp/docker-slim-launcher.events.ipc"
//var evtChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-launcher.events.ipc"
var evtChannel mangos.Socket

func newEvtPublisher(addr string) (mangos.Socket, error) {
	log.Println("launcher: creating event publisher...")
	socket, err := pub.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionSendDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err = socket.Listen(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}

func publishEvt(channel mangos.Socket, evt string) error {
	if err := channel.Send([]byte(evt)); err != nil {
		log.Printf("fail to publish '%v' event:%v\n", evt, err)
		return err
	}

	return nil
}

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}

//////////////

func cleanupOnStartup() {
	if _, err := os.Stat("/tmp/docker-slim-launcher.cmds.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-launcher.cmds.ipc"); err != nil {
			fmt.Printf("Error removing unix socket %s: %s", "/tmp/docker-slim-launcher.cmds.ipc", err.Error())
		}
	}

	if _, err := os.Stat("/tmp/docker-slim-launcher.events.ipc"); err == nil {
		if err := os.Remove("/tmp/docker-slim-launcher.events.ipc"); err != nil {
			fmt.Printf("Error removing unix socket %s: %s", "/tmp/docker-slim-launcher.events.ipc", err.Error())
		}
	}
}

func cleanupOnShutdown() {
	//fmt.Println("cleanupOnShutdown()...")

	if doneChan != nil {
		close(doneChan)
		doneChan = nil
	}

	shutdownCmdChannel()
	shutdownEvtChannel()
}

//////////////

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
	syscall.SIGSTOP,
	syscall.SIGCONT,
}

func initSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, signals...)
	go func() {
		sig := <-sigChan
		fmt.Printf(" launcher: cleanup on signal (%v)...\n", sig)
		cleanupOnShutdown()
		os.Exit(0)
	}()
}

///////////////////////////////////////////////////////////////////////////////

func sendPids(pidList []int) {
	pidsData, err := json.Marshal(pidList)
	failOnError(err)

	monitorSocket, err := net.Dial("unix", "/tmp/amonitor.sock")
	failOnError(err)
	defer monitorSocket.Close()

	monitorSocket.Write(pidsData)
	monitorSocket.Write([]byte("\n"))
}

/////////

type event struct {
	ID      uint32
	Pid     int32
	File    string
	IsRead  bool
	IsWrite bool
}

func check(err error) {
	if err != nil {
		log.Fatalln("monitor error:", err)
	}
}

func monitorProcess(stop chan bool) chan map[int][]int {
	log.Println("monitor: monitorProcess start")

	watcher, err := pdiscover.NewAllWatcher(pdiscover.PROC_EVENT_ALL)
	check(err)

	forksChan := make(chan map[int][]int, 1)

	go func() {
		forks := make(map[int][]int)
		s := false
		for !s {
			select {
			case <-stop:
				s = true
			case ev := <-watcher.Fork:
				forks[ev.ParentPid] = append(forks[ev.ParentPid], ev.ChildPid)
			case <-watcher.Exec:
			case <-watcher.Exit:
			case err := <-watcher.Error:
				log.Println("error: ", err)
				panic(err)
			}
		}
		forksChan <- forks
		watcher.Close()
	}()

	return forksChan
}

func getFiles(events chan map[event]bool, pidsMap chan map[int][]int, pids chan []int) []string {
	p := <-pids
	pm := <-pidsMap
	e := <-events
	allPids := make(map[int]bool, 0)

	for _, v := range p {
		allPids[v] = true
		for _, pl := range pm[v] {
			allPids[pl] = true
		}
	}

	var files []string
	for k := range e {
		_, found := allPids[int(k.Pid)]
		if found {
			files = append(files, k.File)
		}
	}
	return files
}

func getFilesAll(events chan map[event]bool) []string {
	log.Println("launcher: getFilesAll - getting events...")
	e := <-events
	log.Println("launcher: getFilesAll - event count =>", len(e))

	var files []string
	for k := range e {
		log.Println("launcher: getFilesAll - adding file =>", k.File)
		files = append(files, k.File)
	}
	return files
}

func filesToInodes(files []string) []int {
	cmd := "/usr/bin/stat"
	args := []string{"-L", "-c", "%i"}
	args = append(args, files...)
	var inodes []int

	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()
	for _, i := range strings.Split(string(out), "\n") {
		inode, err := strconv.Atoi(strings.TrimSpace(i))
		if err != nil {
			continue
		}
		inodes = append(inodes, inode)
	}
	return inodes
}

func findSymlinks(files []string, mp string) map[string]*artifactProps {
	cmd := "/usr/bin/find"
	args := []string{"-L", mp, "-mount", "-printf", "%i %p\n"}
	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()

	inodes := filesToInodes(files)
	inodeToFiles := make(map[int][]string)

	for _, v := range strings.Split(string(out), "\n") {
		v = strings.TrimSpace(v)
		info := strings.Split(v, " ")
		inode, err := strconv.Atoi(info[0])
		if err != nil {
			continue
		}
		inodeToFiles[inode] = append(inodeToFiles[inode], info[1])
	}

	result := make(map[string]*artifactProps, 0)
	for _, i := range inodes {
		v := inodeToFiles[i]
		for _, f := range v {
			result[f] = nil
		}
	}
	return result
}

func cpFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		log.Println("launcher: monitor - cp - error opening source file =>", src)
		return err
	}
	defer s.Close()

	dstDir := fileDir(dst)
	err = os.MkdirAll(dstDir, 0777)
	if err != nil {
		log.Println("launcher: monitor - dir error =>", err)
	}

	d, err := os.Create(dst)
	if err != nil {
		log.Println("launcher: monitor - cp - error opening dst file =>", dst)
		return err
	}

	srcFileInfo, err := s.Stat()
	if err == nil {
		//if (srcFileInfo.Mode() & 0111) > 0 {
		//	log.Println("TMP: launcher: monitor - cp: executable =>",src,"|perms =>",srcFileInfo.Mode())
		//}

		d.Chmod(srcFileInfo.Mode())
	}

	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func getFileHash(artifactFileName string) (string, error) {
	fileData, err := ioutil.ReadFile(artifactFileName)
	if err != nil {
		return "", err
	}

	hash := sha1.Sum(fileData)
	return hex.EncodeToString(hash[:]), nil
}

func getDataType(artifactFileName string) (string, error) {
	//TODO: use libmagic (pure impl)
	var cerr bytes.Buffer
	var cout bytes.Buffer

	cmd := exec.Command("file", artifactFileName)
	cmd.Stderr = &cerr
	cmd.Stdout = &cout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		err = fmt.Errorf("Error getting data type: %s / stderr: %s", err, cerr.String())
		return "", err
	}

	if typeInfo := strings.Split(strings.TrimSpace(cout.String()), ":"); len(typeInfo) > 1 {
		return strings.TrimSpace(typeInfo[1]), nil
	}

	return "unknown", nil
}

type artifactType int

const (
	dirArtifactType     = 1
	fileArtifactType    = 2
	symlinkArtifactType = 3
	unknownArtifactType = 99
)

var artifactTypeNames = map[artifactType]string{
	dirArtifactType:     "Dir",
	fileArtifactType:    "File",
	symlinkArtifactType: "Symlink",
	unknownArtifactType: "Unknown",
}

func (t artifactType) String() string {
	return artifactTypeNames[t]
}

type artifactProps struct {
	FileType artifactType    `json:"-"`
	FilePath string          `json:"file_path"`
	Mode     os.FileMode     `json:"-"`
	ModeText string          `json:"mode"`
	LinkRef  string          `json:"link_ref,omitempty"`
	Flags    map[string]bool `json:"flags,omitempty"`
	DataType string          `json:"data_type,omitempty"`
	FileSize int64           `json:"file_size"`
	Sha1Hash string          `json:"sha1_hash,omitempty"`
	AppType  string          `json:"app_type,omitempty"`
}

func (p *artifactProps) MarshalJSON() ([]byte, error) {
	type artifactPropsType artifactProps
	return json.Marshal(&struct {
		FileTypeStr string `json:"file_type"`
		*artifactPropsType
	}{
		FileTypeStr:       p.FileType.String(),
		artifactPropsType: (*artifactPropsType)(p),
	})
}

type artifactStore struct {
	storeLocation string
	fanMonReport  *fanMonitorReport
	ptMonReport   *ptMonitorReport
	rawNames      map[string]*artifactProps
	nameList      []string
	resolve       map[string]struct{}
	linkMap       map[string]*artifactProps
	fileMap       map[string]*artifactProps
}

func newArtifactStore(storeLocation string,
	fanMonReport *fanMonitorReport,
	rawNames map[string]*artifactProps,
	ptMonReport *ptMonitorReport) *artifactStore {
	store := &artifactStore{
		storeLocation: storeLocation,
		fanMonReport:  fanMonReport,
		ptMonReport:   ptMonReport,
		rawNames:      rawNames,
		nameList:      make([]string, 0, len(rawNames)),
		resolve:       map[string]struct{}{},
		linkMap:       map[string]*artifactProps{},
		fileMap:       map[string]*artifactProps{},
	}

	return store
}

func (p *artifactStore) getArtifactFlags(artifactFileName string) map[string]bool {
	flags := map[string]bool{}
	for _, processFileMap := range p.fanMonReport.ProcessFiles {
		if finfo, ok := processFileMap[artifactFileName]; ok {
			if finfo.ReadCount > 0 {
				flags["R"] = true
			}

			if finfo.WriteCount > 0 {
				flags["W"] = true
			}

			if finfo.ExeCount > 0 {
				flags["X"] = true
			}
		}
	}

	if len(flags) < 1 {
		return nil
	}

	return flags
}

func (p *artifactStore) prepareArtifact(artifactFileName string) {
	srcLinkFileInfo, err := os.Lstat(artifactFileName)
	if err != nil {
		log.Printf("prepareArtifact - artifact don't exist: %v (%v)\n", artifactFileName, os.IsNotExist(err))
		return
	}

	p.nameList = append(p.nameList, artifactFileName)

	props := &artifactProps{
		FilePath: artifactFileName,
		Mode:     srcLinkFileInfo.Mode(),
		ModeText: srcLinkFileInfo.Mode().String(),
		FileSize: srcLinkFileInfo.Size(),
	}

	props.Flags = p.getArtifactFlags(artifactFileName)

	switch {
	case srcLinkFileInfo.Mode().IsRegular():
		//log.Printf("prepareArtifact - is a regular file")
		props.FileType = fileArtifactType
		props.Sha1Hash, _ = getFileHash(artifactFileName)
		props.DataType, _ = getDataType(artifactFileName)
		p.fileMap[artifactFileName] = props
		p.rawNames[artifactFileName] = props
	case (srcLinkFileInfo.Mode() & os.ModeSymlink) != 0:
		//log.Printf("prepareArtifact - is a symlink")
		linkRef, err := os.Readlink(artifactFileName)
		if err != nil {
			log.Printf("prepareArtifact - error getting reference for symlink: %v\n", artifactFileName)
			return
		}

		//log.Printf("prepareArtifact(%s): src is a link! references => %s\n", artifactFileName, linkRef)
		props.FileType = symlinkArtifactType
		props.LinkRef = linkRef

		if _, ok := p.rawNames[linkRef]; !ok {
			p.resolve[linkRef] = struct{}{}
		}

		p.linkMap[artifactFileName] = props
		p.rawNames[artifactFileName] = props

	case srcLinkFileInfo.Mode().IsDir():
		log.Printf("prepareArtifact - is a directory (shouldn't see it)")
		props.FileType = dirArtifactType
	default:
		log.Printf("prepareArtifact - other type (shouldn't see it)")
	}
}

func (p *artifactStore) prepareArtifacts() {
	for artifactFileName := range p.rawNames {
		//log.Printf("prepareArtifacts - artifact => %v\n",artifactFileName)
		p.prepareArtifact(artifactFileName)
	}

	p.resolveLinks()
}

func (p *artifactStore) resolveLinks() {
	for name := range p.resolve {
		_ = name
		//log.Println("resolveLinks - resolving:", name)
		//TODO
	}
}

func (p *artifactStore) saveArtifacts() {
	for fileName := range p.fileMap {
		filePath := fmt.Sprintf("%s/files%s", p.storeLocation, fileName)
		//log.Println("saveArtifacts - saving file data =>", filePath)
		err := cpFile(fileName, filePath)
		if err != nil {
			log.Println("saveArtifacts - error saving file =>", err)
		}
	}

	for linkName, linkProps := range p.linkMap {
		linkPath := fmt.Sprintf("%s/files%s", p.storeLocation, linkName)
		linkDir := fileDir(linkPath)
		err := os.MkdirAll(linkDir, 0777)
		if err != nil {
			log.Println("saveArtifacts - dir error =>", err)
			continue
		}
		err = os.Symlink(linkProps.LinkRef, linkPath)
		if err != nil {
			log.Println("saveArtifacts - symlink create error ==>", err)
		}
	}
}

type imageReport struct {
	Files []*artifactProps `json:"files"`
}

type monitorReports struct {
	Fan *fanMonitorReport `json:"fan"`
	Pt *ptMonitorReport `json:"pt"`
}

type containerReport struct {
	Monitors monitorReports `json:"monitors"`
	Image   imageReport     `json:"image"`
}

func (p *artifactStore) saveReport() {
	sort.Strings(p.nameList)
	
	report := containerReport{
		Monitors: monitorReports{
			Pt: p.ptMonReport,
			Fan:  p.fanMonReport,
		},
	}

	for _, fname := range p.nameList {
		report.Image.Files = append(report.Image.Files, p.rawNames[fname])
	}

	artifactDirName := "/opt/dockerslim/artifacts"
	reportName := "creport.json"

	_, err := os.Stat(artifactDirName)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactDirName, 0777)
		_, err = os.Stat(artifactDirName)
		check(err)
	}

	reportFilePath := filepath.Join(artifactDirName, reportName)
	log.Println("launcher: monitor - saving report to", reportFilePath)

	reportData, err := json.MarshalIndent(report, "", "  ")
	check(err)

	err = ioutil.WriteFile(reportFilePath, reportData, 0644)
	check(err)
}

func saveResults(fanMonReport *fanMonitorReport, fileNames map[string]*artifactProps, ptMonReport *ptMonitorReport) {
	artifactDirName := "/opt/dockerslim/artifacts"

	artifactStore := newArtifactStore(artifactDirName, fanMonReport, fileNames, ptMonReport)
	artifactStore.prepareArtifacts()
	artifactStore.saveArtifacts()
	artifactStore.saveReport()
}

func writeData(monitorFileName string, files map[string]bool) {
	artifactDirName := "/opt/dockerslim/artifacts"
	/*
		err = os.MkdirAll(artifactDir, 0777)
		if err != nil {
			log.Println("launcher: monitor - artifact dir error =>", err)
		}
	*/
	_, err := os.Stat(artifactDirName)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactDirName, 0777)
		_, err = os.Stat(artifactDirName)
		check(err)
	}

	resultFile := filepath.Join(artifactDirName, monitorFileName)

	log.Println("launcher: monitor - saving results to", resultFile)
	f, err := os.Create(resultFile)
	check(err)
	defer f.Close()
	w := bufio.NewWriter(f)

	for k := range files {
		w.WriteString(k)
		w.WriteString("\n")
	}
	w.Flush()
}

func monitor(stopWork chan bool, 
	stopWorkAck chan bool, 
	pids chan []int, 
	ptmonStartChan chan int,
	appName string,
	appArgs []string,
	dirName string) {
	log.Println("launcher: monitor starting...")
	mountPoint := "/"
	//file := "/opt/dockerslim/artifacts/monitor_results"
	//monitorFileName := "monitor_results"

	//stopEvents := make(chan bool, 1)
	//stopEvents := make(chan bool)
	stopMonitor := make(chan struct{})
	//events := listenEvents(mountPoint, stopEvents)
	//reportChan := fanmon.RunMonitor(mountPoint, stopEvents)
	fanReportChan := fanRunMonitor(mountPoint, stopMonitor)
	ptReportChan := ptRunMonitor(ptmonStartChan, stopMonitor,appName, appArgs, dirName)

	//stop_process := make(chan bool, 1)
	//pidsMap := monitorProcess(stop_process)

	go func() {
		log.Println("launcher: monitor - waiting to stop monitoring...")
		<-stopWork
		log.Println("launcher: monitor - stop message...")
		//stopEvents <- true
		close(stopMonitor)
		//stop_process <- true
		log.Println("launcher: monitor - processing data...")
		//files := getFiles(events, pidsMap, pids)
		//NOTE/TODO:
		//should use getFiles() though it won't work properly for apps that spawn processes
		//because the pid list only contains the pid for the main app process
		//(when process monitoring is not used)
		//files := getFilesAll(events)
		fanReport := <-fanReportChan
		//var fanReport *fanMonitorReport
		ptReport := <-ptReportChan
		//var ptReport *ptMonitorReport

		//processCount := len(report.ProcessFiles)
		fileCount := 0
		for _, processFileMap := range fanReport.ProcessFiles {
			fileCount += len(processFileMap)
		}
		fileList := make([]string, 0, fileCount)
		for _, processFileMap := range fanReport.ProcessFiles {
			for fpath := range processFileMap {
				fileList = append(fileList, fpath)
			}
		}

		allFilesMap := findSymlinks(fileList, mountPoint)
		//allFilesMap := map[string]*artifactProps{}
		//writeData(monitorFileName, allFilesList)
		saveResults(fanReport, allFilesMap, ptReport)
		stopWorkAck <- true
	}()
}

/////////

func main() {
	log.Printf("launcher: args => %#v\n", os.Args)
	failWhen(len(os.Args) < 2, "missing app information")

	dirName, err := os.Getwd()
	failOnError(err)
	log.Printf("launcher: cwd => %#v\n", dirName)

	appName := os.Args[1]
	var appArgs []string
	if len(os.Args) > 2 {
		appArgs = os.Args[2:]
	}

	initSignalHandlers()
	defer func() {
		fmt.Println("defered cleanup on shutdown...")
		cleanupOnShutdown()
	}()

	/*
	   monitorPath := fmt.Sprintf("%s/amonitor",myFileDir())
	   log.Printf("launcher: start monitor (%v)\n",monitorPath)
	   monitorArgs := []string{
	       "-file",
	       "/opt/dockerslim/monitor_results",
	       "-socket",
	       "/tmp/amonitor.sock",
	       "-mount",
	       "/",
	   }
	   monitor := exec.Command(monitorPath,monitorArgs...)
	   err = monitor.Start()
	   failOnError(err)
	   defer monitor.Wait()
	*/

	monDoneChan := make(chan bool, 1)
	monDoneAckChan := make(chan bool)
	pidsChan := make(chan []int, 1)
	ptmonStartChan := make(chan int, 1)
	monitor(monDoneChan, monDoneAckChan, pidsChan, ptmonStartChan,appName,appArgs,dirName)

	log.Printf("launcher: start target app => %v %#v\n", appName, appArgs)

	/*
		app := exec.Command(appName, appArgs...)
		app.Dir = dirName
		app.Stdout = os.Stdout
		app.Stderr = os.Stderr
		err = app.Start()
	*/
	//app, err := startTargetApp(appName, appArgs, dirName, true)
	//failOnError(err)
	//defer app.Wait()
	//log.Printf("launcher: target app pid => %v\n", app.Process.Pid)
	time.Sleep(3 * time.Second)

	//sendPids([]int{app.Process.Pid})
	//TMP: pidsChan <- []int{app.Process.Pid}
	//TMP: ptmonStartChan <- app.Process.Pid

	log.Println("launcher: setting up channels...")
	doneChan = make(chan struct{})
	evtChannel, err = newEvtPublisher(evtChannelAddr)
	failOnError(err)
	cmdChannel, err = newCmdServer(cmdChannelAddr)
	failOnError(err)

	cmdChan, err := runCmdServer(cmdChannel, doneChan)
	failOnError(err)
	log.Println("launcher: waiting for commands...")
	doneRunning:
	for {
		select {
		case cmd := <-cmdChan:
			log.Println("\nlauncher: command =>", cmd)
			switch cmd {
			case "monitor.finish":
				log.Println("launcher: 'monitor.finish' command - stopping monitor...")
				break doneRunning
			default:
				log.Println("launcher: ignoring command =>", cmd)
			}

		case <-time.After(time.Second * 5):
			log.Printf(".")
		}
	}

	log.Println("launcher: stopping monitor...")
	//monitor.Process.Signal(syscall.SIGTERM)

	monDoneChan <- true
	log.Println("launcher: waiting for monitor to finish...")
	<-monDoneAckChan
	//time.Sleep(3 * time.Second)

	for ptry := 0; ptry < 3; ptry++ {
		log.Printf("launcher: trying to publish 'monitor.finish.completed' event (attempt %v)\n", ptry+1)
		err = publishEvt(evtChannel, "monitor.finish.completed")
		if err == nil {
			log.Println("launcher: published 'monitor.finish.completed'")
			break
		}

		switch err {
		case mangos.ErrRecvTimeout:
			log.Println("launcher: publish event timeout... ok")
		default:
			log.Println("launcher: publish event error =>", err)
		}
	}

	log.Println("launcher: done!")
}
