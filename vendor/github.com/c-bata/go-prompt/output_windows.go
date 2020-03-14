// +build windows

package prompt

import (
	"io"

	"github.com/mattn/go-colorable"
)

// WindowsWriter is a ConsoleWriter implementation for Win32 console.
// Output is converted from VT100 escape sequences by mattn/go-colorable.
type WindowsWriter struct {
	VT100Writer
	out io.Writer
}

// Flush to flush buffer
func (w *WindowsWriter) Flush() error {
	_, err := w.out.Write(w.buffer)
	if err != nil {
		return err
	}
	w.buffer = []byte{}
	return nil
}

var _ ConsoleWriter = &WindowsWriter{}

var (
	// Deprecated: Please use NewStdoutWriter
	NewStandardOutputWriter = NewStdoutWriter
)

// NewStdoutWriter returns ConsoleWriter object to write to stdout.
// This generates win32 control sequences.
func NewStdoutWriter() ConsoleWriter {
	return &WindowsWriter{
		out: colorable.NewColorableStdout(),
	}
}

// NewStderrWriter returns ConsoleWriter object to write to stderr.
// This generates win32 control sequences.
func NewStderrWriter() ConsoleWriter {
	return &WindowsWriter{
		out: colorable.NewColorableStderr(),
	}
}
