package text

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// RuneWidth stuff
var (
	rwCondition = runewidth.NewCondition()
)

// InsertEveryN inserts the rune every N characters in the string. For ex.:
//
//	InsertEveryN("Ghost", '-', 1) == "G-h-o-s-t"
//	InsertEveryN("Ghost", '-', 2) == "Gh-os-t"
//	InsertEveryN("Ghost", '-', 3) == "Gho-st"
//	InsertEveryN("Ghost", '-', 4) == "Ghos-t"
//	InsertEveryN("Ghost", '-', 5) == "Ghost"
func InsertEveryN(str string, runeToInsert rune, n int) string {
	if n <= 0 {
		return str
	}

	sLen := RuneWidthWithoutEscSequences(str)
	var out strings.Builder
	out.Grow(sLen + (sLen / n))
	outLen, eSeq := 0, escSeq{}
	for idx, c := range str {
		if eSeq.isIn {
			eSeq.InspectRune(c)
			out.WriteRune(c)
			continue
		}
		eSeq.InspectRune(c)
		if !eSeq.isIn && outLen > 0 && (outLen%n) == 0 && idx != sLen {
			out.WriteRune(runeToInsert)
		}
		out.WriteRune(c)
		if !eSeq.isIn {
			outLen += RuneWidth(c)
		}
	}
	return out.String()
}

// LongestLineLen returns the length of the longest "line" within the
// argument string. For ex.:
//
//	LongestLineLen("Ghost!\nCome back here!\nRight now!") == 15
func LongestLineLen(str string) int {
	maxLength, currLength, eSeq := 0, 0, escSeq{}
	for _, c := range str {
		if eSeq.isIn {
			eSeq.InspectRune(c)
			continue
		}
		eSeq.InspectRune(c)
		if c == '\n' {
			if currLength > maxLength {
				maxLength = currLength
			}
			currLength = 0
		} else if !eSeq.isIn {
			currLength += RuneWidth(c)
		}
	}
	if currLength > maxLength {
		maxLength = currLength
	}
	return maxLength
}

// OverrideRuneWidthEastAsianWidth can *probably* help with alignment, and
// length calculation issues when dealing with Unicode character-set and a
// non-English language set in the LANG variable.
//
// Set this to 'false' to force the "runewidth" library to pretend to deal with
// English character-set. Be warned that if the text/content you are dealing
// with contains East Asian character-set, this may result in unexpected
// behavior.
//
// References:
// * https://github.com/mattn/go-runewidth/issues/64#issuecomment-1221642154
// * https://github.com/jedib0t/go-pretty/issues/220
// * https://github.com/jedib0t/go-pretty/issues/204
func OverrideRuneWidthEastAsianWidth(val bool) {
	rwCondition.EastAsianWidth = val
}

// Pad pads the given string with as many characters as needed to make it as
// long as specified (maxLen). This function does not count escape sequences
// while calculating length of the string. Ex.:
//
//	Pad("Ghost", 0, ' ') == "Ghost"
//	Pad("Ghost", 3, ' ') == "Ghost"
//	Pad("Ghost", 5, ' ') == "Ghost"
//	Pad("Ghost", 7, ' ') == "Ghost  "
//	Pad("Ghost", 10, '.') == "Ghost....."
func Pad(str string, maxLen int, paddingChar rune) string {
	strLen := RuneWidthWithoutEscSequences(str)
	if strLen < maxLen {
		str += strings.Repeat(string(paddingChar), maxLen-strLen)
	}
	return str
}

// RepeatAndTrim repeats the given string until it is as long as maxRunes.
// For ex.:
//
//	RepeatAndTrim("", 5) == ""
//	RepeatAndTrim("Ghost", 0) == ""
//	RepeatAndTrim("Ghost", 5) == "Ghost"
//	RepeatAndTrim("Ghost", 7) == "GhostGh"
//	RepeatAndTrim("Ghost", 10) == "GhostGhost"
func RepeatAndTrim(str string, maxRunes int) string {
	if str == "" || maxRunes == 0 {
		return ""
	} else if maxRunes == utf8.RuneCountInString(str) {
		return str
	}
	repeatedS := strings.Repeat(str, int(maxRunes/utf8.RuneCountInString(str))+1)
	return Trim(repeatedS, maxRunes)
}

// RuneCount is similar to utf8.RuneCountInString, except for the fact that it
// ignores escape sequences while counting. For ex.:
//
//	RuneCount("") == 0
//	RuneCount("Ghost") == 5
//	RuneCount("\x1b[33mGhost\x1b[0m") == 5
//	RuneCount("\x1b[33mGhost\x1b[0") == 5
//
// Deprecated: in favor of RuneWidthWithoutEscSequences
func RuneCount(str string) int {
	return RuneWidthWithoutEscSequences(str)
}

// RuneWidth returns the mostly accurate character-width of the rune. This is
// not 100% accurate as the character width is usually dependent on the
// typeface (font) used in the console/terminal. For ex.:
//
//	RuneWidth('A') == 1
//	RuneWidth('ツ') == 2
//	RuneWidth('⊙') == 1
//	RuneWidth('︿') == 2
//	RuneWidth(0x27) == 0
func RuneWidth(r rune) int {
	return rwCondition.RuneWidth(r)
}

// RuneWidthWithoutEscSequences is similar to RuneWidth, except for the fact
// that it ignores escape sequences while counting. For ex.:
//
//	RuneWidthWithoutEscSequences("") == 0
//	RuneWidthWithoutEscSequences("Ghost") == 5
//	RuneWidthWithoutEscSequences("\x1b[33mGhost\x1b[0m") == 5
//	RuneWidthWithoutEscSequences("\x1b[33mGhost\x1b[0") == 5
func RuneWidthWithoutEscSequences(str string) int {
	count, eSeq := 0, escSeq{}
	for _, c := range str {
		if eSeq.isIn {
			eSeq.InspectRune(c)
			continue
		}
		eSeq.InspectRune(c)
		if !eSeq.isIn {
			count += RuneWidth(c)
		}
	}
	return count
}

// Snip returns the given string with a fixed length. For ex.:
//
//	Snip("Ghost", 0, "~") == "Ghost"
//	Snip("Ghost", 1, "~") == "~"
//	Snip("Ghost", 3, "~") == "Gh~"
//	Snip("Ghost", 5, "~") == "Ghost"
//	Snip("Ghost", 7, "~") == "Ghost  "
//	Snip("\x1b[33mGhost\x1b[0m", 7, "~") == "\x1b[33mGhost\x1b[0m  "
func Snip(str string, length int, snipIndicator string) string {
	if length > 0 {
		lenStr := RuneWidthWithoutEscSequences(str)
		if lenStr > length {
			lenStrFinal := length - RuneWidthWithoutEscSequences(snipIndicator)
			return Trim(str, lenStrFinal) + snipIndicator
		}
	}
	return str
}

// Trim trims a string to the given length while ignoring escape sequences. For
// ex.:
//
//	Trim("Ghost", 3) == "Gho"
//	Trim("Ghost", 6) == "Ghost"
//	Trim("\x1b[33mGhost\x1b[0m", 3) == "\x1b[33mGho\x1b[0m"
//	Trim("\x1b[33mGhost\x1b[0m", 6) == "\x1b[33mGhost\x1b[0m"
func Trim(str string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	var out strings.Builder
	out.Grow(maxLen)

	outLen, eSeq := 0, escSeq{}
	for _, sChr := range str {
		if eSeq.isIn {
			eSeq.InspectRune(sChr)
			out.WriteRune(sChr)
			continue
		}
		eSeq.InspectRune(sChr)
		if eSeq.isIn {
			out.WriteRune(sChr)
			continue
		}
		if outLen < maxLen {
			outLen++
			out.WriteRune(sChr)
			continue
		}
	}
	return out.String()
}
