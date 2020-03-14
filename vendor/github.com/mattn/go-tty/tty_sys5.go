// +build solaris

package tty

import (
	"golang.org/x/sys/unix"
)

const (
	ioctlReadTermios  = unix.TCGETA
	ioctlWriteTermios = unix.TCSETA
)
