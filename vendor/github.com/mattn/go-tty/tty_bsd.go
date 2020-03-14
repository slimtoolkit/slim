// +build darwin dragonfly freebsd netbsd openbsd

package tty

import (
	"syscall"
)

const (
	ioctlReadTermios  = syscall.TIOCGETA
	ioctlWriteTermios = syscall.TIOCSETA
)
