package prompt

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// Document has text displayed in terminal and cursor position.
type Document struct {
	Text string
	// This represents a index in a rune array of Document.Text.
	// So if Document is "日本(cursor)語", cursorPosition is 2.
	// But DisplayedCursorPosition returns 4 because '日' and '本' are double width characters.
	cursorPosition int
}

// NewDocument return the new empty document.
func NewDocument() *Document {
	return &Document{
		Text:           "",
		cursorPosition: 0,
	}
}

// DisplayCursorPosition returns the cursor position on rendered text on terminal emulators.
// So if Document is "日本(cursor)語", DisplayedCursorPosition returns 4 because '日' and '本' are double width characters.
func (d *Document) DisplayCursorPosition() int {
	var position int
	runes := []rune(d.Text)[:d.cursorPosition]
	for i := range runes {
		position += runewidth.RuneWidth(runes[i])
	}
	return position
}

// GetCharRelativeToCursor return character relative to cursor position, or empty string
func (d *Document) GetCharRelativeToCursor(offset int) (r rune) {
	s := d.Text
	cnt := 0

	for len(s) > 0 {
		cnt++
		r, size := utf8.DecodeRuneInString(s)
		if cnt == d.cursorPosition+offset {
			return r
		}
		s = s[size:]
	}
	return 0
}

// TextBeforeCursor returns the text before the cursor.
func (d *Document) TextBeforeCursor() string {
	r := []rune(d.Text)
	return string(r[:d.cursorPosition])
}

// TextAfterCursor returns the text after the cursor.
func (d *Document) TextAfterCursor() string {
	r := []rune(d.Text)
	return string(r[d.cursorPosition:])
}

// GetWordBeforeCursor returns the word before the cursor.
// If we have whitespace before the cursor this returns an empty string.
func (d *Document) GetWordBeforeCursor() string {
	x := d.TextBeforeCursor()
	return x[d.FindStartOfPreviousWord():]
}

// GetWordAfterCursor returns the word after the cursor.
// If we have whitespace after the cursor this returns an empty string.
func (d *Document) GetWordAfterCursor() string {
	x := d.TextAfterCursor()
	return x[:d.FindEndOfCurrentWord()]
}

// GetWordBeforeCursorWithSpace returns the word before the cursor.
// Unlike GetWordBeforeCursor, it returns string containing space
func (d *Document) GetWordBeforeCursorWithSpace() string {
	x := d.TextBeforeCursor()
	return x[d.FindStartOfPreviousWordWithSpace():]
}

// GetWordAfterCursorWithSpace returns the word after the cursor.
// Unlike GetWordAfterCursor, it returns string containing space
func (d *Document) GetWordAfterCursorWithSpace() string {
	x := d.TextAfterCursor()
	return x[:d.FindEndOfCurrentWordWithSpace()]
}

// GetWordBeforeCursorUntilSeparator returns the text before the cursor until next separator.
func (d *Document) GetWordBeforeCursorUntilSeparator(sep string) string {
	x := d.TextBeforeCursor()
	return x[d.FindStartOfPreviousWordUntilSeparator(sep):]
}

// GetWordAfterCursorUntilSeparator returns the text after the cursor until next separator.
func (d *Document) GetWordAfterCursorUntilSeparator(sep string) string {
	x := d.TextAfterCursor()
	return x[:d.FindEndOfCurrentWordUntilSeparator(sep)]
}

// GetWordBeforeCursorUntilSeparatorIgnoreNextToCursor returns the word before the cursor.
// Unlike GetWordBeforeCursor, it returns string containing space
func (d *Document) GetWordBeforeCursorUntilSeparatorIgnoreNextToCursor(sep string) string {
	x := d.TextBeforeCursor()
	return x[d.FindStartOfPreviousWordUntilSeparatorIgnoreNextToCursor(sep):]
}

// GetWordAfterCursorUntilSeparatorIgnoreNextToCursor returns the word after the cursor.
// Unlike GetWordAfterCursor, it returns string containing space
func (d *Document) GetWordAfterCursorUntilSeparatorIgnoreNextToCursor(sep string) string {
	x := d.TextAfterCursor()
	return x[:d.FindEndOfCurrentWordUntilSeparatorIgnoreNextToCursor(sep)]
}

// FindStartOfPreviousWord returns an index relative to the cursor position
// pointing to the start of the previous word. Return 0 if nothing was found.
func (d *Document) FindStartOfPreviousWord() int {
	x := d.TextBeforeCursor()
	i := strings.LastIndexByte(x, ' ')
	if i != -1 {
		return i + 1
	}
	return 0
}

// FindStartOfPreviousWordWithSpace is almost the same as FindStartOfPreviousWord.
// The only difference is to ignore contiguous spaces.
func (d *Document) FindStartOfPreviousWordWithSpace() int {
	x := d.TextBeforeCursor()
	end := lastIndexByteNot(x, ' ')
	if end == -1 {
		return 0
	}

	start := strings.LastIndexByte(x[:end], ' ')
	if start == -1 {
		return 0
	}
	return start + 1
}

// FindStartOfPreviousWordUntilSeparator is almost the same as FindStartOfPreviousWord.
// But this can specify Separator. Return 0 if nothing was found.
func (d *Document) FindStartOfPreviousWordUntilSeparator(sep string) int {
	if sep == "" {
		return d.FindStartOfPreviousWord()
	}

	x := d.TextBeforeCursor()
	i := strings.LastIndexAny(x, sep)
	if i != -1 {
		return i + 1
	}
	return 0
}

// FindStartOfPreviousWordUntilSeparatorIgnoreNextToCursor is almost the same as FindStartOfPreviousWordWithSpace.
// But this can specify Separator. Return 0 if nothing was found.
func (d *Document) FindStartOfPreviousWordUntilSeparatorIgnoreNextToCursor(sep string) int {
	if sep == "" {
		return d.FindStartOfPreviousWordWithSpace()
	}

	x := d.TextBeforeCursor()
	end := lastIndexAnyNot(x, sep)
	if end == -1 {
		return 0
	}
	start := strings.LastIndexAny(x[:end], sep)
	if start == -1 {
		return 0
	}
	return start + 1
}

// FindEndOfCurrentWord returns an index relative to the cursor position.
// pointing to the end of the current word. Return 0 if nothing was found.
func (d *Document) FindEndOfCurrentWord() int {
	x := d.TextAfterCursor()
	i := strings.IndexByte(x, ' ')
	if i != -1 {
		return i
	}
	return len(x)
}

// FindEndOfCurrentWordWithSpace is almost the same as FindEndOfCurrentWord.
// The only difference is to ignore contiguous spaces.
func (d *Document) FindEndOfCurrentWordWithSpace() int {
	x := d.TextAfterCursor()

	start := indexByteNot(x, ' ')
	if start == -1 {
		return len(x)
	}

	end := strings.IndexByte(x[start:], ' ')
	if end == -1 {
		return len(x)
	}

	return start + end
}

// FindEndOfCurrentWordUntilSeparator is almost the same as FindEndOfCurrentWord.
// But this can specify Separator. Return 0 if nothing was found.
func (d *Document) FindEndOfCurrentWordUntilSeparator(sep string) int {
	if sep == "" {
		return d.FindEndOfCurrentWord()
	}

	x := d.TextAfterCursor()
	i := strings.IndexAny(x, sep)
	if i != -1 {
		return i
	}
	return len(x)
}

// FindEndOfCurrentWordUntilSeparatorIgnoreNextToCursor is almost the same as FindEndOfCurrentWordWithSpace.
// But this can specify Separator. Return 0 if nothing was found.
func (d *Document) FindEndOfCurrentWordUntilSeparatorIgnoreNextToCursor(sep string) int {
	if sep == "" {
		return d.FindEndOfCurrentWordWithSpace()
	}

	x := d.TextAfterCursor()

	start := indexAnyNot(x, sep)
	if start == -1 {
		return len(x)
	}

	end := strings.IndexAny(x[start:], sep)
	if end == -1 {
		return len(x)
	}

	return start + end
}

// CurrentLineBeforeCursor returns the text from the start of the line until the cursor.
func (d *Document) CurrentLineBeforeCursor() string {
	s := strings.Split(d.TextBeforeCursor(), "\n")
	return s[len(s)-1]
}

// CurrentLineAfterCursor returns the text from the cursor until the end of the line.
func (d *Document) CurrentLineAfterCursor() string {
	return strings.Split(d.TextAfterCursor(), "\n")[0]
}

// CurrentLine return the text on the line where the cursor is. (when the input
// consists of just one line, it equals `text`.
func (d *Document) CurrentLine() string {
	return d.CurrentLineBeforeCursor() + d.CurrentLineAfterCursor()
}

// Array pointing to the start indexes of all the lines.
func (d *Document) lineStartIndexes() []int {
	// TODO: Cache, because this is often reused.
	// (If it is used, it's often used many times.
	// And this has to be fast for editing big documents!)
	lc := d.LineCount()
	lengths := make([]int, lc)
	for i, l := range d.Lines() {
		lengths[i] = len(l)
	}

	// Calculate cumulative sums.
	indexes := make([]int, lc+1)
	indexes[0] = 0 // https://github.com/jonathanslenders/python-prompt-toolkit/blob/master/prompt_toolkit/document.py#L189
	pos := 0
	for i, l := range lengths {
		pos += l + 1
		indexes[i+1] = pos
	}
	if lc > 1 {
		// Pop the last item. (This is not a new line.)
		indexes = indexes[:lc]
	}
	return indexes
}

// For the index of a character at a certain line, calculate the index of
// the first character on that line.
func (d *Document) findLineStartIndex(index int) (pos int, lineStartIndex int) {
	indexes := d.lineStartIndexes()
	pos = bisectRight(indexes, index) - 1
	lineStartIndex = indexes[pos]
	return
}

// CursorPositionRow returns the current row. (0-based.)
func (d *Document) CursorPositionRow() (row int) {
	row, _ = d.findLineStartIndex(d.cursorPosition)
	return
}

// CursorPositionCol returns the current column. (0-based.)
func (d *Document) CursorPositionCol() (col int) {
	// Don't use self.text_before_cursor to calculate this. Creating substrings
	// and splitting is too expensive for getting the cursor position.
	_, index := d.findLineStartIndex(d.cursorPosition)
	col = d.cursorPosition - index
	return
}

// GetCursorLeftPosition returns the relative position for cursor left.
func (d *Document) GetCursorLeftPosition(count int) int {
	if count < 0 {
		return d.GetCursorRightPosition(-count)
	}
	if d.CursorPositionCol() > count {
		return -count
	}
	return -d.CursorPositionCol()
}

// GetCursorRightPosition returns relative position for cursor right.
func (d *Document) GetCursorRightPosition(count int) int {
	if count < 0 {
		return d.GetCursorLeftPosition(-count)
	}
	if len(d.CurrentLineAfterCursor()) > count {
		return count
	}
	return len(d.CurrentLineAfterCursor())
}

// GetCursorUpPosition return the relative cursor position (character index) where we would be
// if the user pressed the arrow-up button.
func (d *Document) GetCursorUpPosition(count int, preferredColumn int) int {
	var col int
	if preferredColumn == -1 { // -1 means nil
		col = d.CursorPositionCol()
	} else {
		col = preferredColumn
	}

	row := d.CursorPositionRow() - count
	if row < 0 {
		row = 0
	}
	return d.TranslateRowColToIndex(row, col) - d.cursorPosition
}

// GetCursorDownPosition return the relative cursor position (character index) where we would be if the
// user pressed the arrow-down button.
func (d *Document) GetCursorDownPosition(count int, preferredColumn int) int {
	var col int
	if preferredColumn == -1 { // -1 means nil
		col = d.CursorPositionCol()
	} else {
		col = preferredColumn
	}
	row := d.CursorPositionRow() + count
	return d.TranslateRowColToIndex(row, col) - d.cursorPosition
}

// Lines returns the array of all the lines.
func (d *Document) Lines() []string {
	// TODO: Cache, because this one is reused very often.
	return strings.Split(d.Text, "\n")
}

// LineCount return the number of lines in this document. If the document ends
// with a trailing \n, that counts as the beginning of a new line.
func (d *Document) LineCount() int {
	return len(d.Lines())
}

// TranslateIndexToPosition given an index for the text, return the corresponding (row, col) tuple.
// (0-based. Returns (0, 0) for index=0.)
func (d *Document) TranslateIndexToPosition(index int) (row int, col int) {
	row, rowIndex := d.findLineStartIndex(index)
	col = index - rowIndex
	return
}

// TranslateRowColToIndex given a (row, col), return the corresponding index.
// (Row and col params are 0-based.)
func (d *Document) TranslateRowColToIndex(row int, column int) (index int) {
	indexes := d.lineStartIndexes()
	if row < 0 {
		row = 0
	} else if row > len(indexes) {
		row = len(indexes) - 1
	}
	index = indexes[row]
	line := d.Lines()[row]

	// python) result += max(0, min(col, len(line)))
	if column > 0 || len(line) > 0 {
		if column > len(line) {
			index += len(line)
		} else {
			index += column
		}
	}

	// Keep in range. (len(self.text) is included, because the cursor can be
	// right after the end of the text as well.)
	// python) result = max(0, min(result, len(self.text)))
	if index > len(d.Text) {
		index = len(d.Text)
	}
	if index < 0 {
		index = 0
	}
	return index
}

// OnLastLine returns true when we are at the last line.
func (d *Document) OnLastLine() bool {
	return d.CursorPositionRow() == (d.LineCount() - 1)
}

// GetEndOfLinePosition returns relative position for the end of this line.
func (d *Document) GetEndOfLinePosition() int {
	return len([]rune(d.CurrentLineAfterCursor()))
}

func (d *Document) leadingWhitespaceInCurrentLine() (margin string) {
	trimmed := strings.TrimSpace(d.CurrentLine())
	margin = d.CurrentLine()[:len(d.CurrentLine())-len(trimmed)]
	return
}

// bisectRight to Locate the insertion point for v in a to maintain sorted order.
func bisectRight(a []int, v int) int {
	return bisectRightRange(a, v, 0, len(a))
}

func bisectRightRange(a []int, v int, lo, hi int) int {
	s := a[lo:hi]
	return sort.Search(len(s), func(i int) bool {
		return s[i] > v
	})
}

func indexByteNot(s string, c byte) int {
	n := len(s)
	for i := 0; i < n; i++ {
		if s[i] != c {
			return i
		}
	}
	return -1
}

func lastIndexByteNot(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != c {
			return i
		}
	}
	return -1
}

type asciiSet [8]uint32

func (as *asciiSet) notContains(c byte) bool {
	return (as[c>>5] & (1 << uint(c&31))) == 0
}

func makeASCIISet(chars string) (as asciiSet, ok bool) {
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c >= utf8.RuneSelf {
			return as, false
		}
		as[c>>5] |= 1 << uint(c&31)
	}
	return as, true
}

func indexAnyNot(s, chars string) int {
	if len(chars) > 0 {
		if len(s) > 8 {
			if as, isASCII := makeASCIISet(chars); isASCII {
				for i := 0; i < len(s); i++ {
					if as.notContains(s[i]) {
						return i
					}
				}
				return -1
			}
		}
		for i := 0; i < len(s); {
			// I don't know why strings.IndexAny doesn't add rune count here.
			r, size := utf8.DecodeRuneInString(s[i:])
			i += size
			for _, c := range chars {
				if r != c {
					return i
				}
			}
		}
	}
	return -1
}

func lastIndexAnyNot(s, chars string) int {
	if len(chars) > 0 {
		if len(s) > 8 {
			if as, isASCII := makeASCIISet(chars); isASCII {
				for i := len(s) - 1; i >= 0; i-- {
					if as.notContains(s[i]) {
						return i
					}
				}
				return -1
			}
		}
		for i := len(s); i > 0; {
			r, size := utf8.DecodeLastRuneInString(s[:i])
			i -= size
			for _, c := range chars {
				if r != c {
					return i
				}
			}
		}
	}
	return -1
}
