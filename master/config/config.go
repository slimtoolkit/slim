package config

import (
	"github.com/cloudimmunity/go-dockerclientx"
)

type ContainerOverrides struct {
	Entrypoint      []string
	ClearEntrypoint bool
	Cmd             []string
	ClearCmd        bool
	Workdir         string
	Env             []string
	ExposedPorts    map[docker.Port]struct{}
}

type VolumeMount struct {
	Source      string
	Destination string
	Options     string
}

type HttpProbeCmd struct {
	Method   string   `json:"method"`
	Resource string   `json:"resource"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
	Headers  []string `json:"headers"`
	Body     string   `json:"body"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

type HttpProbeCmds struct {
	Commands []HttpProbeCmd `json:"commands"`
}

type DockerClient struct {
	UseTLS      bool
	VerifyTLS   bool
	TLSCertPath string
	Host        string
	Env         map[string]string
}
