// +build dragonfly openbsd

package termios

// #include<stdlib.h>
import "C"

import "syscall"

func open_pty_master() (uintptr, error) {
	rc := C.posix_openpt(syscall.O_NOCTTY | syscall.O_RDWR)
	if rc < 0 {
		return 0, syscall.Errno(rc)
	}
	return uintptr(rc), nil
}

func Ptsname(fd uintptr) (string, error) {
	slavename := C.GoString(C.ptsname(C.int(fd)))
	return slavename, nil
}

func grantpt(fd uintptr) error {
	rc := C.grantpt(C.int(fd))
	if rc == 0 {
		return nil
	}
	return syscall.Errno(rc)
}

func unlockpt(fd uintptr) error {
	rc := C.unlockpt(C.int(fd))
	if rc == 0 {
		return nil
	}
	return syscall.Errno(rc)
}
