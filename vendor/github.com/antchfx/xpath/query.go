package xpath

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"reflect"
	"strconv"
)

// The return type of the XPath expression.
type resultType int

var xpathResultType = struct {
	Boolean resultType
	// A numeric value
	Number resultType
	String resultType
	// A node collection.
	NodeSet resultType
	// Any of the XPath node types.
	Any resultType
}{
	Boolean: 0,
	Number:  1,
	String:  2,
	NodeSet: 3,
	Any:     4,
}

type queryProp int

var queryProps = struct {
	None     queryProp
	Position queryProp
	Count    queryProp
	Cached   queryProp
	Reverse  queryProp
	Merge    queryProp
}{
	None:     0,
	Position: 1,
	Count:    2,
	Cached:   4,
	Reverse:  8,
	Merge:    16,
}

type iterator interface {
	Current() NodeNavigator
}

// An XPath query interface.
type query interface {
	// Select traversing iterator returns a query matched node NodeNavigator.
	Select(iterator) NodeNavigator

	// Evaluate evaluates query and returns values of the current query.
	Evaluate(iterator) interface{}

	Clone() query

	// ValueType returns the value type of the current query.
	ValueType() resultType

	Properties() queryProp
}

// nopQuery is an empty query that always return nil for any query.
type nopQuery struct{}

func (nopQuery) Select(iterator) NodeNavigator { return nil }

func (nopQuery) Evaluate(iterator) interface{} { return nil }

func (nopQuery) Clone() query { return nopQuery{} }

func (nopQuery) ValueType() resultType { return xpathResultType.NodeSet }

func (nopQuery) Properties() queryProp {
	return queryProps.Merge | queryProps.Position | queryProps.Count | queryProps.Cached
}

// contextQuery is returns current node on the iterator object query.
type contextQuery struct {
	count int
}

func (c *contextQuery) Select(t iterator) NodeNavigator {
	if c.count > 0 {
		return nil
	}
	c.count++
	return t.Current().Copy()
}

func (c *contextQuery) Evaluate(iterator) interface{} {
	c.count = 0
	return c
}

func (c *contextQuery) Clone() query {
	return &contextQuery{}
}

func (c *contextQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (c *contextQuery) Properties() queryProp {
	return queryProps.Merge | queryProps.Position | queryProps.Count | queryProps.Cached
}

type absoluteQuery struct {
	count int
}

func (a *absoluteQuery) Select(t iterator) (n NodeNavigator) {
	if a.count > 0 {
		return
	}
	a.count++
	n = t.Current().Copy()
	n.MoveToRoot()
	return
}

func (a *absoluteQuery) Evaluate(t iterator) interface{} {
	a.count = 0
	return a
}

func (a *absoluteQuery) Clone() query {
	return &absoluteQuery{}
}

func (a *absoluteQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (a *absoluteQuery) Properties() queryProp {
	return queryProps.Merge | queryProps.Position | queryProps.Count | queryProps.Cached
}

// ancestorQuery is an XPath ancestor node query.(ancestor::*|ancestor-self::*)
type ancestorQuery struct {
	name     string
	iterator func() NodeNavigator
	table    map[uint64]bool
	pos      int

	Self      bool
	Input     query
	Predicate func(NodeNavigator) bool
}

func (a *ancestorQuery) Select(t iterator) NodeNavigator {
	if a.table == nil {
		a.table = make(map[uint64]bool)
	}

	for {
		if a.iterator == nil {
			node := a.Input.Select(t)
			if node == nil {
				return nil
			}
			// Reset position for a new input context node
			a.pos = 0
			first := true
			node = node.Copy()
			a.iterator = func() NodeNavigator {
				if first {
					first = false
					if a.Self && a.Predicate(node) {
						return node
					}
				}
				for node.MoveToParent() {
					if a.Predicate(node) {
						return node
					}
				}
				return nil
			}
		}

		for node := a.iterator(); node != nil; node = a.iterator() {
			node_id := getHashCode(node.Copy())
			if _, ok := a.table[node_id]; !ok {
				a.table[node_id] = true
				// Increase position for each matched node in current input context
				a.pos++
				return node
			}
		}
		a.iterator = nil
	}
}

func (a *ancestorQuery) Evaluate(t iterator) interface{} {
	a.Input.Evaluate(t)
	a.iterator = nil
	// Reset the table when re-evaluating to ensure clean state
	a.table = nil
	return a
}

func (a *ancestorQuery) Test(n NodeNavigator) bool {
	return a.Predicate(n)
}

func (a *ancestorQuery) Clone() query {
	return &ancestorQuery{name: a.name, Self: a.Self, Input: a.Input.Clone(), Predicate: a.Predicate}
}

func (a *ancestorQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (a *ancestorQuery) Properties() queryProp {
	return queryProps.Position | queryProps.Count | queryProps.Cached | queryProps.Merge | queryProps.Reverse
}

// position returns the ordinal of the current matched node within the axis
// traversal for the current input context node. This is required so numeric
// predicates like [1] or [2] on the ancestor axis resolve in axis order.
func (a *ancestorQuery) position() int {
	return a.pos
}

// attributeQuery is an XPath attribute node query.(@*)
type attributeQuery struct {
	name     string
	iterator func() NodeNavigator

	Input     query
	Predicate func(NodeNavigator) bool
}

func (a *attributeQuery) Select(t iterator) NodeNavigator {
	for {
		if a.iterator == nil {
			node := a.Input.Select(t)
			if node == nil {
				return nil
			}
			node = node.Copy()
			a.iterator = func() NodeNavigator {
				for {
					onAttr := node.MoveToNextAttribute()
					if !onAttr {
						return nil
					}
					if a.Predicate(node) {
						return node
					}
				}
			}
		}

		if node := a.iterator(); node != nil {
			return node
		}
		a.iterator = nil
	}
}

func (a *attributeQuery) Evaluate(t iterator) interface{} {
	a.Input.Evaluate(t)
	a.iterator = nil
	return a
}

func (a *attributeQuery) Test(n NodeNavigator) bool {
	return a.Predicate(n)
}

func (a *attributeQuery) Clone() query {
	return &attributeQuery{name: a.name, Input: a.Input.Clone(), Predicate: a.Predicate}
}

func (a *attributeQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (a *attributeQuery) Properties() queryProp {
	return queryProps.Merge
}

// childQuery is an XPath child node query.(child::*)
type childQuery struct {
	name     string
	posit    int
	iterator func() NodeNavigator

	Input     query
	Predicate func(NodeNavigator) bool
}

func (c *childQuery) Select(t iterator) NodeNavigator {
	for {
		if c.iterator == nil {
			c.posit = 0
			node := c.Input.Select(t)
			if node == nil {
				return nil
			}
			node = node.Copy()
			first := true
			c.iterator = func() NodeNavigator {
				for {
					if (first && !node.MoveToChild()) || (!first && !node.MoveToNext()) {
						return nil
					}
					first = false
					if c.Predicate(node) {
						return node
					}
				}
			}
		}

		if node := c.iterator(); node != nil {
			c.posit++
			return node
		}
		c.iterator = nil
	}
}

func (c *childQuery) Evaluate(t iterator) interface{} {
	c.Input.Evaluate(t)
	c.iterator = nil
	return c
}

func (c *childQuery) Test(n NodeNavigator) bool {
	return c.Predicate(n)
}

func (c *childQuery) Clone() query {
	return &childQuery{name: c.name, Input: c.Input.Clone(), Predicate: c.Predicate}
}

func (c *childQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (c *childQuery) Properties() queryProp {
	return queryProps.Merge
}

// position returns a position of current NodeNavigator.
func (c *childQuery) position() int {
	return c.posit
}

type cachedChildQuery struct {
	name     string
	posit    int
	iterator func() NodeNavigator

	Input     query
	Predicate func(NodeNavigator) bool
}

func (c *cachedChildQuery) Select(t iterator) NodeNavigator {
	for {
		if c.iterator == nil {
			c.posit = 0
			node := c.Input.Select(t)
			if node == nil {
				return nil
			}
			node = node.Copy()
			first := true
			c.iterator = func() NodeNavigator {
				for {
					if (first && !node.MoveToChild()) || (!first && !node.MoveToNext()) {
						return nil
					}
					first = false
					if c.Predicate(node) {
						return node
					}
				}
			}
		}

		if node := c.iterator(); node != nil {
			c.posit++
			return node
		}
		c.iterator = nil
	}
}

func (c *cachedChildQuery) Evaluate(t iterator) interface{} {
	c.Input.Evaluate(t)
	c.iterator = nil
	return c
}

func (c *cachedChildQuery) position() int {
	return c.posit
}

func (c *cachedChildQuery) Test(n NodeNavigator) bool {
	return c.Predicate(n)
}

func (c *cachedChildQuery) Clone() query {
	return &childQuery{name: c.name, Input: c.Input.Clone(), Predicate: c.Predicate}
}

func (c *cachedChildQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (c *cachedChildQuery) Properties() queryProp {
	return queryProps.Merge
}

// descendantQuery is an XPath descendant node query.(descendant::* | descendant-or-self::*)
type descendantQuery struct {
	name     string
	iterator func() NodeNavigator
	posit    int
	level    int

	Self      bool
	Input     query
	Predicate func(NodeNavigator) bool
}

func (d *descendantQuery) Select(t iterator) NodeNavigator {
	for {
		if d.iterator == nil {
			d.posit = 0
			node := d.Input.Select(t)
			if node == nil {
				return nil
			}
			node = node.Copy()
			d.level = 0
			first := true
			d.iterator = func() NodeNavigator {
				if first {
					first = false
					if d.Self && d.Predicate(node) {
						return node
					}
				}

				for {
					if node.MoveToChild() {
						d.level = d.level + 1
					} else {
						for {
							if d.level == 0 {
								return nil
							}
							if node.MoveToNext() {
								break
							}
							node.MoveToParent()
							d.level = d.level - 1
						}
					}
					if d.Predicate(node) {
						return node
					}
				}
			}
		}

		if node := d.iterator(); node != nil {
			d.posit++
			return node
		}
		d.iterator = nil
	}
}

func (d *descendantQuery) Evaluate(t iterator) interface{} {
	d.Input.Evaluate(t)
	d.iterator = nil
	return d
}

func (d *descendantQuery) Test(n NodeNavigator) bool {
	return d.Predicate(n)
}

// position returns a position of current NodeNavigator.
func (d *descendantQuery) position() int {
	return d.posit
}

func (d *descendantQuery) depth() int {
	return d.level
}

func (d *descendantQuery) Clone() query {
	return &descendantQuery{name: d.name, Self: d.Self, Input: d.Input.Clone(), Predicate: d.Predicate}
}

func (d *descendantQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (d *descendantQuery) Properties() queryProp {
	return queryProps.Merge
}

// followingQuery is an XPath following node query.(following::*|following-sibling::*)
type followingQuery struct {
	posit    int
	iterator func() NodeNavigator

	Input     query
	Sibling   bool // The matching sibling node of current node.
	Predicate func(NodeNavigator) bool
}

func (f *followingQuery) Select(t iterator) NodeNavigator {
	for {
		if f.iterator == nil {
			f.posit = 0
			node := f.Input.Select(t)
			if node == nil {
				return nil
			}
			node = node.Copy()
			if f.Sibling {
				f.iterator = func() NodeNavigator {
					for {
						if !node.MoveToNext() {
							return nil
						}
						if f.Predicate(node) {
							f.posit++
							return node
						}
					}
				}
			} else {
				var q *descendantQuery // descendant query
				f.iterator = func() NodeNavigator {
					for {
						if q == nil {
							for !node.MoveToNext() {
								if !node.MoveToParent() {
									return nil
								}
							}
							q = &descendantQuery{
								Self:      true,
								Input:     &contextQuery{},
								Predicate: f.Predicate,
							}
							t.Current().MoveTo(node)
						}
						if node := q.Select(t); node != nil {
							f.posit = q.posit
							return node
						}
						q = nil
					}
				}
			}
		}

		if node := f.iterator(); node != nil {
			return node
		}
		f.iterator = nil
	}
}

func (f *followingQuery) Evaluate(t iterator) interface{} {
	f.Input.Evaluate(t)
	return f
}

func (f *followingQuery) Test(n NodeNavigator) bool {
	return f.Predicate(n)
}

func (f *followingQuery) Clone() query {
	return &followingQuery{Input: f.Input.Clone(), Sibling: f.Sibling, Predicate: f.Predicate}
}

func (f *followingQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (f *followingQuery) Properties() queryProp {
	return queryProps.Merge
}

func (f *followingQuery) position() int {
	return f.posit
}

// precedingQuery is an XPath preceding node query.(preceding::*)
type precedingQuery struct {
	iterator  func() NodeNavigator
	posit     int
	Input     query
	Sibling   bool // The matching sibling node of current node.
	Predicate func(NodeNavigator) bool
}

func (p *precedingQuery) Select(t iterator) NodeNavigator {
	for {
		if p.iterator == nil {
			p.posit = 0
			node := p.Input.Select(t)
			if node == nil {
				return nil
			}
			node = node.Copy()
			if p.Sibling {
				p.iterator = func() NodeNavigator {
					for {
						for !node.MoveToPrevious() {
							return nil
						}
						if p.Predicate(node) {
							p.posit++
							return node
						}
					}
				}
			} else {
				var q query
				p.iterator = func() NodeNavigator {
					for {
						if q == nil {
							for !node.MoveToPrevious() {
								if !node.MoveToParent() {
									return nil
								}
								p.posit = 0
							}
							q = &descendantQuery{
								Self:      true,
								Input:     &contextQuery{},
								Predicate: p.Predicate,
							}
							t.Current().MoveTo(node)
						}
						if node := q.Select(t); node != nil {
							p.posit++
							return node
						}
						q = nil
					}
				}
			}
		}
		if node := p.iterator(); node != nil {
			return node
		}
		p.iterator = nil
	}
}

func (p *precedingQuery) Evaluate(t iterator) interface{} {
	p.Input.Evaluate(t)
	return p
}

func (p *precedingQuery) Test(n NodeNavigator) bool {
	return p.Predicate(n)
}

func (p *precedingQuery) Clone() query {
	return &precedingQuery{Input: p.Input.Clone(), Sibling: p.Sibling, Predicate: p.Predicate}
}

func (p *precedingQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (p *precedingQuery) Properties() queryProp {
	return queryProps.Merge | queryProps.Reverse
}

func (p *precedingQuery) position() int {
	return p.posit
}

// parentQuery is an XPath parent node query.(parent::*)
type parentQuery struct {
	Input     query
	Predicate func(NodeNavigator) bool
}

func (p *parentQuery) Select(t iterator) NodeNavigator {
	for {
		node := p.Input.Select(t)
		if node == nil {
			return nil
		}
		node = node.Copy()
		if node.MoveToParent() && p.Predicate(node) {
			return node
		}
	}
}

func (p *parentQuery) Evaluate(t iterator) interface{} {
	p.Input.Evaluate(t)
	return p
}

func (p *parentQuery) Clone() query {
	return &parentQuery{Input: p.Input.Clone(), Predicate: p.Predicate}
}

func (p *parentQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (p *parentQuery) Properties() queryProp {
	return queryProps.Position | queryProps.Count | queryProps.Cached | queryProps.Merge
}

func (p *parentQuery) Test(n NodeNavigator) bool {
	return p.Predicate(n)
}

// selfQuery is an Self node query.(self::*)
type selfQuery struct {
	Input     query
	Predicate func(NodeNavigator) bool
}

func (s *selfQuery) Select(t iterator) NodeNavigator {
	for {
		node := s.Input.Select(t)
		if node == nil {
			return nil
		}

		if s.Predicate(node) {
			return node
		}
	}
}

func (s *selfQuery) Evaluate(t iterator) interface{} {
	s.Input.Evaluate(t)
	return s
}

func (s *selfQuery) Test(n NodeNavigator) bool {
	return s.Predicate(n)
}

func (s *selfQuery) Clone() query {
	return &selfQuery{Input: s.Input.Clone(), Predicate: s.Predicate}
}

func (s *selfQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (s *selfQuery) Properties() queryProp {
	return queryProps.Merge
}

// filterQuery is an XPath query for predicate filter.
type filterQuery struct {
	Input      query
	Predicate  query
	NoPosition bool

	posit    int
	positmap map[int]int
}

func (f *filterQuery) do(t iterator) bool {
	val := reflect.ValueOf(f.Predicate.Evaluate(t))
	switch val.Kind() {
	case reflect.Bool:
		return val.Bool()
	case reflect.String:
		return len(val.String()) > 0
	case reflect.Float64:
		pt := getNodePosition(f.Input)
		return int(val.Float()) == pt
	default:
		if f.Predicate != nil {
			return f.Predicate.Select(t) != nil
		}
	}
	return false
}

func (f *filterQuery) position() int {
	return f.posit
}

func (f *filterQuery) Select(t iterator) NodeNavigator {
	if f.positmap == nil {
		f.positmap = make(map[int]int)
	}
	for {

		node := f.Input.Select(t)
		if node == nil {
			return nil
		}
		node = node.Copy()

		t.Current().MoveTo(node)
		if f.do(t) {
			// fix https://github.com/antchfx/htmlquery/issues/26
			// Calculate and keep the each of matching node's position in the same depth.
			level := getNodeDepth(f.Input)
			f.positmap[level]++
			f.posit = f.positmap[level]
			return node
		}
	}
}

func (f *filterQuery) Evaluate(t iterator) interface{} {
	f.Input.Evaluate(t)
	// Reset the position map when re-evaluating to ensure clean state
	f.positmap = nil
	return f
}

func (f *filterQuery) Clone() query {
	return &filterQuery{Input: f.Input.Clone(), Predicate: f.Predicate.Clone()}
}

func (f *filterQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (f *filterQuery) Properties() queryProp {
	return (queryProps.Position | f.Input.Properties()) & (queryProps.Reverse | queryProps.Merge)
}

// functionQuery is an XPath function that returns a computed value for
// the Evaluate call of the current NodeNavigator node. Select call isn't
// applicable for functionQuery.
type functionQuery struct {
	Input query                             // Node Set
	Func  func(query, iterator) interface{} // The xpath function.
}

func (f *functionQuery) Select(t iterator) NodeNavigator {
	return nil
}

// Evaluate call a specified function that will returns the
// following value type: number,string,boolean.
func (f *functionQuery) Evaluate(t iterator) interface{} {
	return f.Func(f.Input, t)
}

func (f *functionQuery) Clone() query {
	if f.Input == nil {
		return &functionQuery{Func: f.Func}
	}
	return &functionQuery{Input: f.Input.Clone(), Func: f.Func}
}

func (f *functionQuery) ValueType() resultType {
	return xpathResultType.Any
}

func (f *functionQuery) Properties() queryProp {
	return queryProps.Merge
}

// transformFunctionQuery diffs from functionQuery where the latter computes a scalar
// value (number,string,boolean) for the current NodeNavigator node while the former
// (transformFunctionQuery) performs a mapping or transform of the current NodeNavigator
// and returns a new NodeNavigator. It is used for non-scalar XPath functions such as
// reverse(), remove(), subsequence(), unordered(), etc.
type transformFunctionQuery struct {
	Input    query
	Func     func(query, iterator) func() NodeNavigator
	iterator func() NodeNavigator
}

func (f *transformFunctionQuery) Select(t iterator) NodeNavigator {
	if f.iterator == nil {
		f.iterator = f.Func(f.Input, t)
	}
	return f.iterator()
}

func (f *transformFunctionQuery) Evaluate(t iterator) interface{} {
	f.Input.Evaluate(t)
	f.iterator = nil
	return f
}

func (f *transformFunctionQuery) Clone() query {
	return &transformFunctionQuery{Input: f.Input.Clone(), Func: f.Func}
}

func (f *transformFunctionQuery) ValueType() resultType {
	return xpathResultType.Any
}

func (f *transformFunctionQuery) Properties() queryProp {
	return queryProps.Merge
}

// constantQuery is an XPath constant operand.
type constantQuery struct {
	Val interface{}
}

func (c *constantQuery) Select(t iterator) NodeNavigator {
	return nil
}

func (c *constantQuery) Evaluate(t iterator) interface{} {
	return c.Val
}

func (c *constantQuery) Clone() query {
	return c
}

func (c *constantQuery) ValueType() resultType {
	return getXPathType(c.Val)
}

func (c *constantQuery) Properties() queryProp {
	return queryProps.Position | queryProps.Count | queryProps.Cached | queryProps.Merge
}

type groupQuery struct {
	posit int

	Input query
}

func (g *groupQuery) Select(t iterator) NodeNavigator {
	node := g.Input.Select(t)
	if node == nil {
		return nil
	}
	g.posit++
	return node
}

func (g *groupQuery) Evaluate(t iterator) interface{} {
	return g.Input.Evaluate(t)
}

func (g *groupQuery) Clone() query {
	return &groupQuery{Input: g.Input.Clone()}
}

func (g *groupQuery) ValueType() resultType {
	return g.Input.ValueType()
}

func (g *groupQuery) Properties() queryProp {
	return queryProps.Position
}

func (g *groupQuery) position() int {
	return g.posit
}

// logicalQuery is an XPath logical expression.
type logicalQuery struct {
	Left, Right query

	Do func(iterator, interface{}, interface{}) interface{}
}

func (l *logicalQuery) Select(t iterator) NodeNavigator {
	return nil
}

func (l *logicalQuery) Evaluate(t iterator) interface{} {
	m := l.Left.Evaluate(t)
	n := l.Right.Evaluate(t)
	return l.Do(t, m, n)
}

func (l *logicalQuery) Clone() query {
	return &logicalQuery{Left: l.Left.Clone(), Right: l.Right.Clone(), Do: l.Do}
}

func (l *logicalQuery) ValueType() resultType {
	return xpathResultType.Boolean
}

func (l *logicalQuery) Properties() queryProp {
	return queryProps.Merge
}

// numericQuery is an XPath numeric operator expression.
type numericQuery struct {
	Left, Right query

	Do func(iterator, interface{}, interface{}) interface{}
}

func (n *numericQuery) Select(t iterator) NodeNavigator {
	return nil
}

func (n *numericQuery) Evaluate(t iterator) interface{} {
	m := n.Left.Evaluate(t)
	k := n.Right.Evaluate(t)
	return n.Do(t, m, k)
}

func (n *numericQuery) Clone() query {
	return &numericQuery{Left: n.Left.Clone(), Right: n.Right.Clone(), Do: n.Do}
}

func (n *numericQuery) ValueType() resultType {
	return xpathResultType.Number
}

func (n *numericQuery) Properties() queryProp {
	return queryProps.Merge
}

type booleanQuery struct {
	IsOr        bool
	Left, Right query
	iterator    func() NodeNavigator
}

func (b *booleanQuery) Select(t iterator) NodeNavigator {
	if b.iterator == nil {
		var list []NodeNavigator
		i := 0
		root := t.Current().Copy()
		if b.IsOr {
			for {
				node := b.Left.Select(t)
				if node == nil {
					break
				}
				node = node.Copy()
				list = append(list, node)
			}
			t.Current().MoveTo(root)
			for {
				node := b.Right.Select(t)
				if node == nil {
					break
				}
				node = node.Copy()
				list = append(list, node)
			}
		} else {
			var m []NodeNavigator
			var n []NodeNavigator
			for {
				node := b.Left.Select(t)
				if node == nil {
					break
				}
				node = node.Copy()
				list = append(m, node)
			}
			t.Current().MoveTo(root)
			for {
				node := b.Right.Select(t)
				if node == nil {
					break
				}
				node = node.Copy()
				list = append(n, node)
			}
			for _, k := range m {
				for _, j := range n {
					if k == j {
						list = append(list, k)
					}
				}
			}
		}

		b.iterator = func() NodeNavigator {
			if i >= len(list) {
				return nil
			}
			node := list[i]
			i++
			return node
		}
	}
	return b.iterator()
}

func (b *booleanQuery) Evaluate(t iterator) interface{} {
	n := t.Current().Copy()

	m := b.Left.Evaluate(t)
	left := asBool(t, m)
	if b.IsOr && left {
		return true
	} else if !b.IsOr && !left {
		return false
	}

	t.Current().MoveTo(n)
	m = b.Right.Evaluate(t)
	return asBool(t, m)
}

func (b *booleanQuery) Clone() query {
	return &booleanQuery{IsOr: b.IsOr, Left: b.Left.Clone(), Right: b.Right.Clone()}
}

func (b *booleanQuery) ValueType() resultType {
	return xpathResultType.Boolean
}

func (b *booleanQuery) Properties() queryProp {
	return queryProps.Merge
}

type unionQuery struct {
	Left, Right query
	iterator    func() NodeNavigator
}

func (u *unionQuery) Select(t iterator) NodeNavigator {
	if u.iterator == nil {
		var list []NodeNavigator
		var m = make(map[uint64]bool)
		root := t.Current().Copy()
		for {
			node := u.Left.Select(t)
			if node == nil {
				break
			}
			code := getHashCode(node.Copy())
			if _, ok := m[code]; !ok {
				m[code] = true
				list = append(list, node.Copy())
			}
		}
		t.Current().MoveTo(root)
		for {
			node := u.Right.Select(t)
			if node == nil {
				break
			}
			code := getHashCode(node.Copy())
			if _, ok := m[code]; !ok {
				m[code] = true
				list = append(list, node.Copy())
			}
		}
		var i int
		u.iterator = func() NodeNavigator {
			if i >= len(list) {
				return nil
			}
			node := list[i]
			i++
			return node
		}
	}
	return u.iterator()
}

func (u *unionQuery) Evaluate(t iterator) interface{} {
	u.iterator = nil
	u.Left.Evaluate(t)
	u.Right.Evaluate(t)
	return u
}

func (u *unionQuery) Clone() query {
	return &unionQuery{Left: u.Left.Clone(), Right: u.Right.Clone()}
}

func (u *unionQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (u *unionQuery) Properties() queryProp {
	return queryProps.Merge
}

type lastFuncQuery struct {
	buffer  []NodeNavigator
	counted bool

	Input query
}

func (q *lastFuncQuery) Select(t iterator) NodeNavigator {
	return nil
}

func (q *lastFuncQuery) Evaluate(t iterator) interface{} {
	if !q.counted {
		for {
			node := q.Input.Select(t)
			if node == nil {
				break
			}
			q.buffer = append(q.buffer, node.Copy())
		}
		q.counted = true
	}
	return float64(len(q.buffer))
}

func (q *lastFuncQuery) Clone() query {
	return &lastFuncQuery{Input: q.Input.Clone()}
}

func (q *lastFuncQuery) ValueType() resultType {
	return xpathResultType.Number
}

func (q *lastFuncQuery) Properties() queryProp {
	return queryProps.Merge
}

type descendantOverDescendantQuery struct {
	name        string
	level       int
	posit       int
	currentNode NodeNavigator

	Input     query
	MatchSelf bool
	Predicate func(NodeNavigator) bool
}

func (d *descendantOverDescendantQuery) moveToFirstChild() bool {
	if d.currentNode.MoveToChild() {
		d.level++
		return true
	}
	return false
}

func (d *descendantOverDescendantQuery) moveUpUntilNext() bool {
	for !d.currentNode.MoveToNext() {
		d.level--
		if d.level == 0 {
			return false
		}
		d.currentNode.MoveToParent()
	}
	return true
}

func (d *descendantOverDescendantQuery) Select(t iterator) NodeNavigator {
	for {
		if d.level == 0 {
			node := d.Input.Select(t)
			if node == nil {
				return nil
			}
			d.currentNode = node.Copy()
			d.posit = 0
			if d.MatchSelf && d.Predicate(d.currentNode) {
				d.posit = 1
				return d.currentNode
			}
			d.moveToFirstChild()
		} else if !d.moveUpUntilNext() {
			continue
		}
		for ok := true; ok; ok = d.moveToFirstChild() {
			if d.Predicate(d.currentNode) {
				d.posit++
				return d.currentNode
			}
		}
	}
}

func (d *descendantOverDescendantQuery) Evaluate(t iterator) interface{} {
	d.Input.Evaluate(t)
	return d
}

func (d *descendantOverDescendantQuery) Clone() query {
	return &descendantOverDescendantQuery{Input: d.Input.Clone(), Predicate: d.Predicate, MatchSelf: d.MatchSelf}
}

func (d *descendantOverDescendantQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (d *descendantOverDescendantQuery) Properties() queryProp {
	return queryProps.Merge
}

func (d *descendantOverDescendantQuery) position() int {
	return d.posit
}

type mergeQuery struct {
	Input query
	Child query

	iterator func() NodeNavigator
}

func (m *mergeQuery) Select(t iterator) NodeNavigator {
	for {
		if m.iterator == nil {
			root := m.Input.Select(t)
			if root == nil {
				return nil
			}
			m.Child.Evaluate(t)
			root = root.Copy()
			t.Current().MoveTo(root)
			var list []NodeNavigator
			for node := m.Child.Select(t); node != nil; node = m.Child.Select(t) {
				list = append(list, node.Copy())
			}
			i := 0
			m.iterator = func() NodeNavigator {
				if i >= len(list) {
					return nil
				}
				result := list[i]
				i++
				return result
			}
		}

		if node := m.iterator(); node != nil {
			return node
		}
		m.iterator = nil
	}
}

func (m *mergeQuery) Evaluate(t iterator) interface{} {
	m.Input.Evaluate(t)
	return m
}

func (m *mergeQuery) Clone() query {
	return &mergeQuery{Input: m.Input.Clone(), Child: m.Child.Clone()}
}

func (m *mergeQuery) ValueType() resultType {
	return xpathResultType.NodeSet
}

func (m *mergeQuery) Properties() queryProp {
	return queryProps.Position | queryProps.Count | queryProps.Cached | queryProps.Merge
}

func getHashCode(n NodeNavigator) uint64 {
	var sb bytes.Buffer
	switch n.NodeType() {
	case AttributeNode, TextNode, CommentNode:
		sb.WriteString(n.LocalName())
		sb.WriteByte('=')
		sb.WriteString(n.Value())
		// https://github.com/antchfx/htmlquery/issues/25
		d := 1
		for n.MoveToPrevious() {
			d++
		}
		sb.WriteByte('-')
		sb.WriteString(strconv.Itoa(d))
		for n.MoveToParent() {
			d = 1
			for n.MoveToPrevious() {
				d++
			}
			sb.WriteByte('-')
			sb.WriteString(strconv.Itoa(d))
		}
	case ElementNode:
		sb.WriteString(n.Prefix() + n.LocalName())
		d := 1
		for n.MoveToPrevious() {
			d++
		}
		sb.WriteByte('-')
		sb.WriteString(strconv.Itoa(d))

		for n.MoveToParent() {
			d = 1
			for n.MoveToPrevious() {
				d++
			}
			sb.WriteByte('-')
			sb.WriteString(strconv.Itoa(d))
		}
	}
	h := fnv.New64a()
	h.Write(sb.Bytes())
	return h.Sum64()
}

func getNodePosition(q query) int {
	type Position interface {
		position() int
	}
	if count, ok := q.(Position); ok {
		return count.position()
	}
	return 1
}

func getNodeDepth(q query) int {
	type Depth interface {
		depth() int
	}
	if count, ok := q.(Depth); ok {
		return count.depth()
	}
	return 0
}

func getXPathType(i interface{}) resultType {
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Float64:
		return xpathResultType.Number
	case reflect.String:
		return xpathResultType.String
	case reflect.Bool:
		return xpathResultType.Boolean
	default:
		if _, ok := i.(query); ok {
			return xpathResultType.NodeSet
		}
	}
	panic(fmt.Errorf("xpath unknown value type: %v", v.Kind()))
}
