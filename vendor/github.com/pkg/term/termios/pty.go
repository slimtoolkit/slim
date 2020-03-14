package termios

import (
	"fmt"
	"os"
	"syscall"
)

func open_device(path string) (uintptr, error) {
	fd, err := syscall.Open(path, syscall.O_NOCTTY|syscall.O_RDWR|syscall.O_CLOEXEC, 0666)
	if err != nil {
		return 0, fmt.Errorf("unable to open %q: %v", path, err)
	}
	return uintptr(fd), nil
}

// Pty returns a UNIX 98 pseudoterminal device.
// Pty returns a pair of fds representing the master and slave pair.
func Pty() (*os.File, *os.File, error) {
	ptm, err := open_pty_master()
	if err != nil {
		return nil, nil, err
	}

	sname, err := Ptsname(ptm)
	if err != nil {
		return nil, nil, err
	}

	err = grantpt(ptm)
	if err != nil {
		return nil, nil, err
	}

	err = unlockpt(ptm)
	if err != nil {
		return nil, nil, err
	}

	pts, err := open_device(sname)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(ptm), "ptm"), os.NewFile(uintptr(pts), sname), nil
}
