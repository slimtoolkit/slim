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

// Socket is the main access handle applications use to access the SP
// system.  It is an abstraction of an application's "connection" to a
// messaging topology.  Applications can have more than one Socket open
// at a time.
type Socket interface {
	// Close closes the open Socket.  Further operations on the socket
	// will return ErrClosed.
	Close() error

	// Send puts the message on the outbound send queue.  It blocks
	// until the message can be queued, or the send deadline expires.
	// If a queued message is later dropped for any reason,
	// there will be no notification back to the application.
	Send([]byte) error

	// Recv receives a complete message.  The entire message is received.
	Recv() ([]byte, error)

	// SendMsg puts the message on the outbound send.  It works like Send,
	// but allows the caller to supply message headers.  AGAIN, the Socket
	// ASSUMES OWNERSHIP OF THE MESSAGE.
	SendMsg(*Message) error

	// RecvMsg receives a complete message, including the message header,
	// which is useful for protocols in raw mode.
	RecvMsg() (*Message, error)

	// Dial connects a remote endpoint to the Socket.  The function
	// returns immediately, and an asynchronous goroutine is started to
	// establish and maintain the connection, reconnecting as needed.
	// If the address is invalid, then an error is returned.
	Dial(addr string) error

	DialOptions(addr string, options map[string]interface{}) error

	// NewDialer returns a Dialer object which can be used to get
	// access to the underlying configuration for dialing.
	NewDialer(addr string, options map[string]interface{}) (Dialer, error)

	// Listen connects a local endpoint to the Socket.  Remote peers
	// may connect (e.g. with Dial) and will each be "connected" to
	// the Socket.  The accepter logic is run in a separate goroutine.
	// The only error possible is if the address is invalid.
	Listen(addr string) error

	ListenOptions(addr string, options map[string]interface{}) error

	NewListener(addr string, options map[string]interface{}) (Listener, error)

	// GetOption is used to retrieve an option for a socket.
	GetOption(name string) (interface{}, error)

	// SetOption is used to set an option for a socket.
	SetOption(name string, value interface{}) error

	// Protocol is used to get the underlying Protocol.
	GetProtocol() Protocol

	// AddTransport adds a new Transport to the socket.  Transport specific
	// options may have been configured on the Transport prior to this.
	AddTransport(Transport)

	// SetPortHook sets a PortHook function to be called when a Port is
	// added or removed from this socket (connect/disconnect).  The previous
	// hook is returned (nil if none.)
	SetPortHook(PortHook) PortHook
}
