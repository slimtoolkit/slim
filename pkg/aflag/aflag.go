package aflag

import (
	"sync/atomic"
)

const (
	None uint32 = iota
	Off
	On
)

type Type struct {
	val uint32
}

func (self *Type) Value() uint32 {
	return atomic.LoadUint32(&self.val)
}

func (self *Type) On() {
	self.Set(On)
}

func (self *Type) Off() {
	self.Set(Off)
}

func (self *Type) Set(val uint32) {
	atomic.StoreUint32(&self.val, val)
}

func (self *Type) IsOn() bool {
	return self.Is(On)
}

func (self *Type) IsOff() bool {
	return self.Is(Off)
}

func (self *Type) IsNone() bool {
	return self.Is(None)
}

func (self *Type) Is(val uint32) bool {
	return atomic.LoadUint32(&self.val) == val
}

func (self *Type) Has(val uint32) bool {
	return (atomic.LoadUint32(&self.val) & val) == val
}
