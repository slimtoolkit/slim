package termios

import (
	"bytes"

	"golang.org/x/sys/unix"
)

func open_pty_master() (uintptr, error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_NOCTTY|unix.O_RDWR, 0666)
	if err != nil {
		return 0, err
	}
	return uintptr(fd), nil
}

func Ptsname(fd uintptr) (string, error) {
	ptm, err := unix.IoctlGetPtmget(int(fd), unix.TIOCPTSNAME)
	if err != nil {
		return "", err
	}
	return string(ptm.Sn[:bytes.IndexByte(ptm.Sn[:], 0)]), nil
}

func grantpt(fd uintptr) error {
	return unix.IoctlSetInt(int(fd), unix.TIOCGRANTPT, 0)
}

func unlockpt(fd uintptr) error {
	return nil
}
