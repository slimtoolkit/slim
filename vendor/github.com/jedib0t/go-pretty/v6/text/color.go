package text

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var colorsEnabled = areANSICodesSupported()

// DisableColors (forcefully) disables color coding globally.
func DisableColors() {
	colorsEnabled = false
}

// EnableColors (forcefully) enables color coding globally.
func EnableColors() {
	colorsEnabled = true
}

// The logic here is inspired from github.com/fatih/color; the following is
// the bare minimum logic required to print Colored to the console.
// The differences:
// * This one caches the escape sequences for cases with multiple colors
// * This one handles cases where the incoming already has colors in the
//   form of escape sequences; in which case, text that does not have any
//   escape sequences are colored/escaped

// Color represents a single color to render with.
type Color int

// Base colors -- attributes in reality
const (
	Reset Color = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground colors
const (
	FgBlack Color = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Foreground Hi-Intensity colors
const (
	FgHiBlack Color = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background colors
const (
	BgBlack Color = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
)

// Background Hi-Intensity colors
const (
	BgHiBlack Color = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

// EscapeSeq returns the ANSI escape sequence for the color.
func (c Color) EscapeSeq() string {
	return EscapeStart + strconv.Itoa(int(c)) + EscapeStop
}

// HTMLProperty returns the "class" attribute for the color.
func (c Color) HTMLProperty() string {
	out := ""
	if class, ok := colorCSSClassMap[c]; ok {
		out = fmt.Sprintf("class=\"%s\"", class)
	}
	return out
}

// Sprint colorizes and prints the given string(s).
func (c Color) Sprint(a ...interface{}) string {
	return colorize(fmt.Sprint(a...), c.EscapeSeq())
}

// Sprintf formats and colorizes and prints the given string(s).
func (c Color) Sprintf(format string, a ...interface{}) string {
	return colorize(fmt.Sprintf(format, a...), c.EscapeSeq())
}

// Colors represents an array of Color objects to render with.
// Example: Colors{FgCyan, BgBlack}
type Colors []Color

// colorsSeqMap caches the escape sequence for a set of colors
var colorsSeqMap = sync.Map{}

// EscapeSeq returns the ANSI escape sequence for the colors set.
func (c Colors) EscapeSeq() string {
	if len(c) == 0 {
		return ""
	}

	colorsKey := fmt.Sprintf("%#v", c)
	escapeSeq, ok := colorsSeqMap.Load(colorsKey)
	if !ok || escapeSeq == "" {
		colorNums := make([]string, len(c))
		for idx, color := range c {
			colorNums[idx] = strconv.Itoa(int(color))
		}
		escapeSeq = EscapeStart + strings.Join(colorNums, ";") + EscapeStop
		colorsSeqMap.Store(colorsKey, escapeSeq)
	}
	return escapeSeq.(string)
}

// HTMLProperty returns the "class" attribute for the colors.
func (c Colors) HTMLProperty() string {
	if len(c) == 0 {
		return ""
	}

	var classes []string
	for _, color := range c {
		if class, ok := colorCSSClassMap[color]; ok {
			classes = append(classes, class)
		}
	}
	if len(classes) > 1 {
		sort.Strings(classes)
	}
	return fmt.Sprintf("class=\"%s\"", strings.Join(classes, " "))
}

// Sprint colorizes and prints the given string(s).
func (c Colors) Sprint(a ...interface{}) string {
	return colorize(fmt.Sprint(a...), c.EscapeSeq())
}

// Sprintf formats and colorizes and prints the given string(s).
func (c Colors) Sprintf(format string, a ...interface{}) string {
	return colorize(fmt.Sprintf(format, a...), c.EscapeSeq())
}

func colorize(s string, escapeSeq string) string {
	if !colorsEnabled || escapeSeq == "" {
		return s
	}
	return Escape(s, escapeSeq)
}
