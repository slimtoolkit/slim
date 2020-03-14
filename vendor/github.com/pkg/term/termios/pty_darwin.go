package termios

import (
	"errors"
	"syscall"
	"unsafe"
)

func open_pty_master() (uintptr, error) {
	return open_device("/dev/ptmx")
}

func Ptsname(fd uintptr) (string, error) {
	n := make([]byte, _IOC_PARM_LEN(syscall.TIOCPTYGNAME))

	err := ioctl(fd, syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&n[0])))
	if err != nil {
		return "", err
	}

	for i, c := range n {
		if c == 0 {
			return string(n[:i]), nil
		}
	}
	return "", errors.New("TIOCPTYGNAME string not NUL-terminated")
}

func grantpt(fd uintptr) error {
	return ioctl(fd, syscall.TIOCPTYGRANT, 0)
}

func unlockpt(fd uintptr) error {
	return ioctl(fd, syscall.TIOCPTYUNLK, 0)
}
