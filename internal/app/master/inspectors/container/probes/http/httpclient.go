package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
)

func getHTTP1Client() *http.Client {
	client := &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return client
}

func getHTTP2Client(h2c bool) *http.Client {
	transport := &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Timeout:   time.Second * 30,
		Transport: transport,
	}

	if h2c {
		transport.AllowHTTP = true
		transport.DialTLS = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		}
	}

	return client
}

func getHTTPClient(proto string) *http.Client {
	switch proto {
	case config.ProtoHTTP2:
		return getHTTP2Client(false)
	case config.ProtoHTTP2C:
		return getHTTP2Client(true)
	}

	return getHTTP1Client()
}

func getHTTPAddr(proto, targetHost, port string) string {
	scheme := getHTTPScheme(proto)
	return fmt.Sprintf("%s://%s:%s", scheme, targetHost, port)
}

func getHTTPScheme(proto string) string {
	var scheme string
	switch proto {
	case config.ProtoHTTP:
		scheme = proto
	case config.ProtoHTTPS:
		scheme = proto
	case config.ProtoHTTP2:
		scheme = config.ProtoHTTPS
	case config.ProtoHTTP2C:
		scheme = config.ProtoHTTP
	}

	return scheme
}
