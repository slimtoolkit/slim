package tty

import (
	"bufio"
	"errors"
	"os"
	"syscall"
)

type TTY struct {
	in  *os.File
	bin *bufio.Reader
	out *os.File
}

func open(path string) (*TTY, error) {
	tty := new(TTY)

	in, err := os.Open("/dev/cons")
	if err != nil {
		return nil, err
	}
	tty.in = in
	tty.bin = bufio.NewReader(in)

	out, err := os.OpenFile("/dev/cons", syscall.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	tty.out = out

	return tty, nil
}

func (tty *TTY) buffered() bool {
	return tty.bin.Buffered() > 0
}

func (tty *TTY) readRune() (rune, error) {
	r, _, err := tty.bin.ReadRune()
	return r, err
}

func (tty *TTY) close() (err error) {
	if err2 := tty.in.Close(); err2 != nil {
		err = err2
	}
	if err2 := tty.out.Close(); err2 != nil {
		err = err2
	}
	return
}

func (tty *TTY) size() (int, int, error) {
	return 80, 24, nil
}

func (tty *TTY) sizePixel() (int, int, int, int, error) {
	x, y, _ := tty.size()
	return x, y, -1, -1, errors.New("no implemented method for querying size in pixels on Plan 9")
}

func (tty *TTY) input() *os.File {
	return tty.in
}

func (tty *TTY) output() *os.File {
	return tty.out
}
