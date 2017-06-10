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

// Package req implements the REQ protocol, which is the request side of
// the request/response pattern.  (REP is the response.)
package req

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/go-mangos/mangos"
)

// req is an implementation of the req protocol.
type req struct {
	sync.Mutex
	sock   mangos.ProtocolSocket
	eps    map[uint32]*reqEp
	resend chan *mangos.Message
	raw    bool
	retry  time.Duration
	nextid uint32
	waker  *time.Timer
	w      mangos.Waiter
	init   sync.Once

	// fields describing the outstanding request
	reqmsg *mangos.Message
	reqid  uint32
}

type reqEp struct {
	ep mangos.Endpoint
	cq chan struct{}
}

func (r *req) Init(socket mangos.ProtocolSocket) {
	r.sock = socket
	r.eps = make(map[uint32]*reqEp)
	r.resend = make(chan *mangos.Message)
	r.w.Init()

	r.nextid = uint32(time.Now().UnixNano()) // quasi-random
	r.retry = time.Minute * 1                // retry after a minute
	r.waker = time.NewTimer(r.retry)
	r.waker.Stop()
	r.sock.SetRecvError(mangos.ErrProtoState)
}

func (r *req) Shutdown(expire time.Time) {
	r.w.WaitAbsTimeout(expire)
}

// nextID returns the next request ID.
func (r *req) nextID() uint32 {
	// The high order bit is "special", and must always be set.  (This is
	// how the peer will detect the end of the backtrace.)
	v := r.nextid | 0x80000000
	r.nextid++
	return v
}

// resend sends the request message again, after a timer has expired.
func (r *req) resender() {

	defer r.w.Done()
	cq := r.sock.CloseChannel()

	for {
		select {
		case <-r.waker.C:
		case <-cq:
			return
		}

		r.Lock()
		m := r.reqmsg
		if m == nil {
			r.Unlock()
			continue
		}
		m = m.Dup()
		r.Unlock()

		r.resend <- m
		r.Lock()
		if r.retry > 0 {
			r.waker.Reset(r.retry)
		} else {
			r.waker.Stop()
		}
		r.Unlock()
	}
}

func (r *req) receiver(ep mangos.Endpoint) {
	rq := r.sock.RecvChannel()
	cq := r.sock.CloseChannel()

	for {
		m := ep.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		select {
		case rq <- m:
		case <-cq:
			m.Free()
			break
		}
	}
}

func (r *req) sender(pe *reqEp) {

	// NB: Because this function is only called when an endpoint is
	// added, we can reasonably safely cache the channels -- they won't
	// be changing after this point.

	defer r.w.Done()
	sq := r.sock.SendChannel()
	cq := r.sock.CloseChannel()
	rq := r.resend

	for {
		var m *mangos.Message

		select {
		case m = <-rq:
		case m = <-sq:
		case <-cq:
			return
		case <-pe.cq:
			return
		}

		if pe.ep.SendMsg(m) != nil {
			r.resend <- m
			break
		}
	}
}

func (*req) Number() uint16 {
	return mangos.ProtoReq
}

func (*req) PeerNumber() uint16 {
	return mangos.ProtoRep
}

func (*req) Name() string {
	return "req"
}

func (*req) PeerName() string {
	return "rep"
}

func (r *req) AddEndpoint(ep mangos.Endpoint) {

	r.init.Do(func() {
		r.w.Add()
		go r.resender()
	})

	pe := &reqEp{cq: make(chan struct{}), ep: ep}
	r.Lock()
	r.eps[ep.GetID()] = pe
	r.Unlock()
	go r.receiver(ep)
	r.w.Add()
	go r.sender(pe)
}

func (r *req) RemoveEndpoint(ep mangos.Endpoint) {
	id := ep.GetID()
	r.Lock()
	pe := r.eps[id]
	delete(r.eps, id)
	r.Unlock()
	if pe != nil {
		close(pe.cq)
	}
}

func (r *req) SendHook(m *mangos.Message) bool {

	if r.raw {
		// Raw mode has no automatic retry, and must include the
		// request id in the header coming down.
		return true
	}
	r.Lock()
	defer r.Unlock()

	// We need to generate a new request id, and append it to the header.
	r.reqid = r.nextID()
	v := r.reqid
	m.Header = append(m.Header,
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

	r.reqmsg = m.Dup()

	// Schedule a retry, in case we don't get a reply.
	if r.retry > 0 {
		r.waker.Reset(r.retry)
	} else {
		r.waker.Stop()
	}

	r.sock.SetRecvError(nil)

	return true
}

func (r *req) RecvHook(m *mangos.Message) bool {
	if r.raw {
		// Raw mode just passes up messages unmolested.
		return true
	}
	r.Lock()
	defer r.Unlock()
	if len(m.Header) < 4 {
		return false
	}
	if r.reqmsg == nil {
		return false
	}
	if binary.BigEndian.Uint32(m.Header) != r.reqid {
		return false
	}
	r.waker.Stop()
	r.reqmsg.Free()
	r.reqmsg = nil
	r.sock.SetRecvError(mangos.ErrProtoState)
	return true
}

func (r *req) SetOption(option string, value interface{}) error {
	var ok bool
	switch option {
	case mangos.OptionRaw:
		if r.raw, ok = value.(bool); !ok {
			return mangos.ErrBadValue
		}
		if r.raw {
			r.sock.SetRecvError(nil)
		} else {
			r.sock.SetRecvError(mangos.ErrProtoState)
		}
		return nil
	case mangos.OptionRetryTime:
		r.Lock()
		r.retry, ok = value.(time.Duration)
		r.Unlock()
		if !ok {
			return mangos.ErrBadValue
		}
		return nil
	default:
		return mangos.ErrBadOption
	}
}

func (r *req) GetOption(option string) (interface{}, error) {
	switch option {
	case mangos.OptionRaw:
		return r.raw, nil
	case mangos.OptionRetryTime:
		r.Lock()
		v := r.retry
		r.Unlock()
		return v, nil
	default:
		return nil, mangos.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (mangos.Socket, error) {
	return mangos.MakeSocket(&req{}), nil
}
