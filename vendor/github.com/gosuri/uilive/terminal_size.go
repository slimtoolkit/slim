package uilive

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

type windowSize struct {
	rows    uint16
	cols    uint16
	xPixels uint16
	yPixels uint16
}

var out *os.File
var err error
var sz windowSize

func getTermSize() (int, int) {
	if runtime.GOOS == "openbsd" {
		out, err = os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			os.Exit(1)
		}

	} else {
		out, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err != nil {
			os.Exit(1)
		}
	}
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL,
		out.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&sz)))
	return int(sz.cols), int(sz.rows)
}
