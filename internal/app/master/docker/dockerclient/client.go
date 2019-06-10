package dockerclient

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/Sirupsen/logrus"
)

// New creates a new Docker client instance
func New(config *config.DockerClient) *docker.Client {
	var client *docker.Client
	var err error

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
		errutil.FailOn(err)
		log.Debug("docker-slim: new Docker client (TLS,verify) [1]")

	case config.Host != "" &&
		config.UseTLS &&
		!config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, false)
		errutil.FailOn(err)
		log.Debug("docker-slim: new Docker client (TLS,no verify) [2]")

	case config.Host != "" &&
		!config.UseTLS:
		client, err = docker.NewClient(config.Host)
		errutil.FailOn(err)
		log.Debug("docker-slim: new Docker client [3]")

	case config.Host == "" &&
		!config.VerifyTLS &&
		config.Env["DOCKER_TLS_VERIFY"] == "1" &&
		config.Env["DOCKER_CERT_PATH"] != "" &&
		config.Env["DOCKER_HOST"] != "":
		client, err = newTLSClient(config.Env["DOCKER_HOST"], config.Env["DOCKER_CERT_PATH"], false)
		errutil.FailOn(err)
		log.Debug("docker-slim: new Docker client (TLS,no verify) [4]")

	case config.Env["DOCKER_HOST"] != "":
		client, err = docker.NewClientFromEnv()
		errutil.FailOn(err)
		log.Debug("docker-slim: new Docker client (env) [5]")

	case config.Host == "" && config.Env["DOCKER_HOST"] == "":
		config.Host = "unix:///var/run/docker.sock"
		client, err = docker.NewClient(config.Host)
		errutil.FailOn(err)
		log.Debug("docker-slim: new Docker client (default) [6]")

	default:
		errutil.Fail("no config for Docker client")
	}

	if config.Env["DOCKER_HOST"] == "" {
		if err := os.Setenv("DOCKER_HOST", config.Host); err != nil {
			errutil.WarnOn(err)
		}

		log.Debug("docker-slim: configured DOCKER_HOST env var")
	}

	return client
}
