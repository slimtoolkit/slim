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
	"math/rand"
	"sync"
	"time"
)

var pipes struct {
	byid   map[uint32]*pipe
	nextid uint32
	sync.Mutex
}

// pipe wraps the Pipe data structure with the stuff we need to keep
// for the core.  It implements the Endpoint interface.
type pipe struct {
	pipe   Pipe
	closeq chan struct{} // only closed, never passes data
	id     uint32
	index  int // index in master list of pipes for socket

	l       *listener
	d       *dialer
	sock    *socket
	closing bool // true if we were closed

	sync.Mutex
}

func init() {
	pipes.byid = make(map[uint32]*pipe)
	pipes.nextid = uint32(rand.NewSource(time.Now().UnixNano()).Int63())
}

func newPipe(tranpipe Pipe) *pipe {
	p := &pipe{pipe: tranpipe, index: -1}
	p.closeq = make(chan struct{})
	for {
		pipes.Lock()
		p.id = pipes.nextid & 0x7fffffff
		pipes.nextid++
		if p.id != 0 && pipes.byid[p.id] == nil {
			pipes.byid[p.id] = p
			pipes.Unlock()
			break
		}
		pipes.Unlock()
	}
	return p
}

func (p *pipe) GetID() uint32 {
	return p.id
}

func (p *pipe) Close() error {
	var hook PortHook
	p.Lock()
	sock := p.sock
	if sock != nil {
		hook = sock.porthook
	}
	if p.closing {
		p.Unlock()
		return nil
	}
	p.closing = true
	p.Unlock()
	close(p.closeq)
	if sock != nil {
		sock.remPipe(p)
	}
	p.pipe.Close()
	pipes.Lock()
	delete(pipes.byid, p.id)
	pipes.Unlock()
	if hook != nil {
		hook(PortActionRemove, p)
	}
	return nil
}

func (p *pipe) SendMsg(msg *Message) error {

	if err := p.pipe.Send(msg); err != nil {
		p.Close()
		return err
	}
	return nil
}

func (p *pipe) RecvMsg() *Message {

	msg, err := p.pipe.Recv()
	if err != nil {
		p.Close()
		return nil
	}
	msg.Port = p
	return msg
}

func (p *pipe) Address() string {
	switch {
	case p.l != nil:
		return p.l.Address()
	case p.d != nil:
		return p.d.Address()
	}
	return ""
}

func (p *pipe) GetProp(name string) (interface{}, error) {
	return p.pipe.GetProp(name)
}

func (p *pipe) IsOpen() bool {
	return p.pipe.IsOpen()
}

func (p *pipe) IsClient() bool {
	return p.d != nil
}

func (p *pipe) IsServer() bool {
	return p.l != nil
}

func (p *pipe) LocalProtocol() uint16 {
	return p.pipe.LocalProtocol()
}

func (p *pipe) RemoteProtocol() uint16 {
	return p.pipe.RemoteProtocol()
}

func (p *pipe) Dialer() Dialer {
	return p.d
}

func (p *pipe) Listener() Listener {
	return p.l
}
