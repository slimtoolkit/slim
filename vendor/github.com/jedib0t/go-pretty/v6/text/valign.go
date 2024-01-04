package text

import "strings"

// VAlign denotes how text is to be aligned vertically.
type VAlign int

// VAlign enumerations
const (
	VAlignDefault VAlign = iota // same as VAlignTop
	VAlignTop                   // "top\n\n"
	VAlignMiddle                // "\nmiddle\n"
	VAlignBottom                // "\n\nbottom"
)

// Apply aligns the lines vertically. For ex.:
//   - VAlignTop.Apply({"Game", "Of", "Thrones"},    5)
//     returns {"Game", "Of", "Thrones", "", ""}
//   - VAlignMiddle.Apply({"Game", "Of", "Thrones"}, 5)
//     returns {"", "Game", "Of", "Thrones", ""}
//   - VAlignBottom.Apply({"Game", "Of", "Thrones"}, 5)
//     returns {"", "", "Game", "Of", "Thrones"}
func (va VAlign) Apply(lines []string, maxLines int) []string {
	if len(lines) == maxLines {
		return lines
	} else if len(lines) > maxLines {
		maxLines = len(lines)
	}

	insertIdx := 0
	if va == VAlignMiddle {
		insertIdx = int(maxLines-len(lines)) / 2
	} else if va == VAlignBottom {
		insertIdx = maxLines - len(lines)
	}

	linesOut := strings.Split(strings.Repeat("\n", maxLines-1), "\n")
	for idx, line := range lines {
		linesOut[idx+insertIdx] = line
	}
	return linesOut
}

// ApplyStr aligns the string (of 1 or more lines) vertically. For ex.:
//   - VAlignTop.ApplyStr("Game\nOf\nThrones",    5)
//     returns {"Game", "Of", "Thrones", "", ""}
//   - VAlignMiddle.ApplyStr("Game\nOf\nThrones", 5)
//     returns {"", "Game", "Of", "Thrones", ""}
//   - VAlignBottom.ApplyStr("Game\nOf\nThrones", 5)
//     returns {"", "", "Game", "Of", "Thrones"}
func (va VAlign) ApplyStr(text string, maxLines int) []string {
	return va.Apply(strings.Split(text, "\n"), maxLines)
}

// HTMLProperty returns the equivalent HTML vertical-align tag property.
func (va VAlign) HTMLProperty() string {
	switch va {
	case VAlignTop:
		return "valign=\"top\""
	case VAlignMiddle:
		return "valign=\"middle\""
	case VAlignBottom:
		return "valign=\"bottom\""
	default:
		return ""
	}
}
