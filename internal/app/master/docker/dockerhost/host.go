package dockerhost

import (
	"net"
	"net/url"
	"os"
)

const (
	localHostIP = "127.0.0.1"
)

// GetIP returns the Docker host IP address
func GetIP() string {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		return localHostIP
	}

	u, err := url.Parse(dockerHost)
	if err != nil {
		return localHostIP
	}

	switch u.Scheme {
	case "unix":
		return localHostIP
	default:
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			return localHostIP
		}

		return host
	}
}
