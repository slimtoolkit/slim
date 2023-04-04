// +build go1.10

package xpath

import (
	"math"
	"strings"
)

func round(f float64) int {
	return int(math.Round(f))
}

func newStringBuilder() stringBuilder{ 
	return &strings.Builder{}
}
