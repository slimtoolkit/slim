package dockerclient

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/fsouza/go-dockerclient"

	log "github.com/sirupsen/logrus"
)

const (
	EnvDockerHost        = "DOCKER_HOST"
	EnvDockerTLSVerify   = "DOCKER_TLS_VERIFY"
	EnvDockerCertPath    = "DOCKER_CERT_PATH"
	UnixSocketPath       = "/var/run/docker.sock"
	UnixSocketAddr       = "unix:///var/run/docker.sock"
	unixUserSocketSuffix = ".docker/run/docker.sock"
)

var (
	ErrNoDockerInfo = errors.New("no docker info")
)

func UserDockerSocket() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, unixUserSocketSuffix)
}

func GetUnixSocketAddr() string {
	//note: may move this to dockerutil
	if _, err := os.Stat(UnixSocketPath); err == nil {
		log.Tracef("dockerclient.GetUnixSocketAddr(): found - %s", UnixSocketPath)
		return UnixSocketAddr
	}

	userDockerSocket := UserDockerSocket()
	if _, err := os.Stat(userDockerSocket); err == nil {
		log.Tracef("dockerclient.GetUnixSocketAddr(): found - %s", userDockerSocket)
		return fmt.Sprintf("unix://%s", userDockerSocket)
	}

	return ""
}

// New creates a new Docker client instance
func New(config *config.DockerClient) (*docker.Client, error) {
	var client *docker.Client
	var err error

	unixSocketAddr := GetUnixSocketAddr()
	if unixSocketAddr == "" && config.Env[EnvDockerHost] == "" && config.Host == "" {
		return nil, ErrNoDockerInfo
	}

	newTLSClient := func(host string, certPath string, verify bool) (*docker.Client, error) {
		var ca []byte

		cert, err := os.ReadFile(filepath.Join(certPath, "cert.pem"))
		if err != nil {
			return nil, err
		}

		key, err := os.ReadFile(filepath.Join(certPath, "key.pem"))
		if err != nil {
			return nil, err
		}

		if verify {
			var err error
			ca, err = os.ReadFile(filepath.Join(certPath, "ca.pem"))
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

		log.Debug("dockerclient.New: new Docker client (TLS,verify) [1]")

	case config.Host != "" &&
		config.UseTLS &&
		!config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, false)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,no verify) [2]")

	case config.Host != "" &&
		!config.UseTLS:
		client, err = docker.NewClient(config.Host)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client [3]")

	case config.Host == "" &&
		!config.VerifyTLS &&
		config.Env[EnvDockerTLSVerify] == "1" &&
		config.Env[EnvDockerCertPath] != "" &&
		config.Env[EnvDockerHost] != "":
		client, err = newTLSClient(config.Env[EnvDockerHost], config.Env[EnvDockerCertPath], false)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,no verify) [4]")

	case config.Env[EnvDockerHost] != "":
		client, err = docker.NewClientFromEnv()
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (env) [5]")

	case config.Host == "" && config.Env[EnvDockerHost] == "":
		config.Host = GetUnixSocketAddr()
		if config.Host == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		client, err = docker.NewClient(config.Host)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (default) [6]")

	default:
		return nil, ErrNoDockerInfo
	}

	if config.Env[EnvDockerHost] == "" {
		if err := os.Setenv(EnvDockerHost, config.Host); err != nil {
			errutil.WarnOn(err)
		}

		log.Debug("dockerclient.New: configured DOCKER_HOST env var")
	}

	return client, nil
}
