package streams

import (
	"io"
	"os"

	"github.com/moby/term"
	"github.com/sirupsen/logrus"
)

// Out is an output stream used by the DockerCli to write normal program
// output.
type Out struct {
	commonStream
	out io.Writer
}

func (o *Out) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// SetRawTerminal sets raw mode on the input terminal
func (o *Out) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !o.commonStream.isTerminal {
		return nil
	}
	o.commonStream.state, err = term.SetRawTerminalOutput(o.commonStream.fd)
	return err
}

// GetTtySize returns the height and width in characters of the tty
func (o *Out) GetTtySize() (uint, uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(o.fd)
	if err != nil {
		logrus.Debugf("Error getting size: %s", err)
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOut returns a new Out object from a Writer
func NewOut(out io.Writer) *Out {
	fd, isTerminal := term.GetFdInfo(out)
	return &Out{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, out: out}
}
