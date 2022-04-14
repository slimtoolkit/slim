package docker

import (
	"context"

	"github.com/docker/buildx/driver"
	dockerclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
)

const prioritySupported = 10
const priorityUnsupported = 99

func init() {
	driver.Register(&factory{})
}

type factory struct {
}

func (*factory) Name() string {
	return "docker"
}

func (*factory) Usage() string {
	return "docker"
}

func (*factory) Priority(ctx context.Context, api dockerclient.APIClient) int {
	if api == nil {
		return priorityUnsupported
	}

	c, err := api.DialHijack(ctx, "/grpc", "h2c", nil)
	if err != nil {
		return priorityUnsupported
	}
	c.Close()

	return prioritySupported
}

func (f *factory) New(ctx context.Context, cfg driver.InitConfig) (driver.Driver, error) {
	if cfg.DockerAPI == nil {
		return nil, errors.Errorf("docker driver requires docker API access")
	}
	if len(cfg.Files) > 0 {
		return nil, errors.Errorf("setting config file is not supported for docker driver, use dockerd configuration file")
	}

	return &Driver{factory: f, InitConfig: cfg}, nil
}

func (f *factory) AllowsInstances() bool {
	return false
}
