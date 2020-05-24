htmlquery
====
[![Build Status](https://travis-ci.org/antchfx/htmlquery.svg?branch=master)](https://travis-ci.org/antchfx/htmlquery)
[![Coverage Status](https://coveralls.io/repos/github/antchfx/htmlquery/badge.svg?branch=master)](https://coveralls.io/github/antchfx/htmlquery?branch=master)
[![GoDoc](https://godoc.org/github.com/antchfx/htmlquery?status.svg)](https://godoc.org/github.com/antchfx/htmlquery)
[![Go Report Card](https://goreportcard.com/badge/github.com/antchfx/htmlquery)](https://goreportcard.com/report/github.com/antchfx/htmlquery)

Overview
====

`htmlquery` is an XPath query package for HTML, lets you extract data or evaluate from HTML documents by an XPath expression.

`htmlquery` built-in the query object caching feature based on [LRU](https://godoc.org/github.com/golang/groupcache/lru), this feature will caching the recently used XPATH query string. Enable query caching can avoid re-compile XPath expression each query. 

Installation
====

```
go get github.com/antchfx/htmlquery
```

Getting Started
====

#### Query, returns matched elements or error.

```go
nodes, err := htmlquery.QueryAll(doc, "//a")
if err != nil {
	panic(`not a valid XPath expression.`)
}
```

#### Load HTML document from URL.

```go
doc, err := htmlquery.LoadURL("http://example.com/")
```

#### Load HTML from document.

```go
filePath := "/home/user/sample.html"
doc, err := htmlquery.LoadDoc(filePath)
```

#### Load HTML document from string.

```go
s := `<html>....</html>`
doc, err := htmlquery.Parse(strings.NewReader(s))
```

#### Find all A elements.

```go
list := htmlquery.Find(doc, "//a")
```

#### Find all A elements that have `href` attribute.

```go
list := range htmlquery.Find(doc, "//a[@href]")	
```

#### Find all A elements with `href` attribute and only return `href` value.

```go
list := range htmlquery.Find(doc, "//a/@href")	
for n := range list{
	fmt.Println(htmlquery.InnerText(n)) // output @href value without A element.
}
```

### Find the third A element.

```go
a := htmlquery.FindOne(doc, "//a[3]")
```

#### Evaluate the number of all IMG element.

```go
expr, _ := xpath.Compile("count(//img)")
v := expr.Evaluate(htmlquery.CreateXPathNavigator(doc)).(float64)
fmt.Printf("total count is %f", v)
```


FAQ
====

#### `Find()` vs `QueryAll()`, which is better?

`Find` and `QueryAll` both do the same things, searches all of matched html nodes.
The `Find` will panics if you give an error XPath query, but `QueryAll` will return an error for you.

#### Can I save my query expression object for the next query?

Yes, you can. We offer the `QuerySelector` and `QuerySelectorAll` methods, It will accept your query expression object.

Cache a query expression object(or reused) will avoid re-compile XPath query expression, improve your query performance.

#### XPath query object cache performance

```
goos: windows
goarch: amd64
pkg: github.com/antchfx/htmlquery
BenchmarkSelectorCache-4                20000000                55.2 ns/op
BenchmarkDisableSelectorCache-4           500000              3162 ns/op
```

#### How to disable caching?

```
htmlquery.DisableSelectorCache = true
```

Changelogs
===

2019-11-19 
- Add built-in query object cache feature, avoid re-compilation for the same query string. [#16](https://github.com/antchfx/htmlquery/issues/16)
- Added LoadDoc [18](https://github.com/antchfx/htmlquery/pull/18)

2019-10-05 
- Add new methods that compatible with invalid XPath expression error: `QueryAll` and `Query`.
- Add `QuerySelector` and `QuerySelectorAll` methods, supported reused your query object.

2019-02-04
- [#7](https://github.com/antchfx/htmlquery/issues/7) Removed deprecated `FindEach()` and `FindEachWithBreak()` methods.

2018-12-28
- Avoid adding duplicate elements to list for `Find()` method. [#6](https://github.com/antchfx/htmlquery/issues/6)

Tutorial
===

```go
func main() {
	doc, err := htmlquery.LoadURL("https://www.bing.com/search?q=golang")
	if err != nil {
		panic(err)
	}
	// Find all news item.
	list, err := htmlquery.QueryAll(doc, "//ol/li")
	if err != nil {
		panic(err)
	}
	for i, n := range list {
		a := htmlquery.FindOne(n, "//a")
		fmt.Printf("%d %s(%s)\n", i, htmlquery.InnerText(a), htmlquery.SelectAttr(a, "href"))
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
Please let me know if you have any questions.
