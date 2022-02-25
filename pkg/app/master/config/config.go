package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

// AppOptionsFilename is the default name for the app configs
const AppOptionsFilename = "slim.config.json"

// AppOptions provides a set of global application parameters and command-specific defaults
// AppOptions values override the default flag values if they are set
// AppOptions is loaded from the "slim.config.json" file stored in the state path directory
type AppOptions struct {
	Global *GlobalAppOptions `json:"global,omitempty"`
}

// GlobalAppOptions provides a set of global application parameters
type GlobalAppOptions struct {
	NoColor      *bool   `json:"no_color,omitempty"`
	Debug        *bool   `json:"debug,omitempty"`
	Verbose      *bool   `json:"verbose,omitempty"`
	LogLevel     *string `json:"log_level,omitempty"`
	Log          *string `json:"log,omitempty"`
	LogFormat    *string `json:"log_format,omitempty"`
	UseTLS       *bool   `json:"tls,omitempty"`
	VerifyTLS    *bool   `json:"tls_verify,omitempty"`
	TLSCertPath  *string `json:"tls_cert_path,omitempty"`
	Host         *string `json:"host,omitempty"`
	ArchiveState *string `json:"archive_state,omitempty"`
}

func NewAppOptionsFromFile(dir string) (*AppOptions, error) {
	filePath := filepath.Join(dir, AppOptionsFilename)
	var result AppOptions
	err := fsutil.LoadStructFromFile(filePath, &result)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		if err == fsutil.ErrNoFileData {
			return nil, nil
		}

		return nil, err
	}

	return &result, nil
}

// ContainerOverrides provides a set of container field overrides
// It can also be used to update the image instructions when
// the "image-overrides" flag is provided
type ContainerOverrides struct {
	User            string
	Entrypoint      []string
	ClearEntrypoint bool
	Cmd             []string
	ClearCmd        bool
	Workdir         string
	Env             []string
	Hostname        string
	Network         string
	ExposedPorts    map[docker.Port]struct{}
	Volumes         map[string]struct{}
	Labels          map[string]string
}

// ImageNewInstructions provides a set new image instructions
type ImageNewInstructions struct {
	Entrypoint         []string
	ClearEntrypoint    bool
	Cmd                []string
	ClearCmd           bool
	Workdir            string
	Env                []string
	Volumes            map[string]struct{}
	ExposedPorts       map[docker.Port]struct{}
	Labels             map[string]string
	RemoveEnvs         map[string]struct{}
	RemoveVolumes      map[string]struct{}
	RemoveExposedPorts map[docker.Port]struct{}
	RemoveLabels       map[string]struct{}
}

// ContainerBuildOptions provides the options to use when
// building container images from Dockerfiles
type ContainerBuildOptions struct {
	Dockerfile        string
	DockerfileContext string
	Tag               string
	ExtraHosts        string
	BuildArgs         []CBOBuildArg
	Labels            map[string]string
	CacheFrom         []string
	Target            string
	NetworkMode       string
}

type CBOBuildArg struct {
	Name  string
	Value string
}

// ContainerRunOptions provides the options to use running a container
type ContainerRunOptions struct {
	HostConfig *docker.HostConfig
	//Explicit overrides for the base and host config fields
	//Host config field override are applied
	//on top of the fields in the HostConfig struct if it's provided (volume mounts are merged though)
	Runtime      string
	SysctlParams map[string]string
	ShmSize      int64
}

// VolumeMount provides the volume mount configuration information
type VolumeMount struct {
	Source      string
	Destination string
	Options     string
}

const (
	ProtoHTTP   = "http"
	ProtoHTTPS  = "https"
	ProtoHTTP2  = "http2"
	ProtoHTTP2C = "http2c"
	ProtoWS     = "ws"
	ProtoWSS    = "wss"
)

func IsProto(value string) bool {
	switch strings.ToLower(value) {
	case ProtoHTTP,
		ProtoHTTPS,
		ProtoHTTP2,
		ProtoHTTP2C,
		ProtoWS,
		ProtoWSS:
		return true
	default:
		return false
	}
}

// HTTPProbeCmd provides the HTTP probe parameters
type HTTPProbeCmd struct {
	Method   string   `json:"method"`
	Resource string   `json:"resource"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
	Headers  []string `json:"headers"`
	Body     string   `json:"body"`
	BodyFile string   `json:"body_file"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Crawl    bool     `json:"crawl"`

	FastCGI *FastCGIProbeWrapperConfig `json:"fastcgi,omitempty"`
}

// FastCGI permits fine-grained configuration of the fastcgi RoundTripper.
type FastCGIProbeWrapperConfig struct {
	// Root is the fastcgi root directory.
	// Defaults to the root directory of the container.
	Root string `json:"root,omitempty"`

	// The path in the URL will be split into two, with the first piece ending
	// with the value of SplitPath. The first piece will be assumed as the
	// actual resource (CGI script) name, and the second piece will be set to
	// PATH_INFO for the CGI script to use.
	SplitPath []string `json:"split_path,omitempty"`

	// Extra environment variables.
	EnvVars map[string]string `json:"env,omitempty"`

	// The duration used to set a deadline when connecting to an upstream.
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`

	// The duration used to set a deadline when reading from the FastCGI server.
	ReadTimeout time.Duration `json:"read_timeout,omitempty"`

	// The duration used to set a deadline when sending to the FastCGI server.
	WriteTimeout time.Duration `json:"write_timeout,omitempty"`
}

// HTTPProbeCmds is a list of HTTPProbeCmd instances
type HTTPProbeCmds struct {
	Commands []HTTPProbeCmd `json:"commands"`
}

// DockerClient provides Docker client parameters
type DockerClient struct {
	UseTLS      bool
	VerifyTLS   bool
	TLSCertPath string
	Host        string
	Env         map[string]string
}

const (
	CAMContainerProbe = "container-probe"
	CAMProbe          = "probe"
	CAMEnter          = "enter"
	CAMTimeout        = "timeout"
	CAMSignal         = "signal"
	CAMExec           = "exec"
)

// ContinueAfter provides the command execution mode parameters
type ContinueAfter struct {
	Mode         string
	Timeout      time.Duration
	ContinueChan <-chan struct{}
}
