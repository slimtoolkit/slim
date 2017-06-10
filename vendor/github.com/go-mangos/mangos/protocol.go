// Copyright 2016 The Mangos Authors
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
	"time"
)

// Endpoint represents the handle that a Protocol implementation has
// to the underlying stream transport.  It can be thought of as one side
// of a TCP, IPC, or other type of connection.
type Endpoint interface {
	// GetID returns a unique 31-bit value associated with the Endpoint.
	// The value is unique for a given socket, at a given time.
	GetID() uint32

	// Close does what you think.
	Close() error

	// SendMsg sends a message.  On success it returns nil. This is a
	// blocking call.
	SendMsg(*Message) error

	// RecvMsg receives a message.  It blocks until the message is
	// received.  On error, the pipe is closed and nil is returned.
	RecvMsg() *Message
}

// Protocol implementations handle the "meat" of protocol processing.  Each
// protocol type will implement one of these.  For protocol pairs (REP/REQ),
// there will be one for each half of the protocol.
type Protocol interface {

	// Init is called by the core to allow the protocol to perform
	// any initialization steps it needs.  It should save the handle
	// for future use, as well.
	Init(ProtocolSocket)

	// Shutdown is used to drain the send side.  It is only ever called
	// when the socket is being shutdown cleanly. Protocols should use
	// the linger time, and wait up to that time for sockets to drain.
	Shutdown(time.Time)

	// AddEndpoint is called when a new Endpoint is added to the socket.
	// Typically this is as a result of connect or accept completing.
	AddEndpoint(Endpoint)

	// RemoveEndpoint is called when an Endpoint is removed from the socket.
	// Typically this indicates a disconnected or closed connection.
	RemoveEndpoint(Endpoint)

	// ProtocolNumber returns a 16-bit value for the protocol number,
	// as assigned by the SP governing body. (IANA?)
	Number() uint16

	// Name returns our name.
	Name() string

	// PeerNumber() returns a 16-bit number for our peer protocol.
	PeerNumber() uint16

	// PeerName() returns the name of our peer protocol.
	PeerName() string

	// GetOption is used to retrieve the current value of an option.
	// If the protocol doesn't recognize the option, EBadOption should
	// be returned.
	GetOption(string) (interface{}, error)

	// SetOption is used to set an option.  EBadOption is returned if
	// the option name is not recognized, EBadValue if the value is
	// invalid.
	SetOption(string, interface{}) error
}

// The follow are optional interfaces that a Protocol can choose to implement.

// ProtocolRecvHook is intended to be an additional extension
// to the Protocol interface.
type ProtocolRecvHook interface {
	// RecvHook is called just before the message is handed to the
	// application.  The message may be modified.  If false is returned,
	// then the message is dropped.
	RecvHook(*Message) bool
}

// ProtocolSendHook is intended to be an additional extension
// to the Protocol interface.
type ProtocolSendHook interface {
	// SendHook is called when the application calls Send.
	// If false is returned, the message will be silently dropped.
	// Note that the message may be dropped for other reasons,
	// such as if backpressure is applied.
	SendHook(*Message) bool
}

// ProtocolSocket is the "handle" given to protocols to interface with the
// socket.  The Protocol implementation should not access any sockets or pipes
// except by using functions made available on the ProtocolSocket.  Note
// that all functions listed here are non-blocking.
type ProtocolSocket interface {
	// SendChannel represents the channel used to send messages.  The
	// application injects messages to it, and the protocol consumes
	// messages from it.  The channel may be closed when the core needs to
	// create a new channel, typically after an option is set that requires
	// the channel to be reconfigured.  (OptionWriteQLen) When the protocol
	// implementation notices this, it should call this function again to obtain
	// the value of the new channel.
	SendChannel() <-chan *Message

	// RecvChannel is the channel used to receive messages.  The protocol
	// should inject messages to it, and the application will consume them
	// later.
	RecvChannel() chan<- *Message

	// The protocol can wait on this channel to close.  When it is closed,
	// it indicates that the application has closed the upper read socket,
	// and the protocol should stop any further read operations on this
	// instance.
	CloseChannel() <-chan struct{}

	// GetOption may be used by the protocol to retrieve an option from
	// the socket.  This can ultimately wind up calling into the socket's
	// own GetOption handler, so care should be used!
	GetOption(string) (interface{}, error)

	// SetOption is used by the Protocol to set an option on the socket.
	// Note that this may set transport options, or even call back down
	// into the protocol's own SetOption interface!
	SetOption(string, interface{}) error

	// SetRecvError is used to cause socket RX callers to report an
	// error.  This can be used to force an error return rather than
	// waiting for a message that will never arrive (e.g. due to state).
	// If set to nil, then RX works normally.
	SetRecvError(error)

	// SetSendError is used to cause socket TX callers to report an
	// error.  This can be used to force an error return rather than
	// waiting to send a message that will never be delivered (e.g. due
	// to incorrect state.)  If set to nil, then TX works normally.
	SetSendError(error)
}

// Useful constants for protocol numbers.  Note that the major protocol number
// is stored in the upper 12 bits, and the minor (subprotocol) is located in
// the bottom 4 bits.
const (
	ProtoPair       = (1 * 16)
	ProtoPub        = (2 * 16)
	ProtoSub        = (2 * 16) + 1
	ProtoReq        = (3 * 16)
	ProtoRep        = (3 * 16) + 1
	ProtoPush       = (5 * 16)
	ProtoPull       = (5 * 16) + 1
	ProtoSurveyor   = (6 * 16) + 2
	ProtoRespondent = (6 * 16) + 3
	ProtoBus        = (7 * 16)

	// Experimental Protocols - Use at Risk

	ProtoStar = (100 * 16)
)

// ProtocolName returns the name corresponding to a given protocol number.
// This is useful for transports like WebSocket, which use a text name
// rather than the number in the handshake.
func ProtocolName(number uint16) string {
	names := map[uint16]string{
		ProtoPair:       "pair",
		ProtoPub:        "pub",
		ProtoSub:        "sub",
		ProtoReq:        "req",
		ProtoRep:        "rep",
		ProtoPush:       "push",
		ProtoPull:       "pull",
		ProtoSurveyor:   "surveyor",
		ProtoRespondent: "respondent",
		ProtoBus:        "bus"}
	return names[number]
}

// ValidPeers returns true if the two sockets are capable of
// peering to one another.  For example, REQ can peer with REP,
// but not with BUS.
func ValidPeers(p1, p2 Protocol) bool {
	if p1.Number() != p2.PeerNumber() {
		return false
	}
	if p2.Number() != p1.PeerNumber() {
		return false
	}
	return true
}

// NullRecv simply loops, receiving and discarding messages, until the
// Endpoint returns back a nil message.  This allows the Endpoint to notice
// a dropped connection.  It is intended for use by Protocols that are write
// only -- it lets them become aware of a loss of connectivity even when they
// have no data to send.
func NullRecv(ep Endpoint) {
	for {
		var m *Message
		if m = ep.RecvMsg(); m == nil {
			return
		}
		m.Free()
	}
}
