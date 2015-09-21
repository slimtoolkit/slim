package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    //"syscall"
    "time"
    "path/filepath"
    "net"
    "encoding/json"
    "bufio"
    "strconv"
    "strings"
    "io"

    "bitbucket.org/madmo/fanotify"
    "github.com/cloudimmunity/pdiscover"
)

func failOnError(err error) {
    if err != nil {
        log.Fatalln("launcher: ERROR =>",err)
    }
}

func failWhen(cond bool,msg string) {
    if cond {
        log.Fatalln("launcher: ERROR =>",msg)
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

func sendPids(pidList []int) {
    pidsData, err := json.Marshal(pidList)
    failOnError(err)

    monitorSocket,err := net.Dial("unix","/tmp/amonitor.sock")
    failOnError(err)
    defer monitorSocket.Close()

    monitorSocket.Write(pidsData)
    monitorSocket.Write([]byte("\n"))
}

/////////

type event struct {
    Pid  int32
    File string
}

func check(err error) {
    if err != nil {
        log.Fatalln("monitor error:", err)
    }
}

func listen_events(mount_point string, stop chan bool) chan map[event]bool {
    log.Println("monitor: listen_events start")

    nd, err := fanotify.Initialize(fanotify.FAN_CLASS_NOTIF, os.O_RDONLY)
    check(err)
    err = nd.Mark(fanotify.FAN_MARK_ADD|fanotify.FAN_MARK_MOUNT, fanotify.FAN_ACCESS|fanotify.FAN_OPEN, -1, mount_point)
    check(err)

    events_chan := make(chan map[event]bool, 1)

    go func() {
        log.Println("monitor: listen_events worker starting")
        events := make(map[event]bool, 1)
        eventChan := make(chan event)
        go func() {
            for {
                data, err := nd.GetEvent()
                check(err)
                path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", data.File.Fd()))
                check(err)
                e := event{data.Pid, path}
                data.File.Close()
                eventChan <- e
            }
        }()

        s := false
        for !s {
            select {
            case <- time.After(20 * time.Second):
                log.Println("monitor: listen_events - event timeout...")
                s = true
            case <-stop:
                log.Println("monitor: listen_events stopping...")
                s = true
            case e := <- eventChan:
                events[e] = true
                log.Printf("monitor: listen_events event => %#v\n",e)
            }
        }

        log.Printf("monitor: listen_events sending %v events...\n",len(events))
        events_chan <- events
    }()

    return events_chan
}

func monitor_process(stop chan bool) chan map[int][]int {
    log.Println("monitor: monitor_process start")

    watcher, err := pdiscover.NewAllWatcher(pdiscover.PROC_EVENT_ALL)
    check(err)

    forks_chan := make(chan map[int][]int, 1)

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
        forks_chan <- forks
        watcher.Close()
    }()

    return forks_chan
}

func get_files(events chan map[event]bool, pids_map chan map[int][]int, pids chan []int) []string {
    p := <-pids
    pm := <-pids_map
    e := <-events
    all_pids := make(map[int]bool, 0)

    for _, v := range p {
        all_pids[v] = true
        for _, pl := range pm[v] {
            all_pids[pl] = true
        }
    }

    files := make([]string, 0)
    for k, _ := range e {
        _, found := all_pids[int(k.Pid)]
        if found {
            files = append(files, k.File)
        }
    }
    return files
}

func get_files_all(events chan map[event]bool) []string {
    log.Println("launcher: get_files_all - getting events...")
    e := <-events
    log.Println("launcher: get_files_all - event count =>",len(e))
    files := make([]string, 0)
    for k, _ := range e {
        log.Println("launcher: get_files_all - adding file =>",k.File)
        files = append(files, k.File)
    }
    return files
}

func files_to_inodes(files []string) []int {
    cmd := "/usr/bin/stat"
    args := []string{"-L", "-c", "%i"}
    args = append(args, files...)
    inodes := make([]int, 0)

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

func find_symlinks(files []string, mp string) map[string]bool {
    cmd := "/usr/bin/find"
    args := []string{"-L", mp, "-mount", "-printf", "%i %p\n"}
    c := exec.Command(cmd, args...)
    out, _ := c.Output()
    c.Wait()

    inodes := files_to_inodes(files)
    inode_to_files := make(map[int][]string)

    for _, v := range strings.Split(string(out), "\n") {
        v = strings.TrimSpace(v)
        info := strings.Split(v, " ")
        inode, err := strconv.Atoi(info[0])
        if err != nil {
            continue
        }
        inode_to_files[inode] = append(inode_to_files[inode], info[1])
    }

    result := make(map[string]bool, 0)
    for _, i := range inodes {
        v := inode_to_files[i]
        for _, f := range v {
            result[f] = true
        }
    }
    return result
}

func cp(src, dst string) error {
    s, err := os.Open(src)
    if err != nil {
        log.Println("launcher: monitor - cp - error opening source file =>",src)
        return err
    }
    defer s.Close()

    dstDir := fileDir(dst)
    err = os.MkdirAll(dstDir,0777)
    if err != nil {
        log.Println("launcher: monitor - dir error =>",err)
    }

    d, err := os.Create(dst)
    if err != nil {
        log.Println("launcher: monitor - cp - error opening dst file =>",dst)
        return err
    }
    if _, err := io.Copy(d, s); err != nil {
        d.Close()
        return err
    }
    return d.Close()
}

func write_data(result_file string, files map[string]bool) {
    f, err := os.Create(result_file)
    check(err)
    defer f.Close()
    w := bufio.NewWriter(f)

    artifactDir := "/opt/dockerslim/app_artifacts"
    err = os.MkdirAll(artifactDir,0777)
    if err != nil {
        log.Println("launcher: monitor - artifact dir error =>",err)
    }

    for k, _ := range files {
        log.Println("launcher: monitor - saving file record =>",k)
        w.WriteString(k)
        w.WriteString("\n")

        filePath := fmt.Sprintf("%s%s",artifactDir,k)
        log.Println("launcher: monitor - saving file data =>",filePath)
        err := cp(k,filePath)
        if err != nil {
            log.Println("launcher: monitor - error saving file =>",err)
        }
    }
    w.Flush()
}

func monitor(stop_work chan bool,stop_work_ack chan bool,pids chan []int) {
    log.Println("launcher: monitor starting...")
    mount_point := "/"
    file := "/opt/dockerslim/monitor_results"

    stop_events := make(chan bool,1)
    events := listen_events(mount_point, stop_events)

    //stop_process := make(chan bool, 1)
    //pids_map := monitor_process(stop_process)

    go func() {
        log.Println("launcher: monitor - waiting to stop monitoring...")
        <-stop_work
        log.Println("launcher: monitor - stop message...")
        stop_events <- true
        //stop_process <- true
        log.Println("launcher: monitor - processing data...")
        //files := get_files(events, pids_map, pids)
        files := get_files_all(events)
        all_files := find_symlinks(files, mount_point)
        log.Println("launcher: monitor - saving results to",file)
        write_data(file, all_files)
        stop_work_ack <- true
    }()
}

/////////


func main() {
    log.Printf("launcher: args => %#v\n",os.Args)
    failWhen(len(os.Args) < 2,"missing app information")

    dirName,err := os.Getwd()
    failOnError(err)
    log.Printf("launcher: cwd => %#v\n",dirName)

    appName := os.Args[1]
    var appArgs []string
    if len(os.Args) > 2 {
        appArgs = os.Args[2:]
    }
    
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
    monitor(monDoneChan,monDoneAckChan,pidsChan)

    log.Printf("launcher: start target app => %v %#v\n",appName,appArgs)

    app := exec.Command(appName,appArgs...)
    app.Dir = dirName
    app.Stdout = os.Stdout
    app.Stderr = os.Stderr

    err = app.Start()
    failOnError(err)
    defer app.Wait()
    log.Printf("launcher: target app pid => %v\n",app.Process.Pid)
    time.Sleep(3 * time.Second)
    
    //sendPids([]int{app.Process.Pid})
    pidsChan <- []int{app.Process.Pid}

    log.Println("alauncher: waiting for monitor:")
    endTime := time.After(67 * time.Second)
    work := 0
    doneRunning:
    for {
        select {
            case <- endTime:
                log.Println("\nalauncher: done waiting :)")
                break doneRunning
            case <- time.After(time.Second * 5):
                work++
                log.Printf(".")
        }
    }

    log.Println("launcher: stopping monitor...")
    //monitor.Process.Signal(syscall.SIGTERM)
    monDoneChan <- true
    log.Println("launcher: waiting for monitor to finish...")
    <- monDoneAckChan
    //time.Sleep(3 * time.Second)

    log.Println("launcher: done!")
}

