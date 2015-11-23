package dockerhost

import (
	"net"
	"net/url"
	"os"
)

func GetIP() string {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		return "127.0.0.1"
	}

	u, err := url.Parse(dockerHost)
	if err != nil {
		return "127.0.0.1"
	}

	switch u.Scheme {
	case "unix":
		return "127.0.0.1"
	default:
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			return "127.0.0.1"
		}

		return host
	}
}
