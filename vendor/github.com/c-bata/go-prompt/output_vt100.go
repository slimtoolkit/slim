package prompt

import (
	"bytes"
	"strconv"
)

// VT100Writer generates VT100 escape sequences.
type VT100Writer struct {
	buffer []byte
}

// WriteRaw to write raw byte array
func (w *VT100Writer) WriteRaw(data []byte) {
	w.buffer = append(w.buffer, data...)
	return
}

// Write to write safety byte array by removing control sequences.
func (w *VT100Writer) Write(data []byte) {
	w.WriteRaw(bytes.Replace(data, []byte{0x1b}, []byte{'?'}, -1))
	return
}

// WriteRawStr to write raw string
func (w *VT100Writer) WriteRawStr(data string) {
	w.WriteRaw([]byte(data))
	return
}

// WriteStr to write safety string by removing control sequences.
func (w *VT100Writer) WriteStr(data string) {
	w.Write([]byte(data))
	return
}

/* Erase */

// EraseScreen erases the screen with the background colour and moves the cursor to home.
func (w *VT100Writer) EraseScreen() {
	w.WriteRaw([]byte{0x1b, '[', '2', 'J'})
	return
}

// EraseUp erases the screen from the current line up to the top of the screen.
func (w *VT100Writer) EraseUp() {
	w.WriteRaw([]byte{0x1b, '[', '1', 'J'})
	return
}

// EraseDown erases the screen from the current line down to the bottom of the screen.
func (w *VT100Writer) EraseDown() {
	w.WriteRaw([]byte{0x1b, '[', 'J'})
	return
}

// EraseStartOfLine erases from the current cursor position to the start of the current line.
func (w *VT100Writer) EraseStartOfLine() {
	w.WriteRaw([]byte{0x1b, '[', '1', 'K'})
	return
}

// EraseEndOfLine erases from the current cursor position to the end of the current line.
func (w *VT100Writer) EraseEndOfLine() {
	w.WriteRaw([]byte{0x1b, '[', 'K'})
	return
}

// EraseLine erases the entire current line.
func (w *VT100Writer) EraseLine() {
	w.WriteRaw([]byte{0x1b, '[', '2', 'K'})
	return
}

/* Cursor */

// ShowCursor stops blinking cursor and show.
func (w *VT100Writer) ShowCursor() {
	w.WriteRaw([]byte{0x1b, '[', '?', '1', '2', 'l', 0x1b, '[', '?', '2', '5', 'h'})
}

// HideCursor hides cursor.
func (w *VT100Writer) HideCursor() {
	w.WriteRaw([]byte{0x1b, '[', '?', '2', '5', 'l'})
	return
}

// CursorGoTo sets the cursor position where subsequent text will begin.
func (w *VT100Writer) CursorGoTo(row, col int) {
	if row == 0 && col == 0 {
		// If no row/column parameters are provided (ie. <ESC>[H), the cursor will move to the home position.
		w.WriteRaw([]byte{0x1b, '[', 'H'})
		return
	}
	r := strconv.Itoa(row)
	c := strconv.Itoa(col)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(r))
	w.WriteRaw([]byte{';'})
	w.WriteRaw([]byte(c))
	w.WriteRaw([]byte{'H'})
	return
}

// CursorUp moves the cursor up by 'n' rows; the default count is 1.
func (w *VT100Writer) CursorUp(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorDown(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'A'})
	return
}

// CursorDown moves the cursor down by 'n' rows; the default count is 1.
func (w *VT100Writer) CursorDown(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorUp(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'B'})
	return
}

// CursorForward moves the cursor forward by 'n' columns; the default count is 1.
func (w *VT100Writer) CursorForward(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorBackward(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'C'})
	return
}

// CursorBackward moves the cursor backward by 'n' columns; the default count is 1.
func (w *VT100Writer) CursorBackward(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorForward(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'D'})
	return
}

// AskForCPR asks for a cursor position report (CPR).
func (w *VT100Writer) AskForCPR() {
	// CPR: Cursor Position Request.
	w.WriteRaw([]byte{0x1b, '[', '6', 'n'})
	return
}

// SaveCursor saves current cursor position.
func (w *VT100Writer) SaveCursor() {
	w.WriteRaw([]byte{0x1b, '[', 's'})
	return
}

// UnSaveCursor restores cursor position after a Save Cursor.
func (w *VT100Writer) UnSaveCursor() {
	w.WriteRaw([]byte{0x1b, '[', 'u'})
	return
}

/* Scrolling */

// ScrollDown scrolls display down one line.
func (w *VT100Writer) ScrollDown() {
	w.WriteRaw([]byte{0x1b, 'D'})
	return
}

// ScrollUp scroll display up one line.
func (w *VT100Writer) ScrollUp() {
	w.WriteRaw([]byte{0x1b, 'M'})
	return
}

/* Title */

// SetTitle sets a title of terminal window.
func (w *VT100Writer) SetTitle(title string) {
	titleBytes := []byte(title)
	patterns := []struct {
		from []byte
		to   []byte
	}{
		{
			from: []byte{0x13},
			to:   []byte{},
		},
		{
			from: []byte{0x07},
			to:   []byte{},
		},
	}
	for i := range patterns {
		titleBytes = bytes.Replace(titleBytes, patterns[i].from, patterns[i].to, -1)
	}

	w.WriteRaw([]byte{0x1b, ']', '2', ';'})
	w.WriteRaw(titleBytes)
	w.WriteRaw([]byte{0x07})
	return
}

// ClearTitle clears a title of terminal window.
func (w *VT100Writer) ClearTitle() {
	w.WriteRaw([]byte{0x1b, ']', '2', ';', 0x07})
	return
}

/* Font */

// SetColor sets text and background colors. and specify whether text is bold.
func (w *VT100Writer) SetColor(fg, bg Color, bold bool) {
	if bold {
		w.SetDisplayAttributes(fg, bg, DisplayBold)
	} else {
		// If using `DisplayDefualt`, it will be broken in some environment.
		// Details are https://github.com/c-bata/go-prompt/pull/85
		w.SetDisplayAttributes(fg, bg, DisplayReset)
	}
	return
}

// SetDisplayAttributes to set VT100 display attributes.
func (w *VT100Writer) SetDisplayAttributes(fg, bg Color, attrs ...DisplayAttribute) {
	w.WriteRaw([]byte{0x1b, '['}) // control sequence introducer
	defer w.WriteRaw([]byte{'m'}) // final character

	var separator byte = ';'
	for i := range attrs {
		p, ok := displayAttributeParameters[attrs[i]]
		if !ok {
			continue
		}
		w.WriteRaw(p)
		w.WriteRaw([]byte{separator})
	}

	f, ok := foregroundANSIColors[fg]
	if !ok {
		f = foregroundANSIColors[DefaultColor]
	}
	w.WriteRaw(f)
	w.WriteRaw([]byte{separator})
	b, ok := backgroundANSIColors[bg]
	if !ok {
		b = backgroundANSIColors[DefaultColor]
	}
	w.WriteRaw(b)
	return
}

var displayAttributeParameters = map[DisplayAttribute][]byte{
	DisplayReset:        {'0'},
	DisplayBold:         {'1'},
	DisplayLowIntensity: {'2'},
	DisplayItalic:       {'3'},
	DisplayUnderline:    {'4'},
	DisplayBlink:        {'5'},
	DisplayRapidBlink:   {'6'},
	DisplayReverse:      {'7'},
	DisplayInvisible:    {'8'},
	DisplayCrossedOut:   {'9'},
	DisplayDefaultFont:  {'1', '0'},
}

var foregroundANSIColors = map[Color][]byte{
	DefaultColor: {'3', '9'},

	// Low intensity.
	Black:     {'3', '0'},
	DarkRed:   {'3', '1'},
	DarkGreen: {'3', '2'},
	Brown:     {'3', '3'},
	DarkBlue:  {'3', '4'},
	Purple:    {'3', '5'},
	Cyan:      {'3', '6'},
	LightGray: {'3', '7'},

	// High intensity.
	DarkGray:  {'9', '0'},
	Red:       {'9', '1'},
	Green:     {'9', '2'},
	Yellow:    {'9', '3'},
	Blue:      {'9', '4'},
	Fuchsia:   {'9', '5'},
	Turquoise: {'9', '6'},
	White:     {'9', '7'},
}

var backgroundANSIColors = map[Color][]byte{
	DefaultColor: {'4', '9'},

	// Low intensity.
	Black:     {'4', '0'},
	DarkRed:   {'4', '1'},
	DarkGreen: {'4', '2'},
	Brown:     {'4', '3'},
	DarkBlue:  {'4', '4'},
	Purple:    {'4', '5'},
	Cyan:      {'4', '6'},
	LightGray: {'4', '7'},

	// High intensity
	DarkGray:  {'1', '0', '0'},
	Red:       {'1', '0', '1'},
	Green:     {'1', '0', '2'},
	Yellow:    {'1', '0', '3'},
	Blue:      {'1', '0', '4'},
	Fuchsia:   {'1', '0', '5'},
	Turquoise: {'1', '0', '6'},
	White:     {'1', '0', '7'},
}
