package xmlquery

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// A NodeType is the type of a Node.
type NodeType uint

const (
	// DocumentNode is a document object that, as the root of the document tree,
	// provides access to the entire XML document.
	DocumentNode NodeType = iota
	// DeclarationNode is the document type declaration, indicated by the following
	// tag (for example, <!DOCTYPE...> ).
	DeclarationNode
	// ElementNode is an element (for example, <item> ).
	ElementNode
	// TextNode is the text content of a node.
	TextNode
	// CharDataNode node <![CDATA[content]]>
	CharDataNode
	// CommentNode a comment (for example, <!-- my comment --> ).
	CommentNode
	// AttributeNode is an attribute of element.
	AttributeNode
)

// A Node consists of a NodeType and some Data (tag name for
// element nodes, content for text) and are part of a tree of Nodes.
type Node struct {
	Parent, FirstChild, LastChild, PrevSibling, NextSibling *Node

	Type         NodeType
	Data         string
	Prefix       string
	NamespaceURI string
	Attr         []xml.Attr

	level int // node level in the tree
}

// InnerText returns the text between the start and end tags of the object.
func (n *Node) InnerText() string {
	var output func(*bytes.Buffer, *Node)
	output = func(buf *bytes.Buffer, n *Node) {
		switch n.Type {
		case TextNode, CharDataNode:
			buf.WriteString(n.Data)
		case CommentNode:
		default:
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				output(buf, child)
			}
		}
	}

	var buf bytes.Buffer
	output(&buf, n)
	return buf.String()
}

func (n *Node) sanitizedData(preserveSpaces bool) string {
	if preserveSpaces {
		return strings.Trim(n.Data, "\n\t")
	}
	return strings.TrimSpace(n.Data)
}

func calculatePreserveSpaces(n *Node, pastValue bool) bool {
	if attr := n.SelectAttr("xml:space"); attr == "preserve" {
		return true
	} else if attr == "default" {
		return false
	}
	return pastValue
}

func outputXML(buf *bytes.Buffer, n *Node, preserveSpaces bool) {
	preserveSpaces = calculatePreserveSpaces(n, preserveSpaces)
	switch n.Type {
	case TextNode, CharDataNode:
		xml.EscapeText(buf, []byte(n.sanitizedData(preserveSpaces)))
		return
	case CommentNode:
		buf.WriteString("<!--")
		buf.WriteString(n.Data)
		buf.WriteString("-->")
		return
	case DeclarationNode:
		buf.WriteString("<?" + n.Data)
	default:
		if n.Prefix == "" {
			buf.WriteString("<" + n.Data)
		} else {
			buf.WriteString("<" + n.Prefix + ":" + n.Data)
		}
	}

	for _, attr := range n.Attr {
		if attr.Name.Space != "" {
			buf.WriteString(fmt.Sprintf(` %s:%s=`, attr.Name.Space, attr.Name.Local))
		} else {
			buf.WriteString(fmt.Sprintf(` %s=`, attr.Name.Local))
		}
		buf.WriteByte('"')
		xml.EscapeText(buf, []byte(attr.Value))
		buf.WriteByte('"')
	}
	if n.Type == DeclarationNode {
		buf.WriteString("?>")
	} else {
		buf.WriteString(">")
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		outputXML(buf, child, preserveSpaces)
	}
	if n.Type != DeclarationNode {
		if n.Prefix == "" {
			buf.WriteString(fmt.Sprintf("</%s>", n.Data))
		} else {
			buf.WriteString(fmt.Sprintf("</%s:%s>", n.Prefix, n.Data))
		}
	}
}

// OutputXML returns the text that including tags name.
func (n *Node) OutputXML(self bool) string {
	var buf bytes.Buffer
	if self {
		outputXML(&buf, n, false)
	} else {
		for n := n.FirstChild; n != nil; n = n.NextSibling {
			outputXML(&buf, n, false)
		}
	}

	return buf.String()
}

// AddAttr adds a new attribute specified by 'key' and 'val' to a node 'n'.
func AddAttr(n *Node, key, val string) {
	var attr xml.Attr
	if i := strings.Index(key, ":"); i > 0 {
		attr = xml.Attr{
			Name:  xml.Name{Space: key[:i], Local: key[i+1:]},
			Value: val,
		}
	} else {
		attr = xml.Attr{
			Name:  xml.Name{Local: key},
			Value: val,
		}
	}

	n.Attr = append(n.Attr, attr)
}

// AddChild adds a new node 'n' to a node 'parent' as its last child.
func AddChild(parent, n *Node) {
	n.Parent = parent
	n.NextSibling = nil
	if parent.FirstChild == nil {
		parent.FirstChild = n
		n.PrevSibling = nil
	} else {
		parent.LastChild.NextSibling = n
		n.PrevSibling = parent.LastChild
	}

	parent.LastChild = n
}

// AddSibling adds a new node 'n' as a sibling of a given node 'sibling'.
// Note it is not necessarily true that the new node 'n' would be added
// immediately after 'sibling'. If 'sibling' isn't the last child of its
// parent, then the new node 'n' will be added at the end of the sibling
// chain of their parent.
func AddSibling(sibling, n *Node) {
	for t := sibling.NextSibling; t != nil; t = t.NextSibling {
		sibling = t
	}
	n.Parent = sibling.Parent
	sibling.NextSibling = n
	n.PrevSibling = sibling
	n.NextSibling = nil
	if sibling.Parent != nil {
		sibling.Parent.LastChild = n
	}
}

// RemoveFromTree removes a node and its subtree from the document
// tree it is in. If the node is the root of the tree, then it's no-op.
func RemoveFromTree(n *Node) {
	if n.Parent == nil {
		return
	}
	if n.Parent.FirstChild == n {
		if n.Parent.LastChild == n {
			n.Parent.FirstChild = nil
			n.Parent.LastChild = nil
		} else {
			n.Parent.FirstChild = n.NextSibling
			n.NextSibling.PrevSibling = nil
		}
	} else {
		if n.Parent.LastChild == n {
			n.Parent.LastChild = n.PrevSibling
			n.PrevSibling.NextSibling = nil
		} else {
			n.PrevSibling.NextSibling = n.NextSibling
			n.NextSibling.PrevSibling = n.PrevSibling
		}
	}
	n.Parent = nil
	n.PrevSibling = nil
	n.NextSibling = nil
}
