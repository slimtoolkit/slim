package dockerclient

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	"github.com/slimtoolkit/slim/pkg/util/jsonutil"
)

const (
	EnvDockerAPIVer      = "DOCKER_API_VERSION"
	EnvDockerHost        = "DOCKER_HOST"
	EnvDockerTLSVerify   = "DOCKER_TLS_VERIFY"
	EnvDockerCertPath    = "DOCKER_CERT_PATH"
	UnixSocketPath       = "/var/run/docker.sock"
	UnixSocketAddr       = "unix:///var/run/docker.sock"
	unixUserSocketSuffix = ".docker/run/docker.sock"
)

var EnvVarNames = []string{
	EnvDockerHost,
	EnvDockerTLSVerify,
	EnvDockerCertPath,
	EnvDockerAPIVer,
}

var (
	ErrNoDockerInfo = errors.New("no docker info")
)

func UserDockerSocket() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, unixUserSocketSuffix)
}

type SocketInfo struct {
	Address       string `json:"address"`
	FilePath      string `json:"file_path"`
	FileType      string `json:"type"`
	FilePerms     string `json:"perms"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
	TargetPerms   string `json:"target_perms,omitempty"`
	TargetType    string `json:"target_type,omitempty"`
	CanRead       bool   `json:"can_read"`
	CanWrite      bool   `json:"can_write"`
}

func getSocketInfo(filePath string) (*SocketInfo, error) {
	info := &SocketInfo{
		FileType: "file",
		FilePath: filePath,
	}

	fi, err := os.Lstat(info.FilePath)
	if err != nil {
		log.Errorf("dockerclient.getSocketInfo.os.Lstat(%s): error - %v", filePath, err)
		return nil, err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		info.SymlinkTarget, err = os.Readlink(info.FilePath)
		if err != nil {
			log.Errorf("dockerclient.getSocketInfo.os.Readlink(%s): error - %v", filePath, err)
			return nil, err
		}
		info.FileType = "symlink"
		info.FilePerms = fmt.Sprintf("%#o", fi.Mode().Perm())
		if info.SymlinkTarget != "" {
			tfi, err := os.Lstat(info.SymlinkTarget)
			if err != nil {
				log.Errorf("dockerclient.getSocketInfo.os.Lstat(%s): error - %v", info.SymlinkTarget, err)
				return nil, err
			}

			info.TargetPerms = fmt.Sprintf("%#o", tfi.Mode().Perm())
			if tfi.Mode()&os.ModeSymlink != 0 {
				info.TargetType = "symlink"
			}
		}
	}

	info.CanRead, err = fsutil.HasReadAccess(info.FilePath)
	if err != nil {
		log.Errorf("dockerclient.getSocketInfo.fsutil.HasReadAccess(%s): error - %v", info.FilePath, err)
		return nil, err
	}

	info.CanWrite, err = fsutil.HasWriteAccess(info.FilePath)
	if err != nil {
		log.Errorf("dockerclient.getSocketInfo.fsutil.HasWriteAccess(%s): error - %v", info.FilePath, err)
		return nil, err
	}

	return info, nil
}

func GetUnixSocketAddr() (*SocketInfo, error) {
	//note: may move this to dockerutil
	if _, err := os.Stat(UnixSocketPath); err == nil {
		socketInfo, err := getSocketInfo(UnixSocketPath)
		if err != nil {
			return nil, err
		}

		socketInfo.Address = UnixSocketAddr
		log.Debugf("dockerclient.GetUnixSocketAddr(): found => %s", jsonutil.ToString(socketInfo))
		return socketInfo, nil
	}

	userDockerSocket := UserDockerSocket()
	if _, err := os.Stat(userDockerSocket); err == nil {
		socketInfo, err := getSocketInfo(userDockerSocket)
		if err != nil {
			return nil, err
		}

		socketInfo.Address = fmt.Sprintf("unix://%s", userDockerSocket)
		log.Debugf("dockerclient.GetUnixSocketAddr(): found => %s", jsonutil.ToString(socketInfo))
		return socketInfo, nil
	}

	return nil, fmt.Errorf("docker socket not found")
}

// newVersionedClient builds a *docker.Client for host, honoring an explicit
// apiVersion when the caller set one. When apiVersion is empty (a plain
// "slim build"/"mint build" with no DOCKER_API_VERSION set - the common
// case in every slimtoolkit/slim#646 and mintoolkit/mint#95 report),
// go-dockerclient leaves the client's internal requestedAPIVersion nil for
// its whole life (it only parses config.APIVersion into that field when the
// string is non-empty), so every request the client sends omits the
// "/vX.Y/" path segment. A daemon reached through a proxy - dind
// (Docker-in-Docker: a CI runner's own container running a Docker daemon,
// the topology behind every #646/#95 report) - reads an unversioned
// request as coming from the oldest client it supports and rejects it
// ("client version ... is too old"). Probing the daemon's real API version
// with an initial Version() call and rebuilding the client with that
// version populates requestedAPIVersion exactly as if the caller had set
// DOCKER_API_VERSION by hand, which is the only workaround either issue
// thread ever found.
func newVersionedClient(host, apiVersion string) (*docker.Client, error) {
	client, err := docker.NewVersionedClient(host, apiVersion)
	if err != nil {
		return nil, err
	}

	if apiVersion != "" {
		client.SkipServerVersionCheck = true
		return client, nil
	}

	env, err := client.Version()
	if err != nil {
		// Daemon unreachable or otherwise misbehaving: hand back the
		// original unversioned client so callers see the same connection
		// error they always would have, instead of masking it here.
		return client, nil
	}

	discovered := env.Get("ApiVersion")
	if discovered == "" {
		return client, nil
	}

	versioned, err := docker.NewVersionedClient(host, discovered)
	if err != nil {
		return client, nil
	}

	// The just-discovered version must be trusted as-is: go-dockerclient's
	// own internal version bootstrap (triggered lazily by the first request
	// when SkipServerVersionCheck is false) builds its own "/version" probe
	// through getURL(), which - now that requestedAPIVersion is set -
	// prefixes even that bootstrap call with "/vX.Y/", so it would ask a
	// real daemon for "/v1.44/version" instead of "/version" and fail to
	// parse the (missing) result. Skipping that redundant self-check is
	// exactly what happens today when a caller sets DOCKER_API_VERSION by
	// hand (see the config.APIVersion != "" branches above).
	versioned.SkipServerVersionCheck = true

	return versioned, nil
}

// New creates a new Docker client instance
func New(config *config.DockerClient) (*docker.Client, error) {
	var client *docker.Client
	var err error

	newTLSClient := func(host string, certPath string, verify bool, apiVersion string) (*docker.Client, error) {
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

		return docker.NewVersionedTLSClientFromBytes(host, cert, key, ca, apiVersion)
	}

	switch {
	case config.Host != "" &&
		config.UseTLS &&
		config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, true, config.APIVersion)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,verify) [1]")

	case config.Host != "" &&
		config.UseTLS &&
		!config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, false, config.APIVersion)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,no verify) [2]")

	case config.Host != "" &&
		!config.UseTLS:
		client, err = newVersionedClient(config.Host, config.APIVersion)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client [3]")

	case config.Host == "" &&
		!config.VerifyTLS &&
		config.Env[EnvDockerTLSVerify] == "1" &&
		config.Env[EnvDockerCertPath] != "" &&
		config.Env[EnvDockerHost] != "":
		client, err = newTLSClient(config.Env[EnvDockerHost], config.Env[EnvDockerCertPath], false, config.APIVersion)
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
		socketInfo, err := GetUnixSocketAddr()
		if err != nil {
			return nil, err
		}

		if socketInfo == nil || socketInfo.Address == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		if socketInfo.CanRead == false || socketInfo.CanWrite == false {
			return nil, fmt.Errorf("insufficient socket permissions (can_read=%v can_write=%v)", socketInfo.CanRead, socketInfo.CanWrite)
		}

		config.Host = socketInfo.Address
		client, err = newVersionedClient(config.Host, config.APIVersion)
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

	if config.APIVersion != "" && config.Env[EnvDockerAPIVer] == "" {
		if err := os.Setenv(EnvDockerAPIVer, config.APIVersion); err != nil {
			errutil.WarnOn(err)
		}

		log.Debug("dockerclient.New: configured DOCKER_API_VERSION env var")
	}

	return client, nil
}
