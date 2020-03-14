// +build !windows

package prompt

import (
	"bytes"
	"log"
	"syscall"
	"unsafe"

	"github.com/pkg/term/termios"
)

const maxReadBytes = 1024

// PosixParser is a ConsoleParser implementation for POSIX environment.
type PosixParser struct {
	fd          int
	origTermios syscall.Termios
}

// Setup should be called before starting input
func (t *PosixParser) Setup() error {
	// Set NonBlocking mode because if syscall.Read block this goroutine, it cannot receive data from stopCh.
	if err := syscall.SetNonblock(t.fd, true); err != nil {
		log.Println("[ERROR] Cannot set non blocking mode.")
		return err
	}
	if err := t.setRawMode(); err != nil {
		log.Println("[ERROR] Cannot set raw mode.")
		return err
	}
	return nil
}

// TearDown should be called after stopping input
func (t *PosixParser) TearDown() error {
	if err := syscall.SetNonblock(t.fd, false); err != nil {
		log.Println("[ERROR] Cannot set blocking mode.")
		return err
	}
	if err := t.resetRawMode(); err != nil {
		log.Println("[ERROR] Cannot reset from raw mode.")
		return err
	}
	return nil
}

// Read returns byte array.
func (t *PosixParser) Read() ([]byte, error) {
	buf := make([]byte, maxReadBytes)
	n, err := syscall.Read(t.fd, buf)
	if err != nil {
		return []byte{}, err
	}
	return buf[:n], nil
}

func (t *PosixParser) setRawMode() error {
	x := t.origTermios.Lflag
	if x &^= syscall.ICANON; x != 0 && x == t.origTermios.Lflag {
		// fd is already raw mode
		return nil
	}
	var n syscall.Termios
	if err := termios.Tcgetattr(uintptr(t.fd), &t.origTermios); err != nil {
		return err
	}
	n = t.origTermios
	// "&^=" used like: https://play.golang.org/p/8eJw3JxS4O
	n.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	n.Cc[syscall.VMIN] = 1
	n.Cc[syscall.VTIME] = 0
	termios.Tcsetattr(uintptr(t.fd), termios.TCSANOW, &n)
	return nil
}

func (t *PosixParser) resetRawMode() error {
	if t.origTermios.Lflag == 0 {
		return nil
	}
	return termios.Tcsetattr(uintptr(t.fd), termios.TCSANOW, &t.origTermios)
}

// GetKey returns Key correspond to input byte codes.
func (t *PosixParser) GetKey(b []byte) Key {
	for _, k := range asciiSequences {
		if bytes.Equal(k.ASCIICode, b) {
			return k.Key
		}
	}
	return NotDefined
}

// winsize is winsize struct got from the ioctl(2) system call.
type ioctlWinsize struct {
	Row uint16
	Col uint16
	X   uint16 // pixel value
	Y   uint16 // pixel value
}

// GetWinSize returns WinSize object to represent width and height of terminal.
func (t *PosixParser) GetWinSize() *WinSize {
	ws := &ioctlWinsize{}
	retCode, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.fd),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	return &WinSize{
		Row: ws.Row,
		Col: ws.Col,
	}
}

var _ ConsoleParser = &PosixParser{}

// NewStandardInputParser returns ConsoleParser object to read from stdin.
func NewStandardInputParser() *PosixParser {
	in, err := syscall.Open("/dev/tty", syscall.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}

	return &PosixParser{
		fd: in,
	}
}
