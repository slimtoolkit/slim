package driver

import (
	"context"

	"github.com/docker/buildx/store"
	"github.com/docker/buildx/util/progress"
	clitypes "github.com/docker/cli/cli/config/types"
	"github.com/moby/buildkit/client"
	"github.com/pkg/errors"
)

var ErrNotRunning = errors.Errorf("driver not running")
var ErrNotConnecting = errors.Errorf("driver not connecting")

type Status int

const (
	Inactive Status = iota
	Starting
	Running
	Stopping
	Stopped
)

func (s Status) String() string {
	switch s {
	case Inactive:
		return "inactive"
	case Starting:
		return "starting"
	case Running:
		return "running"
	case Stopping:
		return "stopping"
	case Stopped:
		return "stopped"
	}
	return "unknown"
}

type Info struct {
	Status Status
	// DynamicNodes must be empty if the actual nodes are statically listed in the store
	DynamicNodes []store.Node
}

type Auth interface {
	GetAuthConfig(registryHostname string) (clitypes.AuthConfig, error)
}

type Driver interface {
	Factory() Factory
	Bootstrap(context.Context, progress.Logger) error
	Info(context.Context) (*Info, error)
	Stop(ctx context.Context, force bool) error
	Rm(ctx context.Context, force, rmVolume, rmDaemon bool) error
	Client(ctx context.Context) (*client.Client, error)
	Features() map[Feature]bool
	IsMobyDriver() bool
	Config() InitConfig
}

func Boot(ctx, clientContext context.Context, d Driver, pw progress.Writer) (*client.Client, error) {
	try := 0
	for {
		info, err := d.Info(ctx)
		if err != nil {
			return nil, err
		}
		try++
		if info.Status != Running {
			if try > 2 {
				return nil, errors.Errorf("failed to bootstrap %T driver in attempts", d)
			}
			if err := d.Bootstrap(ctx, pw.Write); err != nil {
				return nil, err
			}
		}

		c, err := d.Client(clientContext)
		if err != nil {
			if errors.Cause(err) == ErrNotRunning && try <= 2 {
				continue
			}
			return nil, err
		}
		return c, nil
	}
}
