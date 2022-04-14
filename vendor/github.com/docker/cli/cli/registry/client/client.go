package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	manifesttypes "github.com/docker/cli/cli/manifest/types"
	"github.com/docker/cli/cli/trust"
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	distributionclient "github.com/docker/distribution/registry/client"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// RegistryClient is a client used to communicate with a Docker distribution
// registry
type RegistryClient interface {
	GetManifest(ctx context.Context, ref reference.Named) (manifesttypes.ImageManifest, error)
	GetManifestList(ctx context.Context, ref reference.Named) ([]manifesttypes.ImageManifest, error)
	MountBlob(ctx context.Context, source reference.Canonical, target reference.Named) error
	PutManifest(ctx context.Context, ref reference.Named, manifest distribution.Manifest) (digest.Digest, error)
	GetTags(ctx context.Context, ref reference.Named) ([]string, error)
}

// NewRegistryClient returns a new RegistryClient with a resolver
func NewRegistryClient(resolver AuthConfigResolver, userAgent string, insecure bool) RegistryClient {
	return &client{
		authConfigResolver: resolver,
		insecureRegistry:   insecure,
		userAgent:          userAgent,
	}
}

// AuthConfigResolver returns Auth Configuration for an index
type AuthConfigResolver func(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig

// PutManifestOptions is the data sent to push a manifest
type PutManifestOptions struct {
	MediaType string
	Payload   []byte
}

type client struct {
	authConfigResolver AuthConfigResolver
	insecureRegistry   bool
	userAgent          string
}

// ErrBlobCreated returned when a blob mount request was created
type ErrBlobCreated struct {
	From   reference.Named
	Target reference.Named
}

func (err ErrBlobCreated) Error() string {
	return fmt.Sprintf("blob mounted from: %v to: %v",
		err.From, err.Target)
}

// ErrHTTPProto returned if attempting to use TLS with a non-TLS registry
type ErrHTTPProto struct {
	OrigErr string
}

func (err ErrHTTPProto) Error() string {
	return err.OrigErr
}

var _ RegistryClient = &client{}

// MountBlob into the registry, so it can be referenced by a manifest
func (c *client) MountBlob(ctx context.Context, sourceRef reference.Canonical, targetRef reference.Named) error {
	repoEndpoint, err := newDefaultRepositoryEndpoint(targetRef, c.insecureRegistry)
	if err != nil {
		return err
	}
	repo, err := c.getRepositoryForReference(ctx, targetRef, repoEndpoint)
	if err != nil {
		return err
	}
	lu, err := repo.Blobs(ctx).Create(ctx, distributionclient.WithMountFrom(sourceRef))
	switch err.(type) {
	case distribution.ErrBlobMounted:
		logrus.Debugf("mount of blob %s succeeded", sourceRef)
		return nil
	case nil:
	default:
		return errors.Wrapf(err, "failed to mount blob %s to %s", sourceRef, targetRef)
	}
	lu.Cancel(ctx)
	logrus.Debugf("mount of blob %s created", sourceRef)
	return ErrBlobCreated{From: sourceRef, Target: targetRef}
}

// PutManifest sends the manifest to a registry and returns the new digest
func (c *client) PutManifest(ctx context.Context, ref reference.Named, manifest distribution.Manifest) (digest.Digest, error) {
	repoEndpoint, err := newDefaultRepositoryEndpoint(ref, c.insecureRegistry)
	if err != nil {
		return digest.Digest(""), err
	}

	repo, err := c.getRepositoryForReference(ctx, ref, repoEndpoint)
	if err != nil {
		return digest.Digest(""), err
	}

	manifestService, err := repo.Manifests(ctx)
	if err != nil {
		return digest.Digest(""), err
	}

	_, opts, err := getManifestOptionsFromReference(ref)
	if err != nil {
		return digest.Digest(""), err
	}

	dgst, err := manifestService.Put(ctx, manifest, opts...)
	return dgst, errors.Wrapf(err, "failed to put manifest %s", ref)
}

func (c *client) GetTags(ctx context.Context, ref reference.Named) ([]string, error) {
	repoEndpoint, err := newDefaultRepositoryEndpoint(ref, c.insecureRegistry)
	if err != nil {
		return nil, err
	}

	repo, err := c.getRepositoryForReference(ctx, ref, repoEndpoint)
	if err != nil {
		return nil, err
	}
	return repo.Tags(ctx).All(ctx)
}

func (c *client) getRepositoryForReference(ctx context.Context, ref reference.Named, repoEndpoint repositoryEndpoint) (distribution.Repository, error) {
	repoName, err := reference.WithName(repoEndpoint.Name())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse repo name from %s", ref)
	}
	httpTransport, err := c.getHTTPTransportForRepoEndpoint(ctx, repoEndpoint)
	if err != nil {
		if !strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
			return nil, err
		}
		if !repoEndpoint.endpoint.TLSConfig.InsecureSkipVerify {
			return nil, ErrHTTPProto{OrigErr: err.Error()}
		}
		// --insecure was set; fall back to plain HTTP
		if url := repoEndpoint.endpoint.URL; url != nil && url.Scheme == "https" {
			url.Scheme = "http"
			httpTransport, err = c.getHTTPTransportForRepoEndpoint(ctx, repoEndpoint)
			if err != nil {
				return nil, err
			}
		}
	}
	return distributionclient.NewRepository(repoName, repoEndpoint.BaseURL(), httpTransport)
}

func (c *client) getHTTPTransportForRepoEndpoint(ctx context.Context, repoEndpoint repositoryEndpoint) (http.RoundTripper, error) {
	httpTransport, err := getHTTPTransport(
		c.authConfigResolver(ctx, repoEndpoint.info.Index),
		repoEndpoint.endpoint,
		repoEndpoint.Name(),
		c.userAgent)
	return httpTransport, errors.Wrap(err, "failed to configure transport")
}

// GetManifest returns an ImageManifest for the reference
func (c *client) GetManifest(ctx context.Context, ref reference.Named) (manifesttypes.ImageManifest, error) {
	var result manifesttypes.ImageManifest
	fetch := func(ctx context.Context, repo distribution.Repository, ref reference.Named) (bool, error) {
		var err error
		result, err = fetchManifest(ctx, repo, ref)
		return result.Ref != nil, err
	}

	err := c.iterateEndpoints(ctx, ref, fetch)
	return result, err
}

// GetManifestList returns a list of ImageManifest for the reference
func (c *client) GetManifestList(ctx context.Context, ref reference.Named) ([]manifesttypes.ImageManifest, error) {
	result := []manifesttypes.ImageManifest{}
	fetch := func(ctx context.Context, repo distribution.Repository, ref reference.Named) (bool, error) {
		var err error
		result, err = fetchList(ctx, repo, ref)
		return len(result) > 0, err
	}

	err := c.iterateEndpoints(ctx, ref, fetch)
	return result, err
}

func getManifestOptionsFromReference(ref reference.Named) (digest.Digest, []distribution.ManifestServiceOption, error) {
	if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
		tag := tagged.Tag()
		return "", []distribution.ManifestServiceOption{distribution.WithTag(tag)}, nil
	}
	if digested, isDigested := ref.(reference.Canonical); isDigested {
		return digested.Digest(), []distribution.ManifestServiceOption{}, nil
	}
	return "", nil, errors.Errorf("%s no tag or digest", ref)
}

// GetRegistryAuth returns the auth config given an input image
func GetRegistryAuth(ctx context.Context, resolver AuthConfigResolver, imageName string) (*types.AuthConfig, error) {
	distributionRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse image name: %s: %s", imageName, err)
	}
	imgRefAndAuth, err := trust.GetImageReferencesAndAuth(ctx, nil, resolver, distributionRef.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to get imgRefAndAuth: %s", err)
	}
	return imgRefAndAuth.AuthConfig(), nil
}
