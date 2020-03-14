package tty

import (
	"os"
	"strings"
	"unicode"
)

func Open() (*TTY, error) {
	return open("/dev/tty")
}

func OpenDevice(path string) (*TTY, error) {
	return open(path)
}

func (tty *TTY) Raw() (func() error, error) {
	return tty.raw()
}

func (tty *TTY) MustRaw() func() error {
	f, err := tty.raw()
	if err != nil {
		panic(err.Error())
	}
	return f
}

func (tty *TTY) Buffered() bool {
	return tty.buffered()
}

func (tty *TTY) ReadRune() (rune, error) {
	return tty.readRune()
}

func (tty *TTY) Close() error {
	return tty.close()
}

func (tty *TTY) Size() (int, int, error) {
	return tty.size()
}

func (tty *TTY) SizePixel() (int, int, int, int, error) {
	return tty.sizePixel()
}

func (tty *TTY) Input() *os.File {
	return tty.input()
}

func (tty *TTY) Output() *os.File {
	return tty.output()
}

// Display types.
const (
	displayNone = iota
	displayRune
	displayMask
)

func (tty *TTY) readString(displayType int) (string, error) {
	rs := []rune{}
loop:
	for {
		r, err := tty.readRune()
		if err != nil {
			return "", err
		}
		switch r {
		case 13:
			break loop
		case 8, 127:
			if len(rs) > 0 {
				rs = rs[:len(rs)-1]
				if displayType != displayNone {
					tty.Output().WriteString("\b \b")
				}
			}
		default:
			if unicode.IsPrint(r) {
				rs = append(rs, r)
				switch displayType {
				case displayRune:
					tty.Output().WriteString(string(r))
				case displayMask:
					tty.Output().WriteString("*")
				}
			}
		}
	}
	return string(rs), nil
}

func (tty *TTY) ReadString() (string, error) {
	defer tty.Output().WriteString("\n")
	return tty.readString(displayRune)
}

func (tty *TTY) ReadPassword() (string, error) {
	defer tty.Output().WriteString("\n")
	return tty.readString(displayMask)
}

func (tty *TTY) ReadPasswordNoEcho() (string, error) {
	defer tty.Output().WriteString("\n")
	return tty.readString(displayNone)
}

func (tty *TTY) ReadPasswordClear() (string, error) {
	s, err := tty.readString(displayMask)
	tty.Output().WriteString(
		strings.Repeat("\b", len(s)) +
			strings.Repeat(" ", len(s)) +
			strings.Repeat("\b", len(s)))
	return s, err
}

type WINSIZE struct {
	W int
	H int
}

func (tty *TTY) SIGWINCH() <-chan WINSIZE {
	return tty.sigwinch()
}
