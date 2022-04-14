package logutil

import (
	"bytes"
	"io"
	"sync"

	"github.com/sirupsen/logrus"
)

func Pause(l *logrus.Logger) func() {
	// initialize formatter with original terminal settings
	l.Formatter.Format(logrus.NewEntry(l))

	bw := newBufferedWriter(l.Out)
	l.SetOutput(bw)
	return func() {
		bw.resume()
	}
}

type bufferedWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
	w   io.Writer
}

func newBufferedWriter(w io.Writer) *bufferedWriter {
	return &bufferedWriter{
		buf: bytes.NewBuffer(nil),
		w:   w,
	}
}

func (bw *bufferedWriter) Write(p []byte) (int, error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if bw.buf == nil {
		return bw.w.Write(p)
	}
	return bw.buf.Write(p)
}

func (bw *bufferedWriter) resume() {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if bw.buf == nil {
		return
	}
	io.Copy(bw.w, bw.buf)
	bw.buf = nil
}
