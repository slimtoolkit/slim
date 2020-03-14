// Code generated "This is a fake comment to avoid golint errors"; DO NOT EDIT.
// FIXME: This is a little bit stupid, but there are many public constants which is no value for writing godoc comment.

package prompt

// Key is the type express the key inserted from user.
type Key int

// ASCIICode is the type contains Key and it's ascii byte array.
type ASCIICode struct {
	Key       Key
	ASCIICode []byte
}

const (
	Escape Key = iota

	ControlA
	ControlB
	ControlC
	ControlD
	ControlE
	ControlF
	ControlG
	ControlH
	ControlI
	ControlJ
	ControlK
	ControlL
	ControlM
	ControlN
	ControlO
	ControlP
	ControlQ
	ControlR
	ControlS
	ControlT
	ControlU
	ControlV
	ControlW
	ControlX
	ControlY
	ControlZ

	ControlSpace
	ControlBackslash
	ControlSquareClose
	ControlCircumflex
	ControlUnderscore
	ControlLeft
	ControlRight
	ControlUp
	ControlDown

	Up
	Down
	Right
	Left

	ShiftLeft
	ShiftUp
	ShiftDown
	ShiftRight

	Home
	End
	Delete
	ShiftDelete
	ControlDelete
	PageUp
	PageDown
	BackTab
	Insert
	Backspace

	// Aliases.
	Tab
	Enter
	// Actually Enter equals ControlM, not ControlJ,
	// However, in prompt_toolkit, we made the mistake of translating
	// \r into \n during the input, so everyone is now handling the
	// enter key by binding ControlJ.

	// From now on, it's better to bind `ASCII_SEQUENCES.Enter` everywhere,
	// because that's future compatible, and will still work when we
	// stop replacing \r by \n.

	F1
	F2
	F3
	F4
	F5
	F6
	F7
	F8
	F9
	F10
	F11
	F12
	F13
	F14
	F15
	F16
	F17
	F18
	F19
	F20
	F21
	F22
	F23
	F24

	// Matches any key.
	Any

	// Special
	CPRResponse
	Vt100MouseEvent
	WindowsMouseEvent
	BracketedPaste

	// Key which is ignored. (The key binding for this key should not do anything.)
	Ignore

	// Key is not defined
	NotDefined
)
