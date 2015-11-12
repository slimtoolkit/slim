package main

import (
	"log"
	"os"
	"os/exec"
	//"runtime"
	"syscall"
)

func startTargetApp(appName string, appArgs []string, appDir string, doPtrace bool) (*exec.Cmd, error) {
	log.Printf("launcher.startTargetApp(%v,%v,%v)\n",appName,appArgs,appDir)
	app := exec.Command(appName, appArgs...)

	if doPtrace {
		app.SysProcAttr = &syscall.SysProcAttr{
			Ptrace:    true,
			Pdeathsig: syscall.SIGKILL,
		}
	}

	app.Dir = appDir
	app.Stdout = os.Stdout
	app.Stderr = os.Stderr
	app.Stdin = os.Stdin

	err := app.Start()
	if err != nil {
		log.Printf("app.Start error: %v", err)
		return nil, err
	}

	log.Printf("launcher.startTargetApp: started target app --> PID=%d\n", app.Process.Pid)
	return app, nil
}
