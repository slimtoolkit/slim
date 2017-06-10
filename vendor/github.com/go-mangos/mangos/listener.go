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

// Listener is an interface to the underlying listener for a transport
// and address.
type Listener interface {
	// Close closes the listener, and removes it from any active socket.
	// Further operations on the Listener will return ErrClosed.
	Close() error

	// Listen starts listening for new connectons on the address.
	Listen() error

	// Address returns the string (full URL) of the Listener.
	Address() string

	// SetOption sets an option the Listener. Setting options
	// can only be done before Listen() has been called.
	SetOption(name string, value interface{}) error

	// GetOption gets an option value from the Listener.
	GetOption(name string) (interface{}, error)
}
