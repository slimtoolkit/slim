package storeutil

import (
	"bytes"
	"os"
	"strings"

	"github.com/docker/buildx/store"
	"github.com/docker/buildx/util/confutil"
	"github.com/docker/buildx/util/imagetools"
	"github.com/docker/buildx/util/resolver"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	buildkitdconfig "github.com/moby/buildkit/cmd/buildkitd/config"
	"github.com/pkg/errors"
)

// GetStore returns current builder instance store
func GetStore(dockerCli command.Cli) (*store.Txn, func(), error) {
	s, err := store.New(confutil.ConfigDir(dockerCli))
	if err != nil {
		return nil, nil, err
	}
	return s.Txn()
}

// GetCurrentEndpoint returns the current default endpoint value
func GetCurrentEndpoint(dockerCli command.Cli) (string, error) {
	name := dockerCli.CurrentContext()
	if name != "default" {
		return name, nil
	}
	de, err := GetDockerEndpoint(dockerCli, name)
	if err != nil {
		return "", errors.Errorf("docker endpoint for %q not found", name)
	}
	return de, nil
}

func GetProxyConfig(dockerCli command.Cli) map[string]string {
	cfg := dockerCli.ConfigFile()
	host := dockerCli.Client().DaemonHost()

	proxy, ok := cfg.Proxies[host]
	if !ok {
		proxy = cfg.Proxies["default"]
	}

	m := map[string]string{}

	if v := proxy.HTTPProxy; v != "" {
		m["HTTP_PROXY"] = v
	}
	if v := proxy.HTTPSProxy; v != "" {
		m["HTTPS_PROXY"] = v
	}
	if v := proxy.NoProxy; v != "" {
		m["NO_PROXY"] = v
	}
	if v := proxy.FTPProxy; v != "" {
		m["FTP_PROXY"] = v
	}
	return m
}

// GetDockerEndpoint returns docker endpoint string for given context
func GetDockerEndpoint(dockerCli command.Cli, name string) (string, error) {
	list, err := dockerCli.ContextStore().List()
	if err != nil {
		return "", err
	}
	for _, l := range list {
		if l.Name == name {
			ep, ok := l.Endpoints["docker"]
			if !ok {
				return "", errors.Errorf("context %q does not have a Docker endpoint", name)
			}
			typed, ok := ep.(docker.EndpointMeta)
			if !ok {
				return "", errors.Errorf("endpoint %q is not of type EndpointMeta, %T", ep, ep)
			}
			return typed.Host, nil
		}
	}
	return "", nil
}

// GetCurrentInstance finds the current builder instance
func GetCurrentInstance(txn *store.Txn, dockerCli command.Cli) (*store.NodeGroup, error) {
	ep, err := GetCurrentEndpoint(dockerCli)
	if err != nil {
		return nil, err
	}
	ng, err := txn.Current(ep)
	if err != nil {
		return nil, err
	}
	if ng == nil {
		ng, _ = GetNodeGroup(txn, dockerCli, dockerCli.CurrentContext())
	}

	return ng, nil
}

// GetNodeGroup returns nodegroup based on the name
func GetNodeGroup(txn *store.Txn, dockerCli command.Cli, name string) (*store.NodeGroup, error) {
	ng, err := txn.NodeGroupByName(name)
	if err != nil {
		if !os.IsNotExist(errors.Cause(err)) {
			return nil, err
		}
	}
	if ng != nil {
		return ng, nil
	}

	if name == "default" {
		name = dockerCli.CurrentContext()
	}

	list, err := dockerCli.ContextStore().List()
	if err != nil {
		return nil, err
	}
	for _, l := range list {
		if l.Name == name {
			return &store.NodeGroup{
				Name: "default",
				Nodes: []store.Node{
					{
						Name:     "default",
						Endpoint: name,
					},
				},
			}, nil
		}
	}

	return nil, errors.Errorf("no builder %q found", name)
}

func GetImageConfig(dockerCli command.Cli, ng *store.NodeGroup) (opt imagetools.Opt, err error) {
	opt.Auth = dockerCli.ConfigFile()

	if ng == nil || len(ng.Nodes) == 0 {
		return opt, nil
	}

	files := ng.Nodes[0].Files

	dt, ok := files["buildkitd.toml"]
	if !ok {
		return opt, nil
	}

	config, err := buildkitdconfig.Load(bytes.NewReader(dt))
	if err != nil {
		return opt, err
	}

	regconfig := make(map[string]resolver.RegistryConfig)

	for k, v := range config.Registries {
		rc := resolver.RegistryConfig{
			Mirrors:   v.Mirrors,
			PlainHTTP: v.PlainHTTP,
			Insecure:  v.Insecure,
		}
		for _, ca := range v.RootCAs {
			dt, ok := files[strings.TrimPrefix(ca, confutil.DefaultBuildKitConfigDir+"/")]
			if ok {
				rc.RootCAs = append(rc.RootCAs, dt)
			}
		}

		for _, kp := range v.KeyPairs {
			key, keyok := files[strings.TrimPrefix(kp.Key, confutil.DefaultBuildKitConfigDir+"/")]
			cert, certok := files[strings.TrimPrefix(kp.Certificate, confutil.DefaultBuildKitConfigDir+"/")]
			if keyok && certok {
				rc.KeyPairs = append(rc.KeyPairs, resolver.TLSKeyPair{
					Key:         key,
					Certificate: cert,
				})
			}
		}
		regconfig[k] = rc
	}

	opt.RegistryConfig = regconfig

	return opt, nil
}
