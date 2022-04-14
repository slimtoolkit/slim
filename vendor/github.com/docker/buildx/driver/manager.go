package driver

import (
	"context"
	"io/ioutil"
	"sort"
	"strings"
	"sync"

	"k8s.io/client-go/rest"

	dockerclient "github.com/docker/docker/client"
	"github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type Factory interface {
	Name() string
	Usage() string
	Priority(context.Context, dockerclient.APIClient) int
	New(ctx context.Context, cfg InitConfig) (Driver, error)
	AllowsInstances() bool
}

type BuildkitConfig struct {
	// Entitlements []string
	// Rootless bool
}

type KubeClientConfig interface {
	ClientConfig() (*rest.Config, error)
	Namespace() (string, bool, error)
}

type KubeClientConfigInCluster struct{}

func (k KubeClientConfigInCluster) ClientConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func (k KubeClientConfigInCluster) Namespace() (string, bool, error) {
	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(namespace)), true, nil
}

type InitConfig struct {
	// This object needs updates to be generic for different drivers
	Name             string
	DockerAPI        dockerclient.APIClient
	KubeClientConfig KubeClientConfig
	BuildkitFlags    []string
	Files            map[string][]byte
	DriverOpts       map[string]string
	Auth             Auth
	Platforms        []specs.Platform
	// ContextPathHash can be used for determining pods in the driver instance
	ContextPathHash string
}

var drivers map[string]Factory

func Register(f Factory) {
	if drivers == nil {
		drivers = map[string]Factory{}
	}
	drivers[f.Name()] = f
}

func GetDefaultFactory(ctx context.Context, c dockerclient.APIClient, instanceRequired bool) (Factory, error) {
	if len(drivers) == 0 {
		return nil, errors.Errorf("no drivers available")
	}
	type p struct {
		f        Factory
		priority int
	}
	dd := make([]p, 0, len(drivers))
	for _, f := range drivers {
		if instanceRequired && !f.AllowsInstances() {
			continue
		}
		dd = append(dd, p{f: f, priority: f.Priority(ctx, c)})
	}
	sort.Slice(dd, func(i, j int) bool {
		return dd[i].priority < dd[j].priority
	})
	return dd[0].f, nil
}

func GetFactory(name string, instanceRequired bool) Factory {
	for _, f := range drivers {
		if instanceRequired && !f.AllowsInstances() {
			continue
		}
		if f.Name() == name {
			return f
		}
	}
	return nil
}

func GetDriver(ctx context.Context, name string, f Factory, api dockerclient.APIClient, auth Auth, kcc KubeClientConfig, flags []string, files map[string][]byte, do map[string]string, platforms []specs.Platform, contextPathHash string) (Driver, error) {
	ic := InitConfig{
		DockerAPI:        api,
		KubeClientConfig: kcc,
		Name:             name,
		BuildkitFlags:    flags,
		DriverOpts:       do,
		Auth:             auth,
		Platforms:        platforms,
		ContextPathHash:  contextPathHash,
		Files:            files,
	}
	if f == nil {
		var err error
		f, err = GetDefaultFactory(ctx, api, false)
		if err != nil {
			return nil, err
		}
	}
	d, err := f.New(ctx, ic)
	if err != nil {
		return nil, err
	}
	return &cachedDriver{Driver: d}, nil
}

func GetFactories() []Factory {
	ds := make([]Factory, 0, len(drivers))
	for _, d := range drivers {
		ds = append(ds, d)
	}
	sort.Slice(ds, func(i, j int) bool {
		return ds[i].Name() < ds[j].Name()
	})
	return ds
}

type cachedDriver struct {
	Driver
	client *client.Client
	err    error
	once   sync.Once
}

func (d *cachedDriver) Client(ctx context.Context) (*client.Client, error) {
	d.once.Do(func() {
		d.client, d.err = d.Driver.Client(ctx)
	})
	return d.client, d.err
}
