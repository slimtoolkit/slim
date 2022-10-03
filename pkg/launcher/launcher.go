//go:build linux
// +build linux

package launcher

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/docker-slim/docker-slim/pkg/system"
)

// copied from libcontainer
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

func fixStdioPermissionsAlt(uid int) error {
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

		if err := unix.Fchmod(int(fd), 0777); err != nil {
			if err == unix.EINVAL || err == unix.EPERM {
				continue
			}
			return err
		}
	}
	return nil
}

// Start starts the target application in the container
func Start(
	appName string,
	appArgs []string,
	appDir string,
	appUser string,
	runTargetAsUser bool,
	doPtrace bool,
	appStdout io.Writer,
	appStderr io.Writer,
) (*exec.Cmd, error) {
	log.Debugf("launcher.Start(%v,%v,%v,%v)", appName, appArgs, appDir, appUser)
	if !runTargetAsUser {
		appUser = ""
	}

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

		appUserParts := strings.Split(appUser, ":")
		if len(appUserParts) > 0 {
			uid, gid, err := system.ResolveUser(appUserParts[0])
			if err == nil {
				if len(appUserParts) > 1 {
					xgid, err := system.ResolveGroup(appUserParts[1])
					if err == nil {
						gid = xgid
					} else {
						log.Errorf("launcher.Start: error resolving group identity (%v/%v) - %v", appUser, appUserParts[1], err)
					}
				}

				app.SysProcAttr.Credential = &syscall.Credential{
					Uid: uid,
					Gid: gid,
				}

				log.Debugf("launcher.Start: start target as user (%s) - (uid=%d,gid=%d)", appUser, uid, gid)

				if err = fixStdioPermissions(int(uid)); err != nil {
					log.Errorf("launcher.Start: error fixing i/o perms for user (%v/%v) - %v", appUser, uid, err)
				}

			} else {
				log.Errorf("launcher.Start: error resolving user identity (%v/%v) - %v", appUser, appUserParts[0], err)
			}
		}
	}

	app.Dir = appDir
	app.Stdin = os.Stdin
	app.Stdout = appStdout
	app.Stderr = appStderr

	err := app.Start()
	if err != nil {
		log.Warnf("launcher.Start: error - %v", err)
		return nil, err
	}

	log.Debugf("launcher.Start: started target app --> PID=%d", app.Process.Pid)
	return app, nil
}
