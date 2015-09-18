package main

import (
    "log"
    "os"
    "os/exec"
)

func failOnError(err error) {
    if err != nil {
        log.Fatalln("ERROR =>",err)
    }
}

func failWhen(cond bool,msg string) {
    if cond {
        log.Fatalln("ERROR =>",msg)
    }
}

//TODO:
//1. set working directory
//2. pass/set env variables
func main() {
    var err error
    log.Printf("launcher: args => %#v\n",os.Args)
    failWhen(len(os.Args) < 2,"missing app information")

    appName := os.Args[1]
    var appArgs []string
    if len(os.Args) > 2 {
        appArgs = os.Args[2:]
    }
    
    log.Println("launcher: start monitor")
    monitor := exec.Command("amonitor_linux")
    err = monitor.Start()
    failOnError(err)
    defer monitor.Wait()

    log.Printf("launcher: start target app => %v %#v\n",appName,appArgs)
    app := exec.Command(appName,appArgs...)
    app.Stdout = os.Stdout
    app.Stderr = os.Stderr

    err = app.Start()
    failOnError(err)
    defer app.Wait()

    log.Println("launcher: target app is running...")
}

