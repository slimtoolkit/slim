package config

import (
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

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
	CAMProbe   = "probe"
	CAMEnter   = "enter"
	CAMTimeout = "timeout"
	CAMSignal  = "signal"
	CAMExec    = "exec"
)

// ContinueAfter provides the command execution mode parameters
type ContinueAfter struct {
	Mode         string
	Timeout      time.Duration
	ContinueChan <-chan struct{}
}
