// example of usage:
//  ./monitor -file full_path_to_the_result_file -socket /tmp/monitoring.sock -mount path_to_directory_which_should_be_monitored

//  SIGUSR1 - starts monitoring process (here just prints to stdout)
//  SIGTERM - stops monitoring and flushes info to file

package main

/* TODO: refactor so we can share the monitor code with the launcher process...
import (
	"bitbucket.org/madmo/fanotify"
	"github.com/cloudimmunity/pdiscover"

	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

type event struct {
	Pid  int32
	File string
}

func check(err error) {
	if err != nil {
		log.Fatalln("monitor error:", err)
	}
}

func listen_signals() chan bool {
	log.Println("monitor: listen_signals start")

	//start_work := make(chan bool, 1)
	stop_work := make(chan bool, 1)
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, syscall.SIGUSR1, syscall.SIGTERM)

	go func() {
		for {
			s := <-signals
			switch s {
			//case syscall.SIGUSR1:
			//	start_work <- true
			case syscall.SIGTERM:
				stop_work <- true
			}
		}
	}()

	//<-start_work
	return stop_work
}

func listen_pids(socket string) chan []int {
	log.Printf("monitor: liste_pids start (%v)\n", socket)

	p := make(chan []int, 1)

	go func() {
		l, err := net.Listen("unix", socket)
		check(err)
		defer l.Close()

		c, err := l.Accept()
		check(err)
		defer c.Close()

		d := byte('\n')
		data, err := bufio.NewReader(c).ReadBytes(d)
		if err != io.EOF && data[len(data)-1] != d {
			panic(err)
		}

		var pids []int
		err = json.Unmarshal(data[:len(data)-1], &pids)
		check(err)

		p <- pids
	}()

	return p
}

func parse_flags() (string, string, string) {
	mp := flag.String("mount", "/", "mount point")
	f := flag.String("file", "test", "file")
	socket := flag.String("socket", "/tmp/monitoring.sock", "unix socket")
	flag.Parse()
	return *socket, *mp, *f
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

func write_data(result_file string, files map[string]bool) {
	f, err := os.Create(result_file)
	check(err)
	defer f.Close()
	w := bufio.NewWriter(f)

	for k, _ := range files {
		w.WriteString(k)
		w.WriteString("\n")
	}
	w.Flush()
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

func listen_events(mount_point string, stop chan bool) chan map[event]bool {
	log.Println("monitor: listen_events start")

	nd, err := fanotify.Initialize(fanotify.FAN_CLASS_NOTIF, os.O_RDONLY)
	check(err)
	err = nd.Mark(fanotify.FAN_MARK_ADD|fanotify.FAN_MARK_MOUNT, fanotify.FAN_ACCESS|fanotify.FAN_OPEN, -1, mount_point)
	check(err)

	events_chan := make(chan map[event]bool, 1)

	go func() {
		events := make(map[event]bool, 1)
		s := false
		for !s {
			select {
			case <-stop:
				s = true
			default:
				data, err := nd.GetEvent()
				check(err)
				path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", data.File.Fd()))
				check(err)
				e := event{data.Pid, path}
				data.File.Close()
				events[e] = true
			}
		}
		events_chan <- events
	}()

	return events_chan
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

func main() {
	log.Println("monitor: starting...")
	socket, mount_point, file := parse_flags()
	log.Printf("monitor: socket=%v base_path=%v result_file=%v\n", socket, mount_point, file)

	stop_work := listen_signals()
	stop_events := make(chan bool, 1)
	events := listen_events(mount_point, stop_events)

	stop_process := make(chan bool, 1)
	pids_map := monitor_process(stop_process)

	pids := listen_pids(socket)

	<-stop_work
	log.Println("monitor: stop signal...")
	stop_events <- true
	stop_process <- true
	log.Println("monitor: processing data...")
	files := get_files(events, pids_map, pids)
	all_files := find_symlinks(files, mount_point)
	log.Println("monitor: saving results to", file)
	write_data(file, all_files)
}
*/
