package xmlquery

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/antchfx/xpath"
	"golang.org/x/net/html/charset"
)

// LoadURL loads the XML document from the specified URL.
func LoadURL(url string) (*Node, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Checking the HTTP Content-Type value from the response headers.(#39)
	v := strings.ToLower(resp.Header.Get("Content-Type"))
	if v == "text/xml" || v == "application/xml" {
		return Parse(resp.Body)
	}
	return nil, fmt.Errorf("invalid XML document(%s)", v)
}

// Parse returns the parse tree for the XML from the given Reader.
func Parse(r io.Reader) (*Node, error) {
	p := createParser(r)
	for {
		_, err := p.parse()
		if err == io.EOF {
			return p.doc, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

type parser struct {
	decoder             *xml.Decoder
	doc                 *Node
	space2prefix        map[string]string
	level               int
	prev                *Node
	streamElementXPath  *xpath.Expr // Under streaming mode, this specifies the xpath to the target element node(s).
	streamElementFilter *xpath.Expr // If specified, it provides a futher filtering on the target element.
	streamNode          *Node       // Need to remmeber the last target node So we can clean it up upon next Read() call.
	streamNodePrev      *Node       // Need to remember target node's prev so upon target node removal, we can restore correct prev.
}

func createParser(r io.Reader) *parser {
	p := &parser{
		decoder:      xml.NewDecoder(r),
		doc:          &Node{Type: DocumentNode},
		space2prefix: make(map[string]string),
		level:        0,
	}
	// http://www.w3.org/XML/1998/namespace is bound by definition to the prefix xml.
	p.space2prefix["http://www.w3.org/XML/1998/namespace"] = "xml"
	p.decoder.CharsetReader = charset.NewReaderLabel
	p.prev = p.doc
	return p
}

func (p *parser) parse() (*Node, error) {
	var streamElementNodeCounter int

	for {
		tok, err := p.decoder.Token()
		if err != nil {
			return nil, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			if p.level == 0 {
				// mising XML declaration
				node := &Node{Type: DeclarationNode, Data: "xml", level: 1}
				AddChild(p.prev, node)
				p.level = 1
				p.prev = node
			}
			// https://www.w3.org/TR/xml-names/#scoping-defaulting
			for _, att := range tok.Attr {
				if att.Name.Local == "xmlns" {
					p.space2prefix[att.Value] = ""
				} else if att.Name.Space == "xmlns" {
					p.space2prefix[att.Value] = att.Name.Local
				}
			}

			if tok.Name.Space != "" {
				if _, found := p.space2prefix[tok.Name.Space]; !found {
					return nil, errors.New("xmlquery: invalid XML document, namespace is missing")
				}
			}

			for i := 0; i < len(tok.Attr); i++ {
				att := &tok.Attr[i]
				if prefix, ok := p.space2prefix[att.Name.Space]; ok {
					att.Name.Space = prefix
				}
			}

			node := &Node{
				Type:         ElementNode,
				Data:         tok.Name.Local,
				Prefix:       p.space2prefix[tok.Name.Space],
				NamespaceURI: tok.Name.Space,
				Attr:         tok.Attr,
				level:        p.level,
			}
			//fmt.Println(fmt.Sprintf("start > %s : %d", node.Data, node.level))
			if p.level == p.prev.level {
				AddSibling(p.prev, node)
			} else if p.level > p.prev.level {
				AddChild(p.prev, node)
			} else if p.level < p.prev.level {
				for i := p.prev.level - p.level; i > 1; i-- {
					p.prev = p.prev.Parent
				}
				AddSibling(p.prev.Parent, node)
			}
			// If we're in the streaming mode, we need to remember the node if it is the target node
			// so that when we finish processing the node's EndElement, we know how/what to return to
			// caller. Also we need to remove the target node from the tree upon next Read() call so
			// memory doesn't grow unbounded.
			if p.streamElementXPath != nil {
				if p.streamNode == nil {
					if QuerySelector(p.doc, p.streamElementXPath) != nil {
						p.streamNode = node
						p.streamNodePrev = p.prev
						streamElementNodeCounter = 1
					}
				} else {
					streamElementNodeCounter++
				}
			}
			p.prev = node
			p.level++
		case xml.EndElement:
			p.level--
			// If we're in streaming mode, and we already have a potential streaming
			// target node identified (p.streamNode != nil) then we need to check if
			// this is the real one we want to return to caller.
			if p.streamNode != nil {
				streamElementNodeCounter--
				if streamElementNodeCounter == 0 {
					// Now we know this element node is the at least passing the initial
					// p.streamElementXPath check and is a potential target node candidate.
					// We need to have 1 more check with p.streamElementFilter (if given) to
					// ensure it is really the element node we want.
					// The reason we need a two-step check process is because the following
					// situation:
					//   <AAA><BBB>b1</BBB></AAA>
					// And say the p.streamElementXPath = "/AAA/BBB[. != 'b1']". Now during
					// xml.StartElement time, the <BBB> node is still empty, so it will pass
					// the p.streamElementXPath check. However, eventually we know this <BBB>
					// shouldn't be returned to the caller. Having a second more fine-grained
					// filter check ensures that. So in this case, the caller should really
					// setup the stream parser with:
					//   streamElementXPath = "/AAA/BBB["
					//   streamElementFilter = "/AAA/BBB[. != 'b1']"
					if p.streamElementFilter == nil || QuerySelector(p.doc, p.streamElementFilter) != nil {
						return p.streamNode, nil
					}
					// otherwise, this isn't our target node, clean things up.
					// note we also remove the underlying *Node from the node tree, to prevent
					// future stream node candidate selection error.
					RemoveFromTree(p.streamNode)
					p.prev = p.streamNodePrev
					p.streamNode = nil
					p.streamNodePrev = nil
				}
			}
		case xml.CharData:
			node := &Node{Type: CharDataNode, Data: string(tok), level: p.level}
			if p.level == p.prev.level {
				AddSibling(p.prev, node)
			} else if p.level > p.prev.level {
				AddChild(p.prev, node)
			} else if p.level < p.prev.level {
				for i := p.prev.level - p.level; i > 1; i-- {
					p.prev = p.prev.Parent
				}
				AddSibling(p.prev.Parent, node)
			}
		case xml.Comment:
			node := &Node{Type: CommentNode, Data: string(tok), level: p.level}
			if p.level == p.prev.level {
				AddSibling(p.prev, node)
			} else if p.level > p.prev.level {
				AddChild(p.prev, node)
			} else if p.level < p.prev.level {
				for i := p.prev.level - p.level; i > 1; i-- {
					p.prev = p.prev.Parent
				}
				AddSibling(p.prev.Parent, node)
			}
		case xml.ProcInst: // Processing Instruction
			if p.prev.Type != DeclarationNode {
				p.level++
			}
			node := &Node{Type: DeclarationNode, Data: tok.Target, level: p.level}
			pairs := strings.Split(string(tok.Inst), " ")
			for _, pair := range pairs {
				pair = strings.TrimSpace(pair)
				if i := strings.Index(pair, "="); i > 0 {
					AddAttr(node, pair[:i], strings.Trim(pair[i+1:], `"`))
				}
			}
			if p.level == p.prev.level {
				AddSibling(p.prev, node)
			} else if p.level > p.prev.level {
				AddChild(p.prev, node)
			}
			p.prev = node
		case xml.Directive:
		}
	}
}

// StreamParser enables loading and parsing an XML document in a streaming fashion.
type StreamParser struct {
	p *parser
}

// CreateStreamParser creates a StreamParser. Argument streamElementXPath is required.
// Argument streamElementFilter is optional and should only be used in advanced scenarios.
//
// Scenario 1: simple case:
//  xml := `<AAA><BBB>b1</BBB><BBB>b2</BBB></AAA>`
//  sp, err := CreateStreamParser(strings.NewReader(xml), "/AAA/BBB")
//  if err != nil {
//      panic(err)
//  }
//  for {
//      n, err := sp.Read()
//      if err != nil {
//          break
//      }
//      fmt.Println(n.OutputXML(true))
//  }
// Output will be:
//   <BBB>b1</BBB>
//   <BBB>b2</BBB>
//
// Scenario 2: advanced case:
//  xml := `<AAA><BBB>b1</BBB><BBB>b2</BBB></AAA>`
//  sp, err := CreateStreamParser(strings.NewReader(xml), "/AAA/BBB", "/AAA/BBB[. != 'b1']")
//  if err != nil {
//      panic(err)
//  }
//  for {
//      n, err := sp.Read()
//      if err != nil {
//          break
//      }
//      fmt.Println(n.OutputXML(true))
//  }
// Output will be:
//   <BBB>b2</BBB>
//
// As the argument names indicate, streamElementXPath should be used for providing xpath query pointing
// to the target element node only, no extra filtering on the element itself or its children; while
// streamElementFilter, if needed, can provide additional filtering on the target element and its children.
//
// CreateStreamParser returns error if either streamElementXPath or streamElementFilter, if provided, cannot
// be successfully parsed and compiled into a valid xpath query.
func CreateStreamParser(r io.Reader, streamElementXPath string, streamElementFilter ...string) (*StreamParser, error) {
	elemXPath, err := getQuery(streamElementXPath)
	if err != nil {
		return nil, fmt.Errorf("invalid streamElementXPath '%s', err: %s", streamElementXPath, err.Error())
	}
	elemFilter := (*xpath.Expr)(nil)
	if len(streamElementFilter) > 0 {
		elemFilter, err = getQuery(streamElementFilter[0])
		if err != nil {
			return nil, fmt.Errorf("invalid streamElementFilter '%s', err: %s", streamElementFilter[0], err.Error())
		}
	}
	sp := &StreamParser{
		p: createParser(r),
	}
	sp.p.streamElementXPath = elemXPath
	sp.p.streamElementFilter = elemFilter
	return sp, nil
}

// Read returns a target node that satisifies the XPath specified by caller at StreamParser creation
// time. If there is no more satisifying target node after reading the rest of the XML document, io.EOF
// will be returned. At any time, any XML parsing error encountered, the error will be returned and
// the stream parsing is stopped. Calling Read() after an error is returned (including io.EOF) is not
// allowed the behavior will be undefined. Also note, due to the streaming nature, calling Read() will
// automatically remove any previous target node(s) from the document tree.
func (sp *StreamParser) Read() (*Node, error) {
	// Because this is a streaming read, we need to release/remove last
	// target node from the node tree to free up memory.
	if sp.p.streamNode != nil {
		RemoveFromTree(sp.p.streamNode)
		sp.p.prev = sp.p.streamNodePrev
		sp.p.streamNode = nil
		sp.p.streamNodePrev = nil
	}
	return sp.p.parse()
}
