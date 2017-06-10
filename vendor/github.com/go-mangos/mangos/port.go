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

// Port represents the high level interface to a low level communications
// channel.  There is one of these associated with a given TCP connection,
// for example.  This interface is intended for application use.
//
// Note that applicatons cannot send or receive data on a Port directly.
type Port interface {

	// Address returns the address (URL form) associated with the port.
	// This matches the string passed to Dial() or Listen().
	Address() string

	// GetProp returns an arbitrary property.  The details will vary
	// for different transport types.
	GetProp(name string) (interface{}, error)

	// IsOpen determines whether this is open or not.
	IsOpen() bool

	// Close closes the Conn.  This does a disconnect, or something similar.
	// Note that if a dialer is present and active, it will redial.
	Close() error

	// IsServer returns true if the connection is from a server (Listen).
	IsServer() bool

	// IsClient returns true if the connection is from a client (Dial).
	IsClient() bool

	// LocalProtocol returns the local protocol number.
	LocalProtocol() uint16

	// RemoteProtocol returns the remote protocol number.
	RemoteProtocol() uint16

	// Dialer returns the dialer for this Port, or nil if a server.
	Dialer() Dialer

	// Listener returns the listener for this Port, or nil if a client.
	Listener() Listener
}

// PortAction determines whether the action on a Port is addition or removal.
type PortAction int

// PortAction values.
const (
	PortActionAdd = iota
	PortActionRemove
)

// PortHook is a function that is called when a port is added or removed to or
// from a Socket.  In the case of PortActionAdd, the function may return false
// to indicate that the port should not be added.
type PortHook func(PortAction, Port) bool
