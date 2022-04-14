package imagetools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/buildx/util/resolver"
	clitypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/util/tracing"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type Auth interface {
	GetAuthConfig(registryHostname string) (clitypes.AuthConfig, error)
}

type Opt struct {
	Auth           Auth
	RegistryConfig map[string]resolver.RegistryConfig
}

type Resolver struct {
	auth   docker.Authorizer
	hosts  docker.RegistryHosts
	buffer contentutil.Buffer
}

func New(opt Opt) *Resolver {
	return &Resolver{
		auth:   docker.NewDockerAuthorizer(docker.WithAuthCreds(toCredentialsFunc(opt.Auth)), docker.WithAuthClient(http.DefaultClient)),
		hosts:  resolver.NewRegistryConfig(opt.RegistryConfig),
		buffer: contentutil.NewBuffer(),
	}
}

func (r *Resolver) resolver() remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Hosts: func(domain string) ([]docker.RegistryHost, error) {
			res, err := r.hosts(domain)
			if err != nil {
				return nil, err
			}
			for i := range res {
				res[i].Authorizer = r.auth
				res[i].Client = tracing.DefaultClient
			}
			return res, nil
		},
	})
}

func (r *Resolver) Resolve(ctx context.Context, in string) (string, ocispec.Descriptor, error) {
	// discard containerd logger to avoid printing unnecessary info during image reference resolution.
	// https://github.com/containerd/containerd/blob/1a88cf5242445657258e0c744def5017d7cfb492/remotes/docker/resolver.go#L288
	logger := logrus.New()
	logger.Out = io.Discard
	ctx = log.WithLogger(ctx, logrus.NewEntry(logger))

	ref, err := parseRef(in)
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	in, desc, err := r.resolver().Resolve(ctx, ref.String())
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	return in, desc, nil
}

func (r *Resolver) Get(ctx context.Context, in string) ([]byte, ocispec.Descriptor, error) {
	in, desc, err := r.Resolve(ctx, in)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	dt, err := r.GetDescriptor(ctx, in, desc)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}
	return dt, desc, nil
}

func (r *Resolver) GetDescriptor(ctx context.Context, in string, desc ocispec.Descriptor) ([]byte, error) {
	fetcher, err := r.resolver().Fetcher(ctx, in)
	if err != nil {
		return nil, err
	}

	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, rc)
	rc.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func parseRef(s string) (reference.Named, error) {
	ref, err := reference.ParseNormalizedNamed(s)
	if err != nil {
		return nil, err
	}
	ref = reference.TagNameOnly(ref)
	return ref, nil
}

func toCredentialsFunc(a Auth) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		if host == "registry-1.docker.io" {
			host = "https://index.docker.io/v1/"
		}
		ac, err := a.GetAuthConfig(host)
		if err != nil {
			return "", "", err
		}
		if ac.IdentityToken != "" {
			return "", ac.IdentityToken, nil
		}
		return ac.Username, ac.Password, nil
	}
}

func RegistryAuthForRef(ref string, a Auth) (string, error) {
	if a == nil {
		return "", nil
	}
	r, err := parseRef(ref)
	if err != nil {
		return "", err
	}
	host := reference.Domain(r)
	if host == "docker.io" {
		host = "https://index.docker.io/v1/"
	}
	ac, err := a.GetAuthConfig(host)
	if err != nil {
		return "", err
	}
	buf, err := json.Marshal(ac)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

func (r *Resolver) ImageConfig(ctx context.Context, in string, platform *ocispec.Platform) (digest.Digest, []byte, error) {
	in, _, err := r.Resolve(ctx, in)
	if err != nil {
		return "", nil, err
	}
	return imageutil.Config(ctx, in, r.resolver(), r.buffer, nil, platform)
}
