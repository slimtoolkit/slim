package text

import "strings"

// ANSICodesSupported will be true on consoles where ANSI Escape Codes/Sequences
// are supported.
var ANSICodesSupported = areANSICodesSupported()

// Escape encodes the string with the ANSI Escape Sequence.
// For ex.:
//
//	Escape("Ghost", "") == "Ghost"
//	Escape("Ghost", "\x1b[91m") == "\x1b[91mGhost\x1b[0m"
//	Escape("\x1b[94mGhost\x1b[0mLady", "\x1b[91m") == "\x1b[94mGhost\x1b[0m\x1b[91mLady\x1b[0m"
//	Escape("Nymeria\x1b[94mGhost\x1b[0mLady", "\x1b[91m") == "\x1b[91mNymeria\x1b[94mGhost\x1b[0m\x1b[91mLady\x1b[0m"
//	Escape("Nymeria \x1b[94mGhost\x1b[0m Lady", "\x1b[91m") == "\x1b[91mNymeria \x1b[94mGhost\x1b[0m\x1b[91m Lady\x1b[0m"
func Escape(str string, escapeSeq string) string {
	out := ""
	if !strings.HasPrefix(str, EscapeStart) {
		out += escapeSeq
	}
	out += strings.Replace(str, EscapeReset, EscapeReset+escapeSeq, -1)
	if !strings.HasSuffix(out, EscapeReset) {
		out += EscapeReset
	}
	if strings.Contains(out, escapeSeq+EscapeReset) {
		out = strings.Replace(out, escapeSeq+EscapeReset, "", -1)
	}
	return out
}

// StripEscape strips all ANSI Escape Sequence from the string.
// For ex.:
//
//	StripEscape("Ghost") == "Ghost"
//	StripEscape("\x1b[91mGhost\x1b[0m") == "Ghost"
//	StripEscape("\x1b[94mGhost\x1b[0m\x1b[91mLady\x1b[0m") == "GhostLady"
//	StripEscape("\x1b[91mNymeria\x1b[94mGhost\x1b[0m\x1b[91mLady\x1b[0m") == "NymeriaGhostLady"
//	StripEscape("\x1b[91mNymeria \x1b[94mGhost\x1b[0m\x1b[91m Lady\x1b[0m") == "Nymeria Ghost Lady"
func StripEscape(str string) string {
	var out strings.Builder
	out.Grow(RuneWidthWithoutEscSequences(str))

	isEscSeq := false
	for _, sChr := range str {
		if sChr == EscapeStartRune {
			isEscSeq = true
		}
		if !isEscSeq {
			out.WriteRune(sChr)
		}
		if isEscSeq && sChr == EscapeStopRune {
			isEscSeq = false
		}
	}
	return out.String()
}
