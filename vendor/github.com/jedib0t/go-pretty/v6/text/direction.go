package text

// Direction defines the overall flow of text. Similar to bidi.Direction, but
// simplified and specific to this package.
type Direction int

// Available Directions.
const (
	Default Direction = iota
	LeftToRight
	RightToLeft
)

// Modifier returns a character to force the given direction for the text that
// follows the modifier.
func (d Direction) Modifier() string {
	switch d {
	case LeftToRight:
		return "\u202a"
	case RightToLeft:
		return "\u202b"
	}
	return ""
}
