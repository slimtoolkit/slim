package dockerhost

import (
	"net"
	"net/url"
	"os"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const (
	localHostIP = "127.0.0.1"
)

// GetIP returns the Docker host IP address
func GetIP(apiClient *dockerapi.Client) string {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		if apiClient != nil {
			netInfo, err := apiClient.NetworkInfo("bridge")
			if err != nil {
				log.WithFields(log.Fields{
					"op":    "dockerhost.GetIP",
					"error": err,
				}).Debug("apiClient.NetworkInfo")
			} else {
				if netInfo != nil && netInfo.Name == "bridge" {
					if len(netInfo.IPAM.Config) > 0 {
						return netInfo.IPAM.Config[0].Gateway
					}
				}
			}
		}

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
