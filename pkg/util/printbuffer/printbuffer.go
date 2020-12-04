package printbuffer

import (
	"bytes"
	"fmt"
)

type PrintBuffer struct {
	Prefix string
}

func (b *PrintBuffer) Write(p []byte) (n int, err error) {
	for _, line := range bytes.Split(bytes.TrimRight(p, "\n"), []byte{'\n'}) {
		if len(line) > 0 {
			fmt.Println(b.Prefix, string(line))
		}
	}
	return len(p), nil
}
