/*
Package htmlquery provides extract data from HTML documents using XPath expression.
*/
package htmlquery

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/antchfx/xpath"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

var _ xpath.NodeNavigator = &NodeNavigator{}

// CreateXPathNavigator creates a new xpath.NodeNavigator for the specified html.Node.
func CreateXPathNavigator(top *html.Node) *NodeNavigator {
	return &NodeNavigator{curr: top, root: top, attr: -1}
}

// Find is like QueryAll but Will panics if the expression `expr` cannot be parsed.
//
// See `QueryAll()` function.
func Find(top *html.Node, expr string) []*html.Node {
	nodes, err := QueryAll(top, expr)
	if err != nil {
		panic(err)
	}
	return nodes
}

// FindOne is like Query but will panics if the expression `expr` cannot be parsed.
// See `Query()` function.
func FindOne(top *html.Node, expr string) *html.Node {
	node, err := Query(top, expr)
	if err != nil {
		panic(err)
	}
	return node
}

// QueryAll searches the html.Node that matches by the specified XPath expr.
// Return an error if the expression `expr` cannot be parsed.
func QueryAll(top *html.Node, expr string) ([]*html.Node, error) {
	exp, err := getQuery(expr)
	if err != nil {
		return nil, err
	}
	nodes := QuerySelectorAll(top, exp)
	return nodes, nil
}

// Query runs the given XPath expression against the given html.Node and
// returns the first matching html.Node, or nil if no matches are found.
//
// Returns an error if the expression `expr` cannot be parsed.
func Query(top *html.Node, expr string) (*html.Node, error) {
	exp, err := getQuery(expr)
	if err != nil {
		return nil, err
	}
	return QuerySelector(top, exp), nil
}

// QuerySelector returns the first matched html.Node by the specified XPath selector.
func QuerySelector(top *html.Node, selector *xpath.Expr) *html.Node {
	t := selector.Select(CreateXPathNavigator(top))
	if t.MoveNext() {
		return getCurrentNode(t.Current().(*NodeNavigator))
	}
	return nil
}

// QuerySelectorAll searches all of the html.Node that matches the specified XPath selectors.
func QuerySelectorAll(top *html.Node, selector *xpath.Expr) []*html.Node {
	var elems []*html.Node
	t := selector.Select(CreateXPathNavigator(top))
	for t.MoveNext() {
		nav := t.Current().(*NodeNavigator)
		n := getCurrentNode(nav)
		elems = append(elems, n)
	}
	return elems
}

// LoadURL loads the HTML document from the specified URL.
func LoadURL(url string) (*html.Node, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	return html.Parse(r)
}

// LoadDoc loads the HTML document from the specified file path.
func LoadDoc(path string) (*html.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return html.Parse(bufio.NewReader(f))
}

func getCurrentNode(n *NodeNavigator) *html.Node {
	if n.NodeType() == xpath.AttributeNode {
		childNode := &html.Node{
			Type: html.TextNode,
			Data: n.Value(),
		}
		return &html.Node{
			Type:       html.ElementNode,
			Data:       n.LocalName(),
			FirstChild: childNode,
			LastChild:  childNode,
		}

	}
	return n.curr
}

// Parse returns the parse tree for the HTML from the given Reader.
func Parse(r io.Reader) (*html.Node, error) {
	return html.Parse(r)
}

// InnerText returns the text between the start and end tags of the object.
func InnerText(n *html.Node) string {
	var output func(*strings.Builder, *html.Node)
	output = func(b *strings.Builder, n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
			return
		case html.CommentNode:
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			output(b, child)
		}
	}

	var b strings.Builder
	output(&b, n)
	return b.String()
}

// SelectAttr returns the attribute value with the specified name.
func SelectAttr(n *html.Node, name string) (val string) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode && n.Parent == nil && name == n.Data {
		return InnerText(n)
	}
	for _, attr := range n.Attr {
		if attr.Key == name {
			val = attr.Val
			break
		}
	}
	return
}

// ExistsAttr returns whether attribute with specified name exists.
func ExistsAttr(n *html.Node, name string) bool {
	if n == nil {
		return false
	}
	for _, attr := range n.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
}

// OutputHTML returns the text including tags name.
func OutputHTML(n *html.Node, self bool) string {
	var b strings.Builder
	if self {
		html.Render(&b, n)
	} else {
		for n := n.FirstChild; n != nil; n = n.NextSibling {
			html.Render(&b, n)
		}
	}
	return b.String()
}

type NodeNavigator struct {
	root, curr *html.Node
	attr       int
}

func (h *NodeNavigator) Current() *html.Node {
	return h.curr
}

func (h *NodeNavigator) NodeType() xpath.NodeType {
	switch h.curr.Type {
	case html.CommentNode:
		return xpath.CommentNode
	case html.TextNode:
		return xpath.TextNode
	case html.DocumentNode:
		return xpath.RootNode
	case html.ElementNode:
		if h.attr != -1 {
			return xpath.AttributeNode
		}
		return xpath.ElementNode
	case html.DoctypeNode:
		// ignored <!DOCTYPE HTML> declare and as Root-Node type.
		return xpath.RootNode
	}
	panic(fmt.Sprintf("unknown HTML node type: %v", h.curr.Type))
}

func (h *NodeNavigator) LocalName() string {
	if h.attr != -1 {
		return h.curr.Attr[h.attr].Key
	}
	return h.curr.Data
}

func (*NodeNavigator) Prefix() string {
	return ""
}

func (h *NodeNavigator) Value() string {
	switch h.curr.Type {
	case html.CommentNode:
		return h.curr.Data
	case html.ElementNode:
		if h.attr != -1 {
			return h.curr.Attr[h.attr].Val
		}
		return InnerText(h.curr)
	case html.TextNode:
		return h.curr.Data
	}
	return ""
}

func (h *NodeNavigator) Copy() xpath.NodeNavigator {
	n := *h
	return &n
}

func (h *NodeNavigator) MoveToRoot() {
	h.curr = h.root
}

func (h *NodeNavigator) MoveToParent() bool {
	if h.attr != -1 {
		h.attr = -1
		return true
	} else if node := h.curr.Parent; node != nil {
		h.curr = node
		return true
	}
	return false
}

func (h *NodeNavigator) MoveToNextAttribute() bool {
	if h.attr >= len(h.curr.Attr)-1 {
		return false
	}
	h.attr++
	return true
}

func (h *NodeNavigator) MoveToChild() bool {
	if h.attr != -1 {
		return false
	}
	if node := h.curr.FirstChild; node != nil {
		h.curr = node
		return true
	}
	return false
}

func (h *NodeNavigator) MoveToFirst() bool {
	if h.attr != -1 || h.curr.PrevSibling == nil {
		return false
	}
	for {
		node := h.curr.PrevSibling
		if node == nil {
			break
		}
		h.curr = node
	}
	return true
}

func (h *NodeNavigator) String() string {
	return h.Value()
}

func (h *NodeNavigator) MoveToNext() bool {
	if h.attr != -1 {
		return false
	}
	if node := h.curr.NextSibling; node != nil {
		h.curr = node
		return true
	}
	return false
}

func (h *NodeNavigator) MoveToPrevious() bool {
	if h.attr != -1 {
		return false
	}
	if node := h.curr.PrevSibling; node != nil {
		h.curr = node
		return true
	}
	return false
}

func (h *NodeNavigator) MoveTo(other xpath.NodeNavigator) bool {
	node, ok := other.(*NodeNavigator)
	if !ok || node.root != h.root {
		return false
	}

	h.curr = node.curr
	h.attr = node.attr
	return true
}
