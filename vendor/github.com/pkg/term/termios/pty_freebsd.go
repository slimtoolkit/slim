package termios

import (
	"fmt"
	"syscall"
	"unsafe"
)

func posix_openpt(oflag int) (fd uintptr, err error) {
	// Copied from debian-golang-pty/pty_freebsd.go.
	r0, _, e1 := syscall.Syscall(syscall.SYS_POSIX_OPENPT, uintptr(oflag), 0, 0)
	fd = uintptr(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

func open_pty_master() (uintptr, error) {
	return posix_openpt(syscall.O_NOCTTY|syscall.O_RDWR|syscall.O_CLOEXEC)
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
	return nil
}
