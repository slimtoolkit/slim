package prompt

import "sync"

var (
	consoleWriterMu sync.Mutex
	consoleWriter   ConsoleWriter
)

func registerConsoleWriter(f ConsoleWriter) {
	consoleWriterMu.Lock()
	defer consoleWriterMu.Unlock()
	consoleWriter = f
}

// DisplayAttribute represents display  attributes like Blinking, Bold, Italic and so on.
type DisplayAttribute int

const (
	// DisplayReset reset all display attributes.
	DisplayReset DisplayAttribute = iota
	// DisplayBold set bold or increases intensity.
	DisplayBold
	// DisplayLowIntensity decreases intensity. Not widely supported.
	DisplayLowIntensity
	// DisplayItalic set italic. Not widely supported.
	DisplayItalic
	// DisplayUnderline set underline
	DisplayUnderline
	// DisplayBlink set blink (less than 150 per minute).
	DisplayBlink
	// DisplayRapidBlink set blink (more than 150 per minute). Not widely supported.
	DisplayRapidBlink
	// DisplayReverse swap foreground and background colors.
	DisplayReverse
	// DisplayInvisible set invisible.  Not widely supported.
	DisplayInvisible
	// DisplayCrossedOut set characters legible, but marked for deletion. Not widely supported.
	DisplayCrossedOut
	// DisplayDefaultFont set primary(default) font
	DisplayDefaultFont
)

// Color represents color on terminal.
type Color int

const (
	// DefaultColor represents a default color.
	DefaultColor Color = iota

	// Low intensity

	// Black represents a black.
	Black
	// DarkRed represents a dark red.
	DarkRed
	// DarkGreen represents a dark green.
	DarkGreen
	// Brown represents a brown.
	Brown
	// DarkBlue represents a dark blue.
	DarkBlue
	// Purple represents a purple.
	Purple
	// Cyan represents a cyan.
	Cyan
	// LightGray represents a light gray.
	LightGray

	// High intensity

	// DarkGray represents a dark gray.
	DarkGray
	// Red represents a red.
	Red
	// Green represents a green.
	Green
	// Yellow represents a yellow.
	Yellow
	// Blue represents a blue.
	Blue
	// Fuchsia represents a fuchsia.
	Fuchsia
	// Turquoise represents a turquoise.
	Turquoise
	// White represents a white.
	White
)

// ConsoleWriter is an interface to abstract output layer.
type ConsoleWriter interface {
	/* Write */

	// WriteRaw to write raw byte array.
	WriteRaw(data []byte)
	// Write to write safety byte array by removing control sequences.
	Write(data []byte)
	// WriteStr to write raw string.
	WriteRawStr(data string)
	// WriteStr to write safety string by removing control sequences.
	WriteStr(data string)
	// Flush to flush buffer.
	Flush() error

	/* Erasing */

	// EraseScreen erases the screen with the background colour and moves the cursor to home.
	EraseScreen()
	// EraseUp erases the screen from the current line up to the top of the screen.
	EraseUp()
	// EraseDown erases the screen from the current line down to the bottom of the screen.
	EraseDown()
	// EraseStartOfLine erases from the current cursor position to the start of the current line.
	EraseStartOfLine()
	// EraseEndOfLine erases from the current cursor position to the end of the current line.
	EraseEndOfLine()
	// EraseLine erases the entire current line.
	EraseLine()

	/* Cursor */

	// ShowCursor stops blinking cursor and show.
	ShowCursor()
	// HideCursor hides cursor.
	HideCursor()
	// CursorGoTo sets the cursor position where subsequent text will begin.
	CursorGoTo(row, col int)
	// CursorUp moves the cursor up by 'n' rows; the default count is 1.
	CursorUp(n int)
	// CursorDown moves the cursor down by 'n' rows; the default count is 1.
	CursorDown(n int)
	// CursorForward moves the cursor forward by 'n' columns; the default count is 1.
	CursorForward(n int)
	// CursorBackward moves the cursor backward by 'n' columns; the default count is 1.
	CursorBackward(n int)
	// AskForCPR asks for a cursor position report (CPR).
	AskForCPR()
	// SaveCursor saves current cursor position.
	SaveCursor()
	// UnSaveCursor restores cursor position after a Save Cursor.
	UnSaveCursor()

	/* Scrolling */

	// ScrollDown scrolls display down one line.
	ScrollDown()
	// ScrollUp scroll display up one line.
	ScrollUp()

	/* Title */

	// SetTitle sets a title of terminal window.
	SetTitle(title string)
	// ClearTitle clears a title of terminal window.
	ClearTitle()

	/* Font */

	// SetColor sets text and background colors. and specify whether text is bold.
	SetColor(fg, bg Color, bold bool)
}
