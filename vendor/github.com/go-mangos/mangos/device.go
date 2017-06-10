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

// Device is used to create a forwarding loop between two sockets.  If the
// same socket is listed (or either socket is nil), then a loopback device
// is established instead.  Note that the single socket case is only valid
// for protocols where the underlying protocol can peer for itself (e.g. PAIR,
// or BUS, but not REQ/REP or PUB/SUB!)  Both sockets will be placed into RAW
// mode.
//
// If the plumbing is successful, nil will be returned.  Two threads will be
// established to forward messages in each direction.  If either socket returns
// error on receive or send, the goroutine doing the forwarding will exit.
// This means that closing either socket will generally cause the goroutines
// to exit.  Apart from closing the socket(s), no further operations should be
// performed against the socket.
func Device(s1 Socket, s2 Socket) error {
	// Is one of the sockets nil?
	if s1 == nil {
		s1 = s2
	}
	if s2 == nil {
		s2 = s1
	}
	// At least one must be non-nil
	if s1 == nil || s2 == nil {
		return ErrClosed
	}

	p1 := s1.GetProtocol()
	p2 := s2.GetProtocol()

	if !ValidPeers(p1, p2) {
		return ErrBadProto
	}

	if err := s1.SetOption(OptionRaw, true); err != nil {
		return err
	}
	if err := s2.SetOption(OptionRaw, true); err != nil {
		return err
	}

	go forwarder(s1, s2)
	go forwarder(s2, s1)
	return nil
}

// Forwarder takes messages from one socket, and sends them to the other.
// The sockets must be of compatible types, and must be in Raw mode.
func forwarder(fromSock Socket, toSock Socket) {
	for {
		m, err := fromSock.RecvMsg()
		if err != nil {
			// Probably closed socket, nothing else we can do.
			return
		}

		err = toSock.SendMsg(m)
		if err != nil {
			return
		}
	}
}
