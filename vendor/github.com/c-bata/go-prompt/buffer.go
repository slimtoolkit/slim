package prompt

import (
	"log"
	"strings"
)

// Buffer emulates the console buffer.
type Buffer struct {
	workingLines    []string // The working lines. Similar to history
	workingIndex    int
	cursorPosition  int
	cacheDocument   *Document
	preferredColumn int // Remember the original column for the next up/down movement.
}

// Text returns string of the current line.
func (b *Buffer) Text() string {
	return b.workingLines[b.workingIndex]
}

// Document method to return document instance from the current text and cursor position.
func (b *Buffer) Document() (d *Document) {
	if b.cacheDocument == nil ||
		b.cacheDocument.Text != b.Text() ||
		b.cacheDocument.cursorPosition != b.cursorPosition {
		b.cacheDocument = &Document{
			Text:           b.Text(),
			cursorPosition: b.cursorPosition,
		}
	}
	return b.cacheDocument
}

// DisplayCursorPosition returns the cursor position on rendered text on terminal emulators.
// So if Document is "日本(cursor)語", DisplayedCursorPosition returns 4 because '日' and '本' are double width characters.
func (b *Buffer) DisplayCursorPosition() int {
	return b.Document().DisplayCursorPosition()
}

// InsertText insert string from current line.
func (b *Buffer) InsertText(v string, overwrite bool, moveCursor bool) {
	or := []rune(b.Text())
	oc := b.cursorPosition

	if overwrite {
		overwritten := string(or[oc : oc+len(v)])
		if strings.Contains(overwritten, "\n") {
			i := strings.IndexAny(overwritten, "\n")
			overwritten = overwritten[:i]
		}
		b.setText(string(or[:oc]) + v + string(or[oc+len(overwritten):]))
	} else {
		b.setText(string(or[:oc]) + v + string(or[oc:]))
	}

	if moveCursor {
		b.cursorPosition += len([]rune(v))
	}
}

// SetText method to set text and update cursorPosition.
// (When doing this, make sure that the cursor_position is valid for this text.
// text/cursor_position should be consistent at any time, otherwise set a Document instead.)
func (b *Buffer) setText(v string) {
	if b.cursorPosition > len([]rune(v)) {
		log.Print("[ERROR] The length of input value should be shorter than the position of cursor.")
	}
	o := b.workingLines[b.workingIndex]
	b.workingLines[b.workingIndex] = v

	if o != v {
		// Text is changed.
		// TODO: Call callback function triggered by text changed. And also history search text should reset.
		// https://github.com/jonathanslenders/python-prompt-toolkit/blob/master/prompt_toolkit/buffer.py#L380-L384
	}
}

// Set cursor position. Return whether it changed.
func (b *Buffer) setCursorPosition(p int) {
	o := b.cursorPosition
	if p > 0 {
		b.cursorPosition = p
	} else {
		b.cursorPosition = 0
	}
	if p != o {
		// Cursor position is changed.
		// TODO: Call a onCursorPositionChanged function.
	}
}

func (b *Buffer) setDocument(d *Document) {
	b.cacheDocument = d
	b.setCursorPosition(d.cursorPosition) // Call before setText because setText check the relation between cursorPosition and line length.
	b.setText(d.Text)
}

// CursorLeft move to left on the current line.
func (b *Buffer) CursorLeft(count int) {
	l := b.Document().GetCursorLeftPosition(count)
	b.cursorPosition += l
	return
}

// CursorRight move to right on the current line.
func (b *Buffer) CursorRight(count int) {
	l := b.Document().GetCursorRightPosition(count)
	b.cursorPosition += l
	return
}

// CursorUp move cursor to the previous line.
// (for multi-line edit).
func (b *Buffer) CursorUp(count int) {
	orig := b.preferredColumn
	if b.preferredColumn == -1 { // -1 means nil
		orig = b.Document().CursorPositionCol()
	}
	b.cursorPosition += b.Document().GetCursorUpPosition(count, orig)

	// Remember the original column for the next up/down movement.
	b.preferredColumn = orig
}

// CursorDown move cursor to the next line.
// (for multi-line edit).
func (b *Buffer) CursorDown(count int) {
	orig := b.preferredColumn
	if b.preferredColumn == -1 { // -1 means nil
		orig = b.Document().CursorPositionCol()
	}
	b.cursorPosition += b.Document().GetCursorDownPosition(count, orig)

	// Remember the original column for the next up/down movement.
	b.preferredColumn = orig
}

// DeleteBeforeCursor delete specified number of characters before cursor and return the deleted text.
func (b *Buffer) DeleteBeforeCursor(count int) (deleted string) {
	if count <= 0 {
		log.Print("[ERROR] The count argument on DeleteBeforeCursor should grater than 0.")
	}
	r := []rune(b.Text())

	if b.cursorPosition > 0 {
		start := b.cursorPosition - count
		if start < 0 {
			start = 0
		}
		deleted = string(r[start:b.cursorPosition])
		b.setDocument(&Document{
			Text:           string(r[:start]) + string(r[b.cursorPosition:]),
			cursorPosition: b.cursorPosition - len([]rune(deleted)),
		})
	}
	return
}

// NewLine means CR.
func (b *Buffer) NewLine(copyMargin bool) {
	if copyMargin {
		b.InsertText("\n"+b.Document().leadingWhitespaceInCurrentLine(), false, true)
	} else {
		b.InsertText("\n", false, true)
	}
}

// Delete specified number of characters and Return the deleted text.
func (b *Buffer) Delete(count int) (deleted string) {
	r := []rune(b.Text())
	if b.cursorPosition < len(r) {
		deleted = b.Document().TextAfterCursor()[:count]
		b.setText(string(r[:b.cursorPosition]) + string(r[b.cursorPosition+len(deleted):]))
	}
	return
}

// JoinNextLine joins the next line to the current one by deleting the line ending after the current line.
func (b *Buffer) JoinNextLine(separator string) {
	if !b.Document().OnLastLine() {
		b.cursorPosition += b.Document().GetEndOfLinePosition()
		b.Delete(1)
		// Remove spaces
		b.setText(b.Document().TextBeforeCursor() + separator + strings.TrimLeft(b.Document().TextAfterCursor(), " "))
	}
}

// SwapCharactersBeforeCursor swaps the last two characters before the cursor.
func (b *Buffer) SwapCharactersBeforeCursor() {
	if b.cursorPosition >= 2 {
		x := b.Text()[b.cursorPosition-2 : b.cursorPosition-1]
		y := b.Text()[b.cursorPosition-1 : b.cursorPosition]
		b.setText(b.Text()[:b.cursorPosition-2] + y + x + b.Text()[b.cursorPosition:])
	}
}

// NewBuffer is constructor of Buffer struct.
func NewBuffer() (b *Buffer) {
	b = &Buffer{
		workingLines:    []string{""},
		workingIndex:    0,
		preferredColumn: -1, // -1 means nil
	}
	return
}
