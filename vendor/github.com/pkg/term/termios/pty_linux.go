package termios

import (
	"fmt"
	"syscall"
	"unsafe"
)

func open_pty_master() (uintptr, error) {
	return open_device("/dev/ptmx")
}

func Ptsname(fd uintptr) (string, error) {
	var n uintptr
	err := ioctl(fd, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n)))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/dev/pts/%d", n), nil
}

func grantpt(fd uintptr) error {
	var n uintptr
	return ioctl(fd, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n)))
}

func unlockpt(fd uintptr) error {
	var n uintptr
	return ioctl(fd, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&n)))
}
