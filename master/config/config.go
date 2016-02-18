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
