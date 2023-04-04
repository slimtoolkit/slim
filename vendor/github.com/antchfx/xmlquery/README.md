xmlquery
====
[![Build Status](https://travis-ci.org/antchfx/xmlquery.svg?branch=master)](https://travis-ci.org/antchfx/xmlquery)
[![Coverage Status](https://coveralls.io/repos/github/antchfx/xmlquery/badge.svg?branch=master)](https://coveralls.io/github/antchfx/xmlquery?branch=master)
[![GoDoc](https://godoc.org/github.com/antchfx/xmlquery?status.svg)](https://godoc.org/github.com/antchfx/xmlquery)
[![Go Report Card](https://goreportcard.com/badge/github.com/antchfx/xmlquery)](https://goreportcard.com/report/github.com/antchfx/xmlquery)

Overview
===

`xmlquery` is an XPath query package for XML document, lets you extract data or evaluate from XML documents by an XPath expression.

`xmlquery` built-in the query object caching feature will caching the recently used XPATH query string. Enable caching can avoid re-compile XPath expression each query. 

Change Logs
===

2020-08-??
- Add XML stream loading and parsing support.

2019-11-11 
- Add XPath query caching.

2019-10-05 
- Add new methods that compatible with invalid XPath expression error: `QueryAll` and `Query`.
- Add `QuerySelector` and `QuerySelectorAll` methods, supported reused your query object.
- PR [#12](https://github.com/antchfx/xmlquery/pull/12) (Thanks @FrancescoIlario)
- PR [#11](https://github.com/antchfx/xmlquery/pull/11) (Thanks @gjvnq)

2018-12-23
- added XML output will including comment node. [#9](https://github.com/antchfx/xmlquery/issues/9)

2018-12-03
- added support attribute name with namespace prefix and XML output. [#6](https://github.com/antchfx/xmlquery/issues/6)

Installation
====
```
 $ go get github.com/antchfx/xmlquery
```

Getting Started
===

### Find specified XPath query.

```go
list, err := xmlquery.QueryAll(doc, "a")
if err != nil {
	panic(err)
}
```

#### Parse an XML from URL.

```go
doc, err := xmlquery.LoadURL("http://www.example.com/sitemap.xml")
```

#### Parse an XML from string.

```go
s := `<?xml version="1.0" encoding="utf-8"?><rss version="2.0"></rss>`
doc, err := xmlquery.Parse(strings.NewReader(s))
```

#### Parse an XML from io.Reader.

```go
f, err := os.Open("../books.xml")
doc, err := xmlquery.Parse(f)
```

#### Parse an XML in a stream fashion (simple case without element filtering).

```go
f, err := os.Open("../books.xml")
p, err := xmlquery.CreateStreamParser(f, "/bookstore/book")
for {
	n, err := p.Read()
	if err == io.EOF {
		break
	}
	if err != nil {
		...
	}
}
```

#### Parse an XML in a stream fashion (simple case advanced element filtering).

```go
f, err := os.Open("../books.xml")
p, err := xmlquery.CreateStreamParser(f, "/bookstore/book", "/bookstore/book[price>=10]")
for {
	n, err := p.Read()
	if err == io.EOF {
		break
	}
	if err != nil {
		...
	}
}
```

#### Find authors of all books in the bookstore.

```go
list := xmlquery.Find(doc, "//book//author")
// or
list := xmlquery.Find(doc, "//author")
```

#### Find the second book.

```go
book := xmlquery.FindOne(doc, "//book[2]")
```

#### Find all book elements and only get `id` attribute self. (New Feature)

```go
list := xmlquery.Find(doc,"//book/@id")
```

#### Find all books with id is bk104.

```go
list := xmlquery.Find(doc, "//book[@id='bk104']")
```

#### Find all books that price less than 5.

```go
list := xmlquery.Find(doc, "//book[price<5]")
```

#### Evaluate the total price of all books.

```go
expr, err := xpath.Compile("sum(//book/price)")
price := expr.Evaluate(xmlquery.CreateXPathNavigator(doc)).(float64)
fmt.Printf("total price: %f\n", price)
```

#### Evaluate the number of all books element.

```go
expr, err := xpath.Compile("count(//book)")
price := expr.Evaluate(xmlquery.CreateXPathNavigator(doc)).(float64)
```

FAQ
====

#### `Find()` vs `QueryAll()`, which is better?

`Find` and `QueryAll` both do the same things, searches all of matched html nodes.
The `Find` will panics if you give an error XPath query, but `QueryAll` will return an error for you.

#### Can I save my query expression object for the next query?

Yes, you can. We offer the `QuerySelector` and `QuerySelectorAll` methods, It will accept your query expression object.

Cache a query expression object(or reused) will avoid re-compile XPath query expression, improve your query performance.

#### Create XML document.

```go
doc := &xmlquery.Node{
	Type: xmlquery.DeclarationNode,
	Data: "xml",
	Attr: []xml.Attr{
		xml.Attr{Name: xml.Name{Local: "version"}, Value: "1.0"},
	},
}
root := &xmlquery.Node{
	Data: "rss",
	Type: xmlquery.ElementNode,
}
doc.FirstChild = root
channel := &xmlquery.Node{
	Data: "channel",
	Type: xmlquery.ElementNode,
}
root.FirstChild = channel
title := &xmlquery.Node{
	Data: "title",
	Type: xmlquery.ElementNode,
}
title_text := &xmlquery.Node{
	Data: "W3Schools Home Page",
	Type: xmlquery.TextNode,
}
title.FirstChild = title_text
channel.FirstChild = title
fmt.Println(doc.OutputXML(true))
// <?xml version="1.0"?><rss><channel><title>W3Schools Home Page</title></channel></rss>
```

Quick Tutorial
===

```go
import (
	"github.com/antchfx/xmlquery"
)

func main(){
	s := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
  <title>W3Schools Home Page</title>
  <link>https://www.w3schools.com</link>
  <description>Free web building tutorials</description>
  <item>
    <title>RSS Tutorial</title>
    <link>https://www.w3schools.com/xml/xml_rss.asp</link>
    <description>New RSS tutorial on W3Schools</description>
  </item>
  <item>
    <title>XML Tutorial</title>
    <link>https://www.w3schools.com/xml</link>
    <description>New XML tutorial on W3Schools</description>
  </item>
</channel>
</rss>`

	doc, err := xmlquery.Parse(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	channel := xmlquery.FindOne(doc, "//channel")
	if n := channel.SelectElement("title"); n != nil {
		fmt.Printf("title: %s\n", n.InnerText())
	}
	if n := channel.SelectElement("link"); n != nil {
		fmt.Printf("link: %s\n", n.InnerText())
	}
	for i, n := range xmlquery.Find(doc, "//item/title") {
		fmt.Printf("#%d %s\n", i, n.InnerText())
	}
}
```

List of supported XPath query packages
===
| Name                                              | Description                               |
| ------------------------------------------------- | ----------------------------------------- |
| [htmlquery](https://github.com/antchfx/htmlquery) | XPath query package for the HTML document |
| [xmlquery](https://github.com/antchfx/xmlquery)   | XPath query package for the XML document  |
| [jsonquery](https://github.com/antchfx/jsonquery) | XPath query package for the JSON document |

 Questions
===
Please let me know if you have any questions
