// +build linux

package target

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"golang.org/x/sys/unix"

	log "github.com/sirupsen/logrus"
)

func fixStdioPermissions(uid int) error {
	var null unix.Stat_t
	if err := unix.Stat("/dev/null", &null); err != nil {
		return err
	}
	for _, fd := range []uintptr{
		os.Stdin.Fd(),
		os.Stderr.Fd(),
		os.Stdout.Fd(),
	} {
		var s unix.Stat_t
		if err := unix.Fstat(int(fd), &s); err != nil {
			return err
		}

		if s.Rdev == null.Rdev {
			continue
		}

		if err := unix.Fchown(int(fd), uid, int(s.Gid)); err != nil {
			if err == unix.EINVAL || err == unix.EPERM {
				continue
			}
			return err
		}
	}
	return nil
}

// Start starts the target application in the container
func Start(appName string, appArgs []string, appDir, appUser string, doPtrace bool) (*exec.Cmd, error) {
	log.Debugf("sensor.startTargetApp(%v,%v,%v,%v)", appName, appArgs, appDir, appUser)
	//TODO: need a flag to not run the target as user 
	//(even if the image Dockerfile is using the USER instruction)
	//to have the original behavior
	//appUser = "" //tmp

	app := exec.Command(appName, appArgs...)

	if doPtrace {
		app.SysProcAttr = &syscall.SysProcAttr{
			Ptrace:    true,
			Pdeathsig: syscall.SIGKILL,
		}
	}

	if appUser != "" {
		if app.SysProcAttr == nil {
			app.SysProcAttr = &syscall.SysProcAttr{}
		}

		userInfo, err := user.Lookup(appUser)
		if err == nil {
			var gid int64
			uid, err := strconv.ParseInt(userInfo.Uid, 0, 32)
			if err == nil {
				gid, err = strconv.ParseInt(userInfo.Gid, 0, 32)
				if err == nil {
					app.SysProcAttr.Credential = &syscall.Credential{
						Uid: uint32(uid),
						Gid: uint32(gid),
					}

					log.Debugf("sensor.startTargetApp: start target as user (%s) - (uid=%d,gid=%d)", appUser, uid, gid)

					if err = fixStdioPermissions(int(uid)); err != nil {
						log.Errorf("sensor.startTargetApp: error fixing i/o perms for user (%v/%v) - %v", appUser, uid, err)
					}
				} else {
					log.Errorf("sensor.startTargetApp: error converting user gid (%v) - %v", appUser, err)
				}
			} else {
				log.Errorf("sensor.startTargetApp: error converting user uid (%v) - %v", appUser, err)
			}
		} else {
			log.Errorf("sensor.startTargetApp: error getting user info (%v) - %v", appUser, err)
		}

	}

	app.Dir = appDir
	app.Stdout = os.Stdout
	app.Stderr = os.Stderr
	app.Stdin = os.Stdin

	err := app.Start()
	if err != nil {
		log.Warnf("app.Start error: %v", err)
		return nil, err
	}

	log.Debugf("sensor.startTargetApp: started target app --> PID=%d", app.Process.Pid)
	return app, nil
}
