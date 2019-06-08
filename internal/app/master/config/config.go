package config

import (
	"time"

	"github.com/cloudimmunity/go-dockerclientx"
)

// ContainerOverrides provides a set of container field overrides
// It can also be used to update the image instructions when
// the "image-overrides" flag is provided
type ContainerOverrides struct {
	Entrypoint      []string
	ClearEntrypoint bool
	Cmd             []string
	ClearCmd        bool
	Workdir         string
	Env             []string
	Hostname        string
	Network         string
	ExposedPorts    map[docker.Port]struct{}
}

// ImageNewInstructions provides a set new image instructions
type ImageNewInstructions struct {
	Entrypoint      []string
	ClearEntrypoint bool
	Cmd             []string
	ClearCmd        bool
	Workdir         string
	Env             []string
	ExposedPorts    map[docker.Port]struct{}
}

// VolumeMount provides the volume mount configuration information
type VolumeMount struct {
	Source      string
	Destination string
	Options     string
}

// HTTPProbeCmd provides the HTTP probe parameters
type HTTPProbeCmd struct {
	Method   string   `json:"method"`
	Resource string   `json:"resource"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
	Headers  []string `json:"headers"`
	Body     string   `json:"body"`
	Username string   `json:"username"`
	Password string   `json:"password"`
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

// ContinueAfter provides the command execution mode parameters
type ContinueAfter struct {
	Mode         string
	Timeout      time.Duration
	ContinueChan <-chan struct{}
}
