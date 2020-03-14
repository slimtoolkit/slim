package termios

import (
	"syscall"
	"unsafe"
)

const (
	TCSETS  = 0x5402
	TCSETSW = 0x5403
	TCSETSF = 0x5404
	TCFLSH  = 0x540B
	TCSBRK  = 0x5409
	TCSBRKP = 0x5425

	IXON    = 0x00000400
	IXANY   = 0x00000800
	IXOFF   = 0x00001000
	CRTSCTS = 0x80000000
)

// Tcgetattr gets the current serial port settings.
func Tcgetattr(fd uintptr, argp *syscall.Termios) error {
	return ioctl(fd, syscall.TCGETS, uintptr(unsafe.Pointer(argp)))
}

// Tcsetattr sets the current serial port settings.
func Tcsetattr(fd, action uintptr, argp *syscall.Termios) error {
	var request uintptr
	switch action {
	case TCSANOW:
		request = TCSETS
	case TCSADRAIN:
		request = TCSETSW
	case TCSAFLUSH:
		request = TCSETSF
	default:
		return syscall.EINVAL
	}
	return ioctl(fd, request, uintptr(unsafe.Pointer(argp)))
}

// Tcsendbreak transmits a continuous stream of zero-valued bits for a specific
// duration, if the terminal is using asynchronous serial data transmission. If
// duration is zero, it transmits zero-valued bits for at least 0.25 seconds, and not more that 0.5 seconds.
// If duration is not zero, it sends zero-valued bits for some
// implementation-defined length of time.
func Tcsendbreak(fd, duration uintptr) error {
	return ioctl(fd, TCSBRKP, duration)
}

// Tcdrain waits until all output written to the object referred to by fd has been transmitted.
func Tcdrain(fd uintptr) error {
	// simulate drain with TCSADRAIN
	var attr syscall.Termios
	if err := Tcgetattr(fd, &attr); err != nil {
		return err
	}
	return Tcsetattr(fd, TCSADRAIN, &attr)
}

// Tcflush discards data written to the object referred to by fd but not transmitted, or data received but not read, depending on the value of selector.
func Tcflush(fd, selector uintptr) error {
	return ioctl(fd, TCFLSH, selector)
}

// Tiocinq returns the number of bytes in the input buffer.
func Tiocinq(fd uintptr, argp *int) error {
	return ioctl(fd, syscall.TIOCINQ, uintptr(unsafe.Pointer(argp)))
}

// Tiocoutq return the number of bytes in the output buffer.
func Tiocoutq(fd uintptr, argp *int) error {
	return ioctl(fd, syscall.TIOCOUTQ, uintptr(unsafe.Pointer(argp)))
}

// Cfgetispeed returns the input baud rate stored in the termios structure.
func Cfgetispeed(attr *syscall.Termios) uint32 { return attr.Ispeed }

// Cfgetospeed returns the output baud rate stored in the termios structure.
func Cfgetospeed(attr *syscall.Termios) uint32 { return attr.Ospeed }
