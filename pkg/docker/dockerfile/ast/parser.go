// Package ast implements a (low level) parser and parse tree dumper for Dockerfiles.
package ast

// Augmented BuildKit parser code to support linting

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

const (
	UnknownInstMsg = "unknown instruction"
)

var (
	ErrDockerfileUnknownInst = errors.New(UnknownInstMsg)
)

type ParseError struct {
	Context string
	Data    string
	Message string
}

func (e ParseError) Error() string {
	return e.Message
}

func pe(context, data, message string) ParseError {
	return ParseError{
		Context: context,
		Data:    data,
		Message: message,
	}
}

// Node is a structure used to represent a parse tree.
//
// In the node there are three fields, Value, Next, and Children. Value is the
// current token's string value. Next is always the next non-child token, and
// children contains all the children. Here's an example:
//
// (value next (child child-next child-next-next) next-next)
//
// This data structure is frankly pretty lousy for handling complex languages,
// but lucky for us the Dockerfile isn't very complicated. This structure
// works a little more effectively than a "proper" parse tree for our needs.
type Node struct {
	IsValid    bool
	Errors     []string
	Value      string // actual content
	ArgsRaw    string
	Next       *Node           // the next item in the current sexp
	Children   []*Node         // the children of this sexp
	Attributes map[string]bool // special attributes for this node
	Original   string          // original line used before parsing
	Flags      []string        // only top Node should have this set
	StartLine  int             // the line in the original dockerfile where the node begins
	EndLine    int             // the line in the original dockerfile where the node ends
}

// Dump dumps the AST defined by `node` as a list of sexps.
// Returns a string suitable for printing.
func (node *Node) Dump() string {
	str := ""

	if !node.IsValid {
		str += fmt.Sprintf("is_invalid errors='%+v' ", node.Errors)
	}

	str += fmt.Sprintf("value=%q", node.Value)

	if len(node.Flags) > 0 {
		str += fmt.Sprintf(" flags=%q", node.Flags)
	}

	if len(node.Attributes) > 0 {
		str += fmt.Sprintf(" attributes=%+v", node.Attributes)
	}

	if len(node.Children) > 0 {
		str += " children start:\n"
		for _, n := range node.Children {
			str += "(" + n.Dump() + ")\n"
		}
		str += ":children done\n"
	}

	for n := node.Next; n != nil; n = n.Next {
		if len(n.Children) > 0 {
			str += " next=" + n.Dump()
		} else {
			str += " nextv=" + strconv.Quote(n.Value)
		}
	}

	return strings.TrimSpace(str)
}

func (node *Node) lines(start, end int) {
	node.StartLine = start
	node.EndLine = end
}

// AddChild adds a new child node, and updates line information
func (node *Node) AddChild(child *Node, startLine, endLine int) {
	child.lines(startLine, endLine)
	if node.StartLine < 0 {
		node.StartLine = startLine
	}
	node.EndLine = endLine
	node.Children = append(node.Children, child)
}

var (
	dispatch           map[string]func(string, *Directive) (*Node, map[string]bool, error)
	tokenWhitespace    = regexp.MustCompile(`[\t\v\f\r ]+`)
	tokenEscapeCommand = regexp.MustCompile(`^#[ \t]*escape[ \t]*=[ \t]*(?P<escapechar>.).*$`)
	tokenComment       = regexp.MustCompile(`^#.*$`)
)

// DefaultEscapeToken is the default escape token
const DefaultEscapeToken = '\\'

// Directive is the structure used during a build run to hold the state of
// parsing directives.
type Directive struct {
	escapeToken           rune           // Current escape token
	lineContinuationRegex *regexp.Regexp // Current line continuation regex
	processingComplete    bool           // Whether we are done looking for directives
	escapeSeen            bool           // Whether the escape directive has been seen
}

// setEscapeToken sets the default token for escaping characters in a Dockerfile.
func (d *Directive) setEscapeToken(s string) error {
	if s != "`" && s != "\\" {
		return fmt.Errorf("invalid ESCAPE '%s'. Must be ` or \\", s)
	}
	d.escapeToken = rune(s[0])
	d.lineContinuationRegex = regexp.MustCompile(`\` + s + `[ \t]*$`)
	return nil
}

// possibleParserDirective looks for parser directives, eg '# escapeToken=<char>'.
// Parser directives must precede any builder instruction or other comments,
// and cannot be repeated.
func (d *Directive) possibleParserDirective(line string) error {
	if d.processingComplete {
		return nil
	}

	tecMatch := tokenEscapeCommand.FindStringSubmatch(strings.ToLower(line))
	if len(tecMatch) != 0 {
		for i, n := range tokenEscapeCommand.SubexpNames() {
			if n == "escapechar" {
				if d.escapeSeen {
					return errors.New("only one escape parser directive can be used")
				}
				d.escapeSeen = true
				return d.setEscapeToken(tecMatch[i])
			}
		}
	}

	d.processingComplete = true
	return nil
}

// NewDefaultDirective returns a new Directive with the default escapeToken token
func NewDefaultDirective() *Directive {
	directive := Directive{}
	directive.setEscapeToken(string(DefaultEscapeToken))
	return &directive
}

func init() {
	// Dispatch Table. see line_parsers.go for the parse functions.
	// The command is parsed and mapped to the line parser. The line parser
	// receives the arguments but not the command, and returns an AST after
	// reformulating the arguments according to the rules in the parser
	// functions. Errors are propagated up by Parse() and the resulting AST can
	// be incorporated directly into the existing AST as a next.
	dispatch = map[string]func(string, *Directive) (*Node, map[string]bool, error){
		instruction.Add:         parseMaybeJSONToList,
		instruction.Arg:         parseNameOrNameVal,
		instruction.Cmd:         parseMaybeJSON,
		instruction.Copy:        parseMaybeJSONToList,
		instruction.Entrypoint:  parseMaybeJSON,
		instruction.Env:         parseEnv,
		instruction.Expose:      parseStringsWhitespaceDelimited,
		instruction.From:        parseStringsWhitespaceDelimited,
		instruction.Healthcheck: parseHealthConfig,
		instruction.Label:       parseLabel,
		instruction.Maintainer:  parseString,
		instruction.Onbuild:     parseSubCommand,
		instruction.Run:         parseMaybeJSON,
		instruction.Shell:       parseMaybeJSON,
		instruction.StopSignal:  parseString,
		instruction.User:        parseString,
		instruction.Volume:      parseMaybeJSONToList,
		instruction.Workdir:     parseString,
	}
}

// newNodeFromLine splits the line into parts, and dispatches to a function
// based on the command and command arguments. A Node is created from the
// result of the dispatch.
func newNodeFromLine(line string, directive *Directive) (*Node, error) {
	cmd, flags, args, err := splitCommand(line)
	if err != nil {
		return nil, err
	}

	fn, found := dispatch[cmd]
	// Ignore invalid Dockerfile instructions
	if !found {
		fn = parseIgnore
	}

	var isValid bool
	var errors []string

	next, attrs, err := fn(args, directive)
	if err != nil {
		errors = append(errors, err.Error())
		next = &Node{
			Value: args,
		}
	} else {
		if !found {
			errors = append(errors, UnknownInstMsg)
			next.Value = args
		} else {
			isValid = true
		}
	}

	return &Node{
		IsValid:    isValid,
		Errors:     errors,
		Value:      cmd,
		Original:   line,
		Flags:      flags,
		Next:       next,
		Attributes: attrs,
		ArgsRaw:    args,
	}, nil
}

// Result is the result of parsing a Dockerfile
type Result struct {
	Lines       []string
	AST         *Node
	EscapeToken rune
	Warnings    []string
}

// PrintWarnings to the writer
func (r *Result) PrintWarnings(out io.Writer) {
	if len(r.Warnings) == 0 {
		return
	}
	fmt.Fprintf(out, strings.Join(r.Warnings, "\n")+"\n")
}

// Parse reads lines from a Reader, parses the lines into an AST and returns
// the AST and escape token
func Parse(rwc io.Reader) (*Result, error) {
	var lines []string
	d := NewDefaultDirective()
	currentLine := 0

	root := &Node{IsValid: true, StartLine: -1}
	scanner := bufio.NewScanner(rwc)
	warnings := []string{}

	var err error
	for scanner.Scan() {
		bytesRead := scanner.Bytes()
		if currentLine == 0 {
			// First line, strip the byte-order-marker if present
			bytesRead = bytes.TrimPrefix(bytesRead, utf8bom)
		}

		lines = append(lines, string(bytesRead))

		bytesRead, err = processLine(d, bytesRead, true)
		if err != nil {
			return nil, err
		}
		currentLine++

		startLine := currentLine
		line, isEndOfLine := trimContinuationCharacter(string(bytesRead), d)
		if isEndOfLine && line == "" {
			continue
		}

		var hasEmptyContinuationLine bool
		for !isEndOfLine && scanner.Scan() {
			bytesRead, err := processLine(d, scanner.Bytes(), false)
			if err != nil {
				return nil, err
			}
			currentLine++

			if isComment(scanner.Bytes()) {
				// original line was a comment (processLine strips comments)
				continue
			}
			if isEmptyContinuationLine(bytesRead) {
				hasEmptyContinuationLine = true
				continue
			}

			continuationLine := string(bytesRead)
			continuationLine, isEndOfLine = trimContinuationCharacter(continuationLine, d)
			line += continuationLine
		}

		if hasEmptyContinuationLine {
			warnings = append(warnings, "[WARNING]: Empty continuation line found in:\n    "+line)
		}

		child, err := newNodeFromLine(line, d)
		if err != nil {
			return nil, err
		}
		root.AddChild(child, startLine, currentLine)
	}

	if len(warnings) > 0 {
		warnings = append(warnings, "[WARNING]: Empty continuation lines will become errors in a future release.")
	}

	if root.StartLine < 0 {
		warnings = append(warnings, "[WARNING]: File with no instructions.")
		//return nil, errors.New("file with no instructions.")
	}

	return &Result{
		Lines:       lines,
		AST:         root,
		Warnings:    warnings,
		EscapeToken: d.escapeToken,
	}, handleScannerError(scanner.Err())
}

func trimComments(src []byte) []byte {
	return tokenComment.ReplaceAll(src, []byte{})
}

func trimWhitespace(src []byte) []byte {
	return bytes.TrimLeftFunc(src, unicode.IsSpace)
}

func isComment(line []byte) bool {
	return tokenComment.Match(trimWhitespace(line))
}

func isEmptyContinuationLine(line []byte) bool {
	return len(trimWhitespace(line)) == 0
}

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

func trimContinuationCharacter(line string, d *Directive) (string, bool) {
	if d.lineContinuationRegex.MatchString(line) {
		line = d.lineContinuationRegex.ReplaceAllString(line, "")
		return line, false
	}
	return line, true
}

// TODO: remove stripLeftWhitespace after deprecation period. It seems silly
// to preserve whitespace on continuation lines. Why is that done?
func processLine(d *Directive, token []byte, stripLeftWhitespace bool) ([]byte, error) {
	if stripLeftWhitespace {
		token = trimWhitespace(token)
	}
	return trimComments(token), d.possibleParserDirective(string(token))
}

func handleScannerError(err error) error {
	switch err {
	case bufio.ErrTooLong:
		return errors.Errorf("dockerfile line greater than max allowed size of %d", bufio.MaxScanTokenSize-1)
	default:
		return err
	}
}
