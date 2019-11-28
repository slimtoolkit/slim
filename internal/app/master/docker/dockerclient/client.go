package dockerclient

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	"github.com/fsouza/go-dockerclient"

	log "github.com/sirupsen/logrus"
)

const (
	EnvDockerHost      = "DOCKER_HOST"
	EnvDockerTLSVerify = "DOCKER_TLS_VERIFY"
	EnvDockerCertPath  = "DOCKER_CERT_PATH"
	UnixSocketPath     = "/var/run/docker.sock"
	UnixSocketAddr     = "unix:///var/run/docker.sock"
)

var (
	ErrNoDockerInfo = errors.New("no docker info")
)

// New creates a new Docker client instance
func New(config *config.DockerClient) (*docker.Client, error) {
	var client *docker.Client
	var err error

	if !fsutil.Exists(UnixSocketPath) && config.Env[EnvDockerHost] == "" && config.Host == "" {
		return nil, ErrNoDockerInfo
	}

	newTLSClient := func(host string, certPath string, verify bool) (*docker.Client, error) {
		var ca []byte

		cert, err := ioutil.ReadFile(filepath.Join(certPath, "cert.pem"))
		if err != nil {
			return nil, err
		}

		key, err := ioutil.ReadFile(filepath.Join(certPath, "key.pem"))
		if err != nil {
			return nil, err
		}

		if verify {
			var err error
			ca, err = ioutil.ReadFile(filepath.Join(certPath, "ca.pem"))
			if err != nil {
				return nil, err
			}
		}

		return docker.NewVersionedTLSClientFromBytes(host, cert, key, ca, "")
	}

	switch {
	case config.Host != "" &&
		config.UseTLS &&
		config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, true)
		if err != nil {
			return nil, err
		}

		log.Debug("docker-slim: new Docker client (TLS,verify) [1]")

	case config.Host != "" &&
		config.UseTLS &&
		!config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, false)
		if err != nil {
			return nil, err
		}

		log.Debug("docker-slim: new Docker client (TLS,no verify) [2]")

	case config.Host != "" &&
		!config.UseTLS:
		client, err = docker.NewClient(config.Host)
		if err != nil {
			return nil, err
		}

		log.Debug("docker-slim: new Docker client [3]")

	case config.Host == "" &&
		!config.VerifyTLS &&
		config.Env[EnvDockerTLSVerify] == "1" &&
		config.Env[EnvDockerCertPath] != "" &&
		config.Env[EnvDockerHost] != "":
		client, err = newTLSClient(config.Env[EnvDockerHost], config.Env[EnvDockerCertPath], false)
		if err != nil {
			return nil, err
		}

		log.Debug("docker-slim: new Docker client (TLS,no verify) [4]")

	case config.Env[EnvDockerHost] != "":
		client, err = docker.NewClientFromEnv()
		if err != nil {
			return nil, err
		}

		log.Debug("docker-slim: new Docker client (env) [5]")

	case config.Host == "" && config.Env[EnvDockerHost] == "":
		config.Host = UnixSocketAddr
		client, err = docker.NewClient(config.Host)
		if err != nil {
			return nil, err
		}

		log.Debug("docker-slim: new Docker client (default) [6]")

	default:
		return nil, ErrNoDockerInfo
	}

	if config.Env[EnvDockerHost] == "" {
		if err := os.Setenv(EnvDockerHost, config.Host); err != nil {
			errutil.WarnOn(err)
		}

		log.Debug("docker-slim: configured DOCKER_HOST env var")
	}

	return client, nil
}
