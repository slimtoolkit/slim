// Copyright 2015 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mangos

import (
	"sync"
	"time"
)

// CondTimed is a condition variable (ala sync.Cond) but inclues a timeout.
type CondTimed struct {
	sync.Cond
}

// WaitRelTimeout is like Wait, but it times out.  The fact that
// it timed out can be determined by checking the return value.  True
// indicates that it woke up without a timeout (signaled another way),
// whereas false indicates a timeout occurred.
func (cv *CondTimed) WaitRelTimeout(when time.Duration) bool {
	timer := time.AfterFunc(when, func() {
		cv.L.Lock()
		cv.Broadcast()
		cv.L.Unlock()
	})
	cv.Wait()
	return timer.Stop()
}

// WaitAbsTimeout is like WaitRelTimeout, but expires on an absolute time
// instead of a relative one.
func (cv *CondTimed) WaitAbsTimeout(when time.Time) bool {
	now := time.Now()
	if when.After(now) {
		return cv.WaitRelTimeout(when.Sub(now))
	}
	return cv.WaitRelTimeout(time.Duration(0))
}

// Waiter is a way to wait for completion, but it includes a timeout.  It
// is similar in some respects to sync.WaitGroup.
type Waiter struct {
	cv  CondTimed
	cnt int
	sync.Mutex
}

// Init must be called to initialize the Waiter.
func (w *Waiter) Init() {
	w.cv.L = w
	w.cnt = 0
}

// Add adds a new go routine/item to wait for. This should be called before
// starting go routines you want to wait for, for example.
func (w *Waiter) Add() {
	w.Lock()
	w.cnt++
	w.Unlock()
}

// Done is called when the item to wait for is done. There should be a one to
// one correspondance between Add and Done.  When the count drops to zero,
// any callers blocked in Wait() are woken.  If the count drops below zero,
// it panics.
func (w *Waiter) Done() {
	w.Lock()
	w.cnt--
	if w.cnt < 0 {
		panic("wait count dropped < 0")
	}
	if w.cnt == 0 {
		w.cv.Broadcast()
	}
	w.Unlock()
}

// Wait waits without a timeout.  It only completes when the count drops
// to zero.
func (w *Waiter) Wait() {
	w.Lock()
	for w.cnt != 0 {
		w.cv.Wait()
	}
	w.Unlock()
}

// WaitRelTimeout waits until either the count drops to zero, or the timeout
// expires.  It returns true if the count is zero, false otherwise.
func (w *Waiter) WaitRelTimeout(d time.Duration) bool {
	w.Lock()
	for w.cnt != 0 {
		if !w.cv.WaitRelTimeout(d) {
			break
		}
	}
	done := w.cnt == 0
	w.Unlock()
	return done
}

// WaitAbsTimeout is like WaitRelTimeout, but waits until an absolute time.
func (w *Waiter) WaitAbsTimeout(t time.Time) bool {
	w.Lock()
	for w.cnt != 0 {
		if !w.cv.WaitAbsTimeout(t) {
			break
		}
	}
	done := w.cnt == 0
	w.Unlock()
	return done
}
