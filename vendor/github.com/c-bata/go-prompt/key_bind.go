package prompt

// KeyBindFunc receives buffer and processed it.
type KeyBindFunc func(*Buffer)

// KeyBind represents which key should do what operation.
type KeyBind struct {
	Key Key
	Fn  KeyBindFunc
}

// ASCIICodeBind represents which []byte should do what operation
type ASCIICodeBind struct {
	ASCIICode []byte
	Fn        KeyBindFunc
}

// KeyBindMode to switch a key binding flexibly.
type KeyBindMode string

const (
	// CommonKeyBind is a mode without any keyboard shortcut
	CommonKeyBind KeyBindMode = "common"
	// EmacsKeyBind is a mode to use emacs-like keyboard shortcut
	EmacsKeyBind KeyBindMode = "emacs"
)

var commonKeyBindings = []KeyBind{
	// Go to the End of the line
	{
		Key: End,
		Fn:  GoLineEnd,
	},
	// Go to the beginning of the line
	{
		Key: Home,
		Fn:  GoLineBeginning,
	},
	// Delete character under the cursor
	{
		Key: Delete,
		Fn:  DeleteChar,
	},
	// Backspace
	{
		Key: Backspace,
		Fn:  DeleteBeforeChar,
	},
	// Right allow: Forward one character
	{
		Key: Right,
		Fn:  GoRightChar,
	},
	// Left allow: Backward one character
	{
		Key: Left,
		Fn:  GoLeftChar,
	},
}
