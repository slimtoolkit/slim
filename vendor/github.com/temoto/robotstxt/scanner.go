package robotstxt

import (
	"bytes"
	"fmt"
	"go/token"
	"os"
	"sync"
	"unicode/utf8"
)

type byteScanner struct {
	pos           token.Position
	buf           []byte
	ErrorCount    int
	ch            rune
	Quiet         bool
	keyTokenFound bool
	lastChunk     bool
}

const tokEOL = "\n"

var WhitespaceChars = []rune{' ', '\t', '\v'}
var tokBuffers = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 32)) }}

func newByteScanner(srcname string, quiet bool) *byteScanner {
	return &byteScanner{
		Quiet: quiet,
		ch:    -1,
		pos:   token.Position{Filename: srcname},
	}
}

func (s *byteScanner) feed(input []byte, end bool) {
	s.buf = input
	s.pos.Offset = 0
	s.pos.Line = 1
	s.pos.Column = 1
	s.lastChunk = end

	// Read first char into look-ahead buffer `s.ch`.
	if !s.nextChar() {
		return
	}

	// Skip UTF-8 byte order mark
	if s.ch == 65279 {
		s.nextChar()
		s.pos.Column = 1
	}
}

func (s *byteScanner) GetPosition() token.Position {
	return s.pos
}

func (s *byteScanner) scan() string {
	// Note Offset > len, not >=, so we can scan last character.
	if s.lastChunk && s.pos.Offset > len(s.buf) {
		return ""
	}

	s.skipSpace()

	if s.ch == -1 {
		return ""
	}

	// EOL
	if s.isEol() {
		s.keyTokenFound = false
		// skip subsequent newline chars
		for s.ch != -1 && s.isEol() {
			s.nextChar()
		}
		// emit newline as separate token
		return tokEOL
	}

	// skip comments
	if s.ch == '#' {
		s.keyTokenFound = false
		s.skipUntilEol()
		if s.ch == -1 {
			return ""
		}
		// emit newline as separate token
		return tokEOL
	}

	// else we found something
	tok := tokBuffers.Get().(*bytes.Buffer)
	defer tokBuffers.Put(tok)
	tok.Reset()
	tok.WriteRune(s.ch)
	s.nextChar()
	for s.ch != -1 && !s.isSpace() && !s.isEol() {
		// Do not consider ":" to be a token separator if a first key token
		// has already been found on this line (avoid cutting an absolute URL
		// after the "http:")
		if s.ch == ':' && !s.keyTokenFound {
			s.nextChar()
			s.keyTokenFound = true
			break
		}

		tok.WriteRune(s.ch)
		s.nextChar()
	}
	return tok.String()
}

func (s *byteScanner) scanAll() []string {
	results := make([]string, 0, 64) // random guess of average tokens length
	for {
		token := s.scan()
		if token != "" {
			results = append(results, token)
		} else {
			break
		}
	}
	return results
}

func (s *byteScanner) error(pos token.Position, msg string) {
	s.ErrorCount++
	if !s.Quiet {
		fmt.Fprintf(os.Stderr, "robotstxt from %s: %s\n", pos.String(), msg)
	}
}

func (s *byteScanner) isEol() bool {
	return s.ch == '\n' || s.ch == '\r'
}

func (s *byteScanner) isSpace() bool {
	for _, r := range WhitespaceChars {
		if s.ch == r {
			return true
		}
	}
	return false
}

func (s *byteScanner) skipSpace() {
	for s.ch != -1 && s.isSpace() {
		s.nextChar()
	}
}

func (s *byteScanner) skipUntilEol() {
	for s.ch != -1 && !s.isEol() {
		s.nextChar()
	}
	// skip subsequent newline chars
	for s.ch != -1 && s.isEol() {
		s.nextChar()
	}
}

// Reads next Unicode char.
func (s *byteScanner) nextChar() bool {
	if s.pos.Offset >= len(s.buf) {
		s.ch = -1
		return false
	}
	s.pos.Column++
	if s.ch == '\n' {
		s.pos.Line++
		s.pos.Column = 1
	}
	r, w := rune(s.buf[s.pos.Offset]), 1
	if r >= 0x80 {
		r, w = utf8.DecodeRune(s.buf[s.pos.Offset:])
		if r == utf8.RuneError && w == 1 {
			s.error(s.pos, "illegal UTF-8 encoding")
		}
	}
	s.pos.Column++
	s.pos.Offset += w
	s.ch = r
	return true
}
