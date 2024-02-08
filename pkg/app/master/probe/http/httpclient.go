package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/probe/http/internal"
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

func getHTTPClient(proto string) (*http.Client, error) {
	switch proto {
	case config.ProtoHTTP2:
		return getHTTP2Client(false), nil
	case config.ProtoHTTP2C:
		return getHTTP2Client(true), nil
	default:
		return getHTTP1Client(), nil
	}

	return nil, fmt.Errorf("unsupported HTTP-family protocol %s", proto)
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

func getFastCGIClient(cfg *config.FastCGIProbeWrapperConfig) *http.Client {

	genericTimeout := time.Second * 30
	var dialTimeout, readTimeout, writeTimeout time.Duration
	if dialTimeout = cfg.DialTimeout; dialTimeout == 0 {
		dialTimeout = genericTimeout
	}
	if readTimeout = cfg.ReadTimeout; readTimeout == 0 {
		readTimeout = genericTimeout
	}
	if writeTimeout = cfg.WriteTimeout; writeTimeout == 0 {
		writeTimeout = genericTimeout
	}

	return &http.Client{
		Timeout: genericTimeout,
		Transport: &internal.FastCGITransport{
			Root:         cfg.Root,
			SplitPath:    cfg.SplitPath,
			EnvVars:      cfg.EnvVars,
			DialTimeout:  dialTimeout,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
	}
}
