package termios

// #include <termios.h>
// typedef struct termios termios_t;
import "C"

import (
	"syscall"

	"golang.org/x/sys/unix"
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

// See /usr/include/sys/termios.h
const FIORDCHK = C.FIORDCHK

// Tcgetattr gets the current serial port settings.
func Tcgetattr(fd uintptr, argp *syscall.Termios) error {
	termios, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	*argp = *(tiosToSyscall(termios))
	return err
}

// Tcsetattr sets the current serial port settings.
func Tcsetattr(fd, action uintptr, argp *syscall.Termios) error {
	return unix.IoctlSetTermios(int(fd), uint(action), tiosToUnix(argp))
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
func Tiocinq(fd uintptr, argp *int) (err error) {
	*argp, err = unix.IoctlGetInt(int(fd), FIORDCHK)
	return err
}

// Tiocoutq return the number of bytes in the output buffer.
func Tiocoutq(fd uintptr, argp *int) error {
	return ioctl(fd, syscall.TIOCOUTQ, uintptr(unsafe.Pointer(argp)))
}

// Cfgetispeed returns the input baud rate stored in the termios structure.
func Cfgetispeed(attr *syscall.Termios) uint32 {
	solTermios := tiosToUnix(attr)
	return uint32(C.cfgetispeed((*C.termios_t)(unsafe.Pointer(solTermios))))
}

// Cfsetispeed sets the input baud rate stored in the termios structure.
func Cfsetispeed(attr *syscall.Termios, speed uintptr) error {
	solTermios := tiosToUnix(attr)
	_, err := C.cfsetispeed((*C.termios_t)(unsafe.Pointer(solTermios)), C.speed_t(speed))
	return err
}

// Cfgetospeed returns the output baud rate stored in the termios structure.
func Cfgetospeed(attr *syscall.Termios) uint32 {
	solTermios := tiosToUnix(attr)
	return uint32(C.cfgetospeed((*C.termios_t)(unsafe.Pointer(solTermios))))
}

// Cfsetospeed sets the output baud rate stored in the termios structure.
func Cfsetospeed(attr *syscall.Termios, speed uintptr) error {
	solTermios := tiosToUnix(attr)
	_, err := C.cfsetospeed((*C.termios_t)(unsafe.Pointer(solTermios)), C.speed_t(speed))
	return err
}


// tiosToUnix copies a syscall.Termios to a x/sys/unix.Termios.
// This is needed since type conversions between the two fail due to
// more recent x/sys/unix.Termios renaming the padding field.
func tiosToUnix(st *syscall.Termios) *unix.Termios {
	return &unix.Termios{
		Iflag:  st.Iflag,
		Oflag:  st.Oflag,
		Cflag:  st.Cflag,
		Lflag:  st.Lflag,
		Cc:     st.Cc,
	}
}

// tiosToSyscall copies a x/sys/unix.Termios to a syscall.Termios.
func tiosToSyscall(ut *unix.Termios) *syscall.Termios {
	return &syscall.Termios{
		Iflag:  ut.Iflag,
		Oflag:  ut.Oflag,
		Cflag:  ut.Cflag,
		Lflag:  ut.Lflag,
		Cc:     ut.Cc,
	}
}
