// This file is a modified version of
// https://github.com/caddyserver/caddy/blob/44e5e9e/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Modifications made permit targeting containerized PHP scripts
// and decouple Caddy dependencies.

// Copyright 2015 Matthew Holt and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var _ http.RoundTripper = &FastCGITransport{}

// FastCGITransport facilitates FastCGI communication.
type FastCGITransport struct {
	// Root is the fastcgi root directory.
	// Defaults to the root directory of the container.
	Root string

	// The path in the URL will be split into two, with the first piece ending
	// with the value of SplitPath. The first piece will be assumed as the
	// actual resource (CGI script) name, and the second piece will be set to
	// PATH_INFO for the CGI script to use.
	SplitPath []string

	// Extra environment variables.
	EnvVars map[string]string

	// The duration used to set a deadline when connecting to an upstream.
	DialTimeout time.Duration

	// The duration used to set a deadline when reading from the FastCGI server.
	ReadTimeout time.Duration

	// The duration used to set a deadline when sending to the FastCGI server.
	WriteTimeout time.Duration
}

// init sets up t.
func (t *FastCGITransport) init() {
	if t.Root == "" {
		t.Root = "/"
	}
}

// RoundTrip implements http.RoundTripper.
func (t FastCGITransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.init()

	env, err := t.buildEnv(r)
	if err != nil {
		return nil, fmt.Errorf("building environment: %v", err)
	}

	network, address := "tcp", r.URL.Host

	if log.IsLevelEnabled(log.DebugLevel) {
		envJSON, _ := json.Marshal(env)
		log.Debugf("HTTP probe - FastCGI env - %s", string(envJSON))
	}

	ctx := r.Context()
	dialer := net.Dialer{Timeout: t.DialTimeout}
	fcgiBackend, err := DialWithDialerContext(ctx, network, address, dialer)
	if err != nil {
		// TODO: wrap in a special error type if the dial failed, so retries can happen if enabled
		return nil, fmt.Errorf("dialing backend: %v", err)
	}
	// fcgiBackend gets closed when response body is closed (see clientCloser)

	// read/write timeouts
	if err := fcgiBackend.SetReadTimeout(t.ReadTimeout); err != nil {
		return nil, fmt.Errorf("setting read timeout: %v", err)
	}
	if err := fcgiBackend.SetWriteTimeout(t.WriteTimeout); err != nil {
		return nil, fmt.Errorf("setting write timeout: %v", err)
	}

	contentLength := r.ContentLength
	if contentLength == 0 {
		contentLength, _ = strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	}

	var resp *http.Response
	switch r.Method {
	case http.MethodHead:
		resp, err = fcgiBackend.Head(env)
	case http.MethodGet:
		resp, err = fcgiBackend.Get(env, r.Body, contentLength)
	case http.MethodOptions:
		resp, err = fcgiBackend.Options(env)
	default:
		resp, err = fcgiBackend.Post(env, r.Method, r.Header.Get("Content-Type"), r.Body, contentLength)
	}

	return resp, err
}

// buildEnv returns a set of CGI environment variables for the request.
func (t FastCGITransport) buildEnv(r *http.Request) (map[string]string, error) {

	// Separate remote IP and port; more lenient than net.SplitHostPort
	var ip, port string
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > -1 {
		ip = r.RemoteAddr[:idx]
		port = r.RemoteAddr[idx+1:]
	} else {
		ip = r.RemoteAddr
	}

	// Remove [] from IPv6 addresses
	ip = strings.Replace(ip, "[", "", 1)
	ip = strings.Replace(ip, "]", "", 1)

	// make sure file root is absolute
	if !path.IsAbs(t.Root) {
		return nil, fmt.Errorf("root %q is not absolute", t.Root)
	}
	root := t.Root

	fpath := r.URL.Path
	scriptName := fpath

	docURI := fpath
	// split "actual path" from "path info" if configured
	var pathInfo string
	if splitPos := t.splitPos(fpath); splitPos > -1 {
		docURI = fpath[:splitPos]
		pathInfo = fpath[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		scriptName = strings.TrimSuffix(scriptName, pathInfo)
	}

	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	scriptFilename := path.Join(root, scriptName)

	// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
	// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
	if scriptName != "" && !path.IsAbs(scriptName) {
		scriptName = "/" + scriptName
	}

	requestScheme := "http"
	if r.TLS != nil {
		requestScheme = "https"
	}

	reqHost, reqPort, err := net.SplitHostPort(r.Host)
	if err != nil {
		// whatever, just assume there was no port
		reqHost = r.Host
	}

	authUser, _, _ := r.BasicAuth()

	// Some variables are unused but cleared explicitly to prevent
	// the parent environment from interfering.
	env := map[string]string{
		// Variables defined in CGI 1.1 spec
		"AUTH_TYPE":         "", // Not used
		"CONTENT_LENGTH":    r.Header.Get("Content-Length"),
		"CONTENT_TYPE":      r.Header.Get("Content-Type"),
		"GATEWAY_INTERFACE": "CGI/1.1",
		"PATH_INFO":         pathInfo,
		"QUERY_STRING":      r.URL.RawQuery,
		"REMOTE_ADDR":       ip,
		"REMOTE_HOST":       ip, // For speed, remote host lookups disabled
		"REMOTE_PORT":       port,
		"REMOTE_IDENT":      "", // Not used
		"REMOTE_USER":       authUser,
		"REQUEST_METHOD":    r.Method,
		"REQUEST_SCHEME":    requestScheme,
		"SERVER_NAME":       reqHost,
		"SERVER_PROTOCOL":   r.Proto,
		"SERVER_SOFTWARE":   "slimtoolkit/v0.0.0",

		// Other variables
		"DOCUMENT_ROOT": root,
		"DOCUMENT_URI":  docURI,
		"HTTP_HOST":     r.Host, // added here, since not always part of headers
		// "REQUEST_URI":     "",
		"SCRIPT_FILENAME": scriptFilename,
		"SCRIPT_NAME":     scriptName,
	}

	// compliance with the CGI specification requires that
	// PATH_TRANSLATED should only exist if PATH_INFO is defined.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 14
	if pathInfo != "" {
		env["PATH_TRANSLATED"] = path.Join(root, pathInfo)
	}

	// compliance with the CGI specification requires that
	// SERVER_PORT should only exist if it's a valid numeric value.
	// Info: https://www.ietf.org/rfc/rfc3875 Page 18
	if reqPort != "" {
		env["SERVER_PORT"] = reqPort
	}

	// Some web apps rely on knowing HTTPS or not
	if r.TLS != nil {
		env["HTTPS"] = "on"
		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have a SSL_ prefix and not TLS_).
		v, ok := tlsProtocolStrings[r.TLS.Version]
		if ok {
			env["SSL_PROTOCOL"] = v
		}
		// and pass the cipher suite in a manner compatible with apache's mod_ssl
		for _, cs := range tls.CipherSuites() {
			if cs.ID == r.TLS.CipherSuite {
				env["SSL_CIPHER"] = cs.Name
				break
			}
		}
	}

	// Add env variables from config (with support for placeholders in values)
	for key, value := range t.EnvVars {
		env[key] = value
	}

	// Add all HTTP headers to env variables
	for field, val := range r.Header {
		header := strings.ToUpper(field)
		header = headerNameReplacer.Replace(header)
		env["HTTP_"+header] = strings.Join(val, ", ")
	}

	return env, nil
}

// splitPos returns the index where path should
// be split based on t.SplitPath.
func (t FastCGITransport) splitPos(path string) int {
	if len(t.SplitPath) == 0 {
		return 0
	}

	lowerPath := strings.ToLower(path)
	for _, split := range t.SplitPath {
		if idx := strings.Index(lowerPath, strings.ToLower(split)); idx > -1 {
			return idx + len(split)
		}
	}
	return -1
}

// Map of supported protocols to Apache ssl_mod format
// Note that these are slightly different from SupportedProtocols in caddytls/config.go
var tlsProtocolStrings = map[uint16]string{
	tls.VersionTLS10: "TLSv1",
	tls.VersionTLS11: "TLSv1.1",
	tls.VersionTLS12: "TLSv1.2",
	tls.VersionTLS13: "TLSv1.3",
}

var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")
