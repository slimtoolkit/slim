package dockerimage

import (
	"container/heap"
)

type TopObjects []*ObjectMetadata

func NewTopObjects(n int) TopObjects {
	if n < 1 {
		n = 1
	}
	n++
	return make(TopObjects, 0, n)
}

func (to TopObjects) Len() int { return len(to) }

func (to TopObjects) Less(i, j int) bool {
	return to[i].Size < to[j].Size
}

func (to TopObjects) Swap(i, j int) {
	to[i], to[j] = to[j], to[i]
}

func (to *TopObjects) Push(x interface{}) {
	item := x.(*ObjectMetadata)
	*to = append(*to, item)
}

func (to *TopObjects) Pop() interface{} {
	old := *to
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*to = old[0 : n-1]
	return item
}

func (to TopObjects) List() []*ObjectMetadata {
	list := []*ObjectMetadata{}
	for len(to) > 0 {
		item := heap.Pop(&to).(*ObjectMetadata)
		list = append([]*ObjectMetadata{item}, list...)
	}

	return list
}
