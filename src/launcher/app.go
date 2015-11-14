package main

import (
	"os"
	"os/exec"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

func startTargetApp(appName string, appArgs []string, appDir string, doPtrace bool) (*exec.Cmd, error) {
	log.Debugf("launcher.startTargetApp(%v,%v,%v)\n", appName, appArgs, appDir)
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
		log.Warnf("app.Start error: %v\n", err)
		return nil, err
	}

	log.Debugf("launcher.startTargetApp: started target app --> PID=%d\n", app.Process.Pid)
	return app, nil
}
