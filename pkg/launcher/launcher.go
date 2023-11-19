//go:build linux
// +build linux

package launcher

import (
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/slimtoolkit/slim/pkg/system"
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

	app.Dir = appDir
	app.Stdin = os.Stdin
	app.Stdout = appStdout
	app.Stderr = appStderr

	if appUser != "" {
		if app.SysProcAttr == nil {
			app.SysProcAttr = &syscall.SysProcAttr{}
		}

		appUserParts := strings.Split(appUser, ":")
		if len(appUserParts) > 0 {
			uid, gid, home, err := system.ResolveUser(appUserParts[0])
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

				app.Env = appEnv(home)
			} else {
				log.Errorf("launcher.Start: error resolving user identity (%v/%v) - %v", appUser, appUserParts[0], err)
			}
		}
	} else {
		// This is not exactly the same as leaving app.Env unset.
		// When cmd.Env == nil, Go stdlib takes the os.Environ() list
		// **AND** adds the PWD=`cmd.Dir` unless Dir is empty. This doesn't
		// match the Docker's standard behavior - even when an image has
		// the WORKDIR set, there is no PWD env var unless it's set
		// explicitly. Thus, before this change was made to the sensor
		// logic, instrumented containers would have non-identical ENV list.
		app.Env = os.Environ()
	}

	err := app.Start()
	if err != nil {
		log.Errorf("launcher.Start: error - %v", err)
		return nil, err
	}

	log.Debugf("launcher.Start: started target app --> PID=%d", app.Process.Pid)
	return app, nil
}

func appEnv(appUserHome string) (appEnv []string) {
	// Another attempt to make the sensor's presence invisible to the target app.
	// Instrumented containers must be started as "root". But it makes Docker
	// (and other container runtimes) setting the HOME env var accordingly
	// (typically, using "/root" if /etc/passwd record exists or defaulting to "/"
	// otherwise to stay POSIX-conformant). However, when the target app needs
	// to be run as `appUser` != "root", the HOME env var may very well be different.
	// So we need to restore it by reading the corresponding /etc/passwd record.
	//
	// Note that the above logic is applicable only to the HOME env var. Other
	// typical env vars like USER or PATH don't need to be restored. Container
	// runtimes typically don't touch the USER var and almost always do set
	// the PATH var explicitly (during image building). So we just (implicitly)
	// propagate these values to app.Env from os.Environ().

	sensorUser, err := user.Current()
	if err != nil {
		log.WithError(err).Error("launcher.Start: couldn't get current user")
		return os.Environ()
	}

	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "HOME=") {
			appEnv = append(appEnv, e) // Just copy everything... except HOME.
			continue
		}

		if "HOME="+sensorUser.HomeDir == e {
			// Since current HOME var is equal to the sensor's user HomeDir,
			// it's highly likely it wasn't set explicitly in the `docker run`
			// command (or alike) and instead was "computed" by the runtime upon
			// launching the container. Since the target app user != sensor's user,
			// we need to "recompute" it.
			appEnv = append(appEnv, "HOME="+appUserHome)
		} else {
			appEnv = append(appEnv, e) // Likely the HOME var was set explicitly - don't mess with it.
		}
	}

	return appEnv
}
