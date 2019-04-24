package uilive

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

// ESC is the ASCII code for escape character
const ESC = 27

// RefreshInterval is the default refresh interval to update the ui
var RefreshInterval = time.Millisecond

var overFlowHandled bool

var termWidth int

// Out is the default output writer for the Writer
var Out = os.Stdout

// ErrClosedPipe is the error returned when trying to writer is not listening
var ErrClosedPipe = errors.New("uilive: read/write on closed pipe")

// FdWriter is a writer with a file descriptor.
type FdWriter interface {
	io.Writer
	Fd() uintptr
}

// Writer is a buffered the writer that updates the terminal. The contents of writer will be flushed on a timed interval or when Flush is called.
type Writer struct {
	// Out is the writer to write to
	Out io.Writer

	// RefreshInterval is the time the UI sould refresh
	RefreshInterval time.Duration

	ticker *time.Ticker
	tdone  chan bool

	buf       bytes.Buffer
	mtx       *sync.Mutex
	lineCount int
}

type bypass struct {
	writer *Writer
}

type newline struct {
	writer *Writer
}

// New returns a new Writer with defaults
func New() *Writer {
	termWidth, _ = getTermSize()
	if termWidth != 0 {
		overFlowHandled = true
	}

	return &Writer{
		Out:             Out,
		RefreshInterval: RefreshInterval,

		mtx: &sync.Mutex{},
	}
}

// Flush writes to the out and resets the buffer. It should be called after the last call to Write to ensure that any data buffered in the Writer is written to output.
// Any incomplete escape sequence at the end is considered complete for formatting purposes.
// An error is returned if the contents of the buffer cannot be written to the underlying output stream
func (w *Writer) Flush() error {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	// do nothing if buffer is empty
	if len(w.buf.Bytes()) == 0 {
		return nil
	}
	w.clearLines()

	lines := 0
	var currentLine bytes.Buffer
	for _, b := range w.buf.Bytes() {
		if b == '\n' {
			lines++
			currentLine.Reset()
		} else {
			currentLine.Write([]byte{b})
			if overFlowHandled && currentLine.Len() > termWidth {
				lines++
				currentLine.Reset()
			}
		}
	}
	w.lineCount = lines
	_, err := w.Out.Write(w.buf.Bytes())
	w.buf.Reset()
	return err
}

// Start starts the listener in a non-blocking manner
func (w *Writer) Start() {
	if w.ticker == nil {
		w.ticker = time.NewTicker(w.RefreshInterval)
		w.tdone = make(chan bool, 1)
	}

	go w.Listen()
}

// Stop stops the listener that updates the terminal
func (w *Writer) Stop() {
	w.Flush()
	close(w.tdone)
}

// Listen listens for updates to the writer's buffer and flushes to the out provided. It blocks the runtime.
func (w *Writer) Listen() {
	for {
		select {
		case <-w.ticker.C:
			if w.ticker != nil {
				w.Flush()
			}
		case <-w.tdone:
			w.mtx.Lock()
			w.ticker.Stop()
			w.ticker = nil
			w.mtx.Unlock()
			return
		}
	}
}

// Write save the contents of buf to the writer b. The only errors returned are ones encountered while writing to the underlying buffer.
func (w *Writer) Write(buf []byte) (n int, err error) {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	return w.buf.Write(buf)
}

// Bypass creates an io.Writer which allows non-buffered output to be written to the underlying output
func (w *Writer) Bypass() io.Writer {
	return &bypass{writer: w}
}

func (b *bypass) Write(p []byte) (int, error) {
	b.writer.mtx.Lock()
	defer b.writer.mtx.Unlock()

	b.writer.clearLines()
	b.writer.lineCount = 0
	return b.writer.Out.Write(p)
}

// Newline creates an io.Writer which allows buffered output to be written to the underlying output. This enable writing
// to multiple lines at once.
func (w *Writer) Newline() io.Writer {
	return &newline{writer: w}
}

func (n *newline) Write(p []byte) (int, error) {
	n.writer.mtx.Lock()
	defer n.writer.mtx.Unlock()
	return n.writer.buf.Write(p)
}