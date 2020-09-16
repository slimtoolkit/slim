package acounter

import (
	"sync/atomic"
)

type Type struct {
	val uint64
}

func (self *Type) Value() uint64 {
	return atomic.LoadUint64(&self.val)
}

func (self *Type) Inc() uint64 {
	return self.Add(1)
}

func (self *Type) Add(val uint64) uint64 {
	return atomic.AddUint64(&self.val, val)
}
