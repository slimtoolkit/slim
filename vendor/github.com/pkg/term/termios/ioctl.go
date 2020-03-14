// +build !windows,!solaris

package termios

import "syscall"

func ioctl(fd, request, argp uintptr) error {
	if _, _, e := syscall.Syscall6(syscall.SYS_IOCTL, fd, request, argp, 0, 0, 0); e != 0 {
		return e
	}
	return nil
}
