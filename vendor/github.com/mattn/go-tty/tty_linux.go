// +build linux

package tty

const (
	ioctlReadTermios  = 0x5401 // syscall.TCGETS
	ioctlWriteTermios = 0x5402 // syscall.TCSETS
)
