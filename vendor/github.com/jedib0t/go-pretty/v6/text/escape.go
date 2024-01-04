package text

import "strings"

// Constants
const (
	CSIStartRune    = rune(91) // [
	CSIStopRune     = 'm'
	EscapeReset     = EscapeStart + "0" + EscapeStop
	EscapeStart     = "\x1b["
	EscapeStartRune = rune(27) // \x1b
	EscapeStop      = "m"
	EscapeStopRune  = 'm'
	OSIStartRune    = rune(93) // ]
	OSIStopRune     = '\\'
)

type escKind int

const (
	escKindUnknown escKind = iota
	escKindCSI
	escKindOSI
)

type escSeq struct {
	isIn    bool
	content strings.Builder
	kind    escKind
}

func (e *escSeq) InspectRune(r rune) {
	if !e.isIn && r == EscapeStartRune {
		e.isIn = true
		e.kind = escKindUnknown
		e.content.Reset()
		e.content.WriteRune(r)
	} else if e.isIn {
		switch {
		case e.kind == escKindUnknown && r == CSIStartRune:
			e.kind = escKindCSI
		case e.kind == escKindUnknown && r == OSIStartRune:
			e.kind = escKindOSI
		case e.kind == escKindCSI && r == CSIStopRune || e.kind == escKindOSI && r == OSIStopRune:
			e.isIn = false
			e.kind = escKindUnknown
		}
		e.content.WriteRune(r)
	}
}
