// example of usage:
//  ./monitor -file full_path_to_the_result_file -pid 2,3,4,5 -mount path_to_directory_which_should_be_monitored

//  SIGUSR1 - starts monitoring process (here just prints to stdout)
//  SIGTERM - stops monitoring and flushes info to file

package main

import (
    "bufio"
    "encoding/json"
    "flag"
    "log"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"
)

type event struct {
    Pid    int
    File   string
    Action string
}

func check(err error) {
    if err != nil {
        log.Fatalln("monitor error:",err)
    }
}

func write_data(pids []int, mount_point, result_file string) {
    f, err := os.Create(result_file)
    check(err)
    defer f.Close()
    w := bufio.NewWriter(f)

    for _, v := range pids {
        event, err := json.Marshal(event{v, "/bin/ping", "open"})
        check(err)
        w.Write(event)
    }

    w.Flush()
    os.Exit(0)
}

func main() {
    sigs_start := make(chan os.Signal, 1)
    sigs_stop := make(chan os.Signal, 1)

    signal.Notify(sigs_start, syscall.SIGUSR1)
    signal.Notify(sigs_stop, syscall.SIGTERM)

    pids := flag.String("pid", "1, 2", "pids")
    mp := flag.String("mount", "/", "mount point")
    rf := flag.String("file", "test", "file")
    flag.Parse()

    numbers := strings.Split(*pids, ",")
    pid_slice := make([]int, len(numbers))

    for i, v := range numbers {
        n, err := strconv.Atoi(v)
        check(err)
        pid_slice[i] = n
    }

    for {
        select {
        case <-sigs_start:
            log.Println("start working")
        case <-sigs_stop:
            write_data(pid_slice, *mp, *rf)
        }
    }
}
