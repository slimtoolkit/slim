// +build !windows

package uilive

import (
	"fmt"
	"strings"
)

// clear the line and move the cursor up
var clear = fmt.Sprintf("%c[%dA%c[2K", ESC, 1, ESC)

func (w *Writer) clearLines() {
	_, _ = fmt.Fprint(w.Out, strings.Repeat(clear, w.lineCount))
}
