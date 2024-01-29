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
		client, err = docker.NewVersionedClient(config.Host, config.APIVersion)
		if err != nil {
			return nil, err
		}

		if config.APIVersion != "" {
			client.SkipServerVersionCheck = true
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
		client, err = docker.NewVersionedClient(config.Host, config.APIVersion)
		if err != nil {
			return nil, err
		}

		if config.APIVersion != "" {
			client.SkipServerVersionCheck = true
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
