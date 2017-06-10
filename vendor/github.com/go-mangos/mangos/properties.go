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

// The following are Properties which are exposed on a Port.

const (
	// PropLocalAddr expresses a local address.  For dialers, this is
	// the (often random) address that was locally bound.  For listeners,
	// it is usually the service address.  The value is a net.Addr.
	PropLocalAddr = "LOCAL-ADDR"

	// PropRemoteAddr expresses a remote address.  For dialers, this is
	// the service address.  For listeners, its the address of the far
	// end dialer.  The value is a net.Addr.
	PropRemoteAddr = "REMOTE-ADDR"

	// PropTLSConnState is used to supply TLS connection details. The
	// value is a tls.ConnectionState.  It is only valid when TLS is used.
	PropTLSConnState = "TLS-STATE"

	// PropHTTPRequest conveys an *http.Request.  This property only exists
	// for websocket connections.
	PropHTTPRequest = "HTTP-REQUEST"
)
