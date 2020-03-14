package termios

import "golang.org/x/sys/unix"

func ioctl(fd, request, argp uintptr) error {
	return unix.IoctlSetInt(int(fd), uint(request), int(argp))
}
