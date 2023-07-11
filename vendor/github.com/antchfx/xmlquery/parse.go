package xmlquery

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/antchfx/xpath"
	"golang.org/x/net/html/charset"
)

var xmlMIMERegex = regexp.MustCompile(`(?i)((application|image|message|model)/((\w|\.|-)+\+?)?|text/)(wb)?xml`)

// LoadURL loads the XML document from the specified URL.
func LoadURL(url string) (*Node, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Make sure the Content-Type has a valid XML MIME type
	if xmlMIMERegex.MatchString(resp.Header.Get("Content-Type")) {
		return Parse(resp.Body)
	}
	return nil, fmt.Errorf("invalid XML document(%s)", resp.Header.Get("Content-Type"))
}

// Parse returns the parse tree for the XML from the given Reader.
func Parse(r io.Reader) (*Node, error) {
	return ParseWithOptions(r, ParserOptions{})
}

// ParseWithOptions is like parse, but with custom options
func ParseWithOptions(r io.Reader, options ParserOptions) (*Node, error) {
	p := createParser(r)
	options.apply(p)
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
	level               int
	prev                *Node
	streamElementXPath  *xpath.Expr   // Under streaming mode, this specifies the xpath to the target element node(s).
	streamElementFilter *xpath.Expr   // If specified, it provides further filtering on the target element.
	streamNode          *Node         // Need to remember the last target node So we can clean it up upon next Read() call.
	streamNodePrev      *Node         // Need to remember target node's prev so upon target node removal, we can restore correct prev.
	reader              *cachedReader // Need to maintain a reference to the reader, so we can determine whether a node contains CDATA.
}

func createParser(r io.Reader) *parser {
	reader := newCachedReader(bufio.NewReader(r))
	p := &parser{
		decoder: xml.NewDecoder(reader),
		doc:     &Node{Type: DocumentNode},
		level:   0,
		reader:  reader,
	}
	if p.decoder.CharsetReader == nil {
		p.decoder.CharsetReader = charset.NewReaderLabel
	}
	p.prev = p.doc
	return p
}

func (p *parser) parse() (*Node, error) {
	var streamElementNodeCounter int
	space2prefix := map[string]string{"http://www.w3.org/XML/1998/namespace": "xml"}

	for {
		p.reader.StartCaching()
		tok, err := p.decoder.Token()
		p.reader.StopCaching()
		if err != nil {
			return nil, err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			if p.level == 0 {
				// mising XML declaration
				attributes := make([]Attr, 1)
				attributes[0].Name = xml.Name{Local: "version"}
				attributes[0].Value = "1.0"
				node := &Node{
					Type:  DeclarationNode,
					Data:  "xml",
					Attr:  attributes,
					level: 1,
				}
				AddChild(p.prev, node)
				p.level = 1
				p.prev = node
			}

			for _, att := range tok.Attr {
				if att.Name.Local == "xmlns" {
					space2prefix[att.Value] = "" // reset empty if exist the default namespace
					//	defaultNamespaceURL = att.Value
				} else if att.Name.Space == "xmlns" {
					// maybe there are have duplicate NamespaceURL?
					space2prefix[att.Value] = att.Name.Local
				}
			}

			if space := tok.Name.Space; space != "" {
				if _, found := space2prefix[space]; !found && p.decoder.Strict {
					return nil, fmt.Errorf("xmlquery: invalid XML document, namespace %s is missing", space)
				}
			}

			attributes := make([]Attr, len(tok.Attr))
			for i, att := range tok.Attr {
				name := att.Name
				if prefix, ok := space2prefix[name.Space]; ok {
					name.Space = prefix
				}
				attributes[i] = Attr{
					Name:         name,
					Value:        att.Value,
					NamespaceURI: att.Name.Space,
				}
			}

			node := &Node{
				Type:         ElementNode,
				Data:         tok.Name.Local,
				NamespaceURI: tok.Name.Space,
				Attr:         attributes,
				level:        p.level,
			}

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

			if node.NamespaceURI != "" {
				if v, ok := space2prefix[node.NamespaceURI]; ok {
					cached := string(p.reader.Cache())
					if strings.HasPrefix(cached, fmt.Sprintf("%s:%s", v, node.Data)) || strings.HasPrefix(cached, fmt.Sprintf("<%s:%s", v, node.Data)) {
						node.Prefix = v
					}
				}
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
			// First, normalize the cache...
			cached := strings.ToUpper(string(p.reader.Cache()))
			nodeType := TextNode
			if strings.HasPrefix(cached, "<![CDATA[") || strings.HasPrefix(cached, "![CDATA[") {
				nodeType = CharDataNode
			}

			node := &Node{Type: nodeType, Data: string(tok), level: p.level}
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
			} else if p.level < p.prev.level {
				for i := p.prev.level - p.level; i > 1; i-- {
					p.prev = p.prev.Parent
				}
				AddSibling(p.prev.Parent, node)
			}
			p.prev = node
		case xml.Directive:
		}
	}
}

// StreamParser enables loading and parsing an XML document in a streaming
// fashion.
type StreamParser struct {
	p *parser
}

// CreateStreamParser creates a StreamParser. Argument streamElementXPath is
// required.
// Argument streamElementFilter is optional and should only be used in advanced
// scenarios.
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
// As the argument names indicate, streamElementXPath should be used for
// providing xpath query pointing to the target element node only, no extra
// filtering on the element itself or its children; while streamElementFilter,
// if needed, can provide additional filtering on the target element and its
// children.
//
// CreateStreamParser returns an error if either streamElementXPath or
// streamElementFilter, if provided, cannot be successfully parsed and compiled
// into a valid xpath query.
func CreateStreamParser(r io.Reader, streamElementXPath string, streamElementFilter ...string) (*StreamParser, error) {
	return CreateStreamParserWithOptions(r, ParserOptions{}, streamElementXPath, streamElementFilter...)
}

// CreateStreamParserWithOptions is like CreateStreamParser, but with custom options
func CreateStreamParserWithOptions(
	r io.Reader,
	options ParserOptions,
	streamElementXPath string,
	streamElementFilter ...string,
) (*StreamParser, error) {
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
	parser := createParser(r)
	options.apply(parser)
	sp := &StreamParser{
		p: parser,
	}
	sp.p.streamElementXPath = elemXPath
	sp.p.streamElementFilter = elemFilter
	return sp, nil
}

// Read returns a target node that satisfies the XPath specified by caller at
// StreamParser creation time. If there is no more satisfying target nodes after
// reading the rest of the XML document, io.EOF will be returned. At any time,
// any XML parsing error encountered will be returned, and the stream parsing
// stopped. Calling Read() after an error is returned (including io.EOF) results
// undefined behavior. Also note, due to the streaming nature, calling Read()
// will automatically remove any previous target node(s) from the document tree.
func (sp *StreamParser) Read() (*Node, error) {
	// Because this is a streaming read, we need to release/remove last
	// target node from the node tree to free up memory.
	if sp.p.streamNode != nil {
		// We need to remove all siblings before the current stream node,
		// because the document may contain unwanted nodes between the target
		// ones (for example new line text node), which would otherwise
		// accumulate as first childs, and slow down the stream over time
		for sp.p.streamNode.PrevSibling != nil {
			RemoveFromTree(sp.p.streamNode.PrevSibling)
		}
		sp.p.prev = sp.p.streamNode.Parent
		RemoveFromTree(sp.p.streamNode)
		sp.p.streamNode = nil
		sp.p.streamNodePrev = nil
	}
	return sp.p.parse()
}
