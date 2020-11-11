package printbuffer

import (
	"fmt"
	"strings"
)

type PrintBuffer struct {
	Prefix string
}

func (b *PrintBuffer) Write(p []byte) (n int, err error) {
	s := strings.TrimRight(string(p), "\n")
	for _, line := range strings.Split(s, "\n") {
		if len(line) > 0 {
			n, err := fmt.Println(b.Prefix, line)
			if err != nil {
				panic(err)
			}
			if n != len(b.Prefix + line) + 2 {
				panic(fmt.Sprintf("printing failed %d %d", n, len(b.Prefix + line)))
			}
		}
	}
	return len(p), nil
}
