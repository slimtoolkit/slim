package trust

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/tlsconfig"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/passphrase"
	"github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/signed"
)

var (
	// ReleasesRole is the role named "releases"
	ReleasesRole = data.RoleName(path.Join(data.CanonicalTargetsRole.String(), "releases"))
	// ActionsPullOnly defines the actions for read-only interactions with a Notary Repository
	ActionsPullOnly = []string{"pull"}
	// ActionsPushAndPull defines the actions for read-write interactions with a Notary Repository
	ActionsPushAndPull = []string{"pull", "push"}
	// NotaryServer is the endpoint serving the Notary trust server
	NotaryServer = "https://notary.docker.io"
)

// GetTrustDirectory returns the base trust directory name
func GetTrustDirectory() string {
	return filepath.Join(cliconfig.Dir(), "trust")
}

// certificateDirectory returns the directory containing
// TLS certificates for the given server. An error is
// returned if there was an error parsing the server string.
func certificateDirectory(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}

	return filepath.Join(cliconfig.Dir(), "tls", u.Host), nil
}

// Server returns the base URL for the trust server.
func Server(index *registrytypes.IndexInfo) (string, error) {
	if s := os.Getenv("DOCKER_CONTENT_TRUST_SERVER"); s != "" {
		urlObj, err := url.Parse(s)
		if err != nil || urlObj.Scheme != "https" {
			return "", errors.Errorf("valid https URL required for trust server, got %s", s)
		}

		return s, nil
	}
	if index.Official {
		return NotaryServer, nil
	}
	return "https://" + index.Name, nil
}

type simpleCredentialStore struct {
	auth types.AuthConfig
}

func (scs simpleCredentialStore) Basic(u *url.URL) (string, string) {
	return scs.auth.Username, scs.auth.Password
}

func (scs simpleCredentialStore) RefreshToken(u *url.URL, service string) string {
	return scs.auth.IdentityToken
}

func (scs simpleCredentialStore) SetRefreshToken(*url.URL, string, string) {
}

// GetNotaryRepository returns a NotaryRepository which stores all the
// information needed to operate on a notary repository.
// It creates an HTTP transport providing authentication support.
func GetNotaryRepository(in io.Reader, out io.Writer, userAgent string, repoInfo *registry.RepositoryInfo, authConfig *types.AuthConfig, actions ...string) (client.Repository, error) {
	server, err := Server(repoInfo.Index)
	if err != nil {
		return nil, err
	}

	var cfg = tlsconfig.ClientDefault()
	cfg.InsecureSkipVerify = !repoInfo.Index.Secure

	// Get certificate base directory
	certDir, err := certificateDirectory(server)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("reading certificate directory: %s", certDir)

	if err := registry.ReadCertsDirectory(cfg, certDir); err != nil {
		return nil, err
	}

	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
		DisableKeepAlives:   true,
	}

	// Skip configuration headers since request is not going to Docker daemon
	modifiers := registry.Headers(userAgent, http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}
	endpointStr := server + "/v2/"
	req, err := http.NewRequest("GET", endpointStr, nil)
	if err != nil {
		return nil, err
	}

	challengeManager := challenge.NewSimpleManager()

	resp, err := pingClient.Do(req)
	if err != nil {
		// Ignore error on ping to operate in offline mode
		logrus.Debugf("Error pinging notary server %q: %s", endpointStr, err)
	} else {
		defer resp.Body.Close()

		// Add response to the challenge manager to parse out
		// authentication header and register authentication method
		if err := challengeManager.AddResponse(resp); err != nil {
			return nil, err
		}
	}

	scope := auth.RepositoryScope{
		Repository: repoInfo.Name.Name(),
		Actions:    actions,
		Class:      repoInfo.Class,
	}
	creds := simpleCredentialStore{auth: *authConfig}
	tokenHandlerOptions := auth.TokenHandlerOptions{
		Transport:   authTransport,
		Credentials: creds,
		Scopes:      []auth.Scope{scope},
		ClientID:    registry.AuthClientID,
	}
	tokenHandler := auth.NewTokenHandlerWithOptions(tokenHandlerOptions)
	basicHandler := auth.NewBasicHandler(creds)
	modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	tr := transport.NewTransport(base, modifiers...)

	return client.NewFileCachedRepository(
		GetTrustDirectory(),
		data.GUN(repoInfo.Name.Name()),
		server,
		tr,
		GetPassphraseRetriever(in, out),
		trustpinning.TrustPinConfig{})
}

// GetPassphraseRetriever returns a passphrase retriever that utilizes Content Trust env vars
func GetPassphraseRetriever(in io.Reader, out io.Writer) notary.PassRetriever {
	aliasMap := map[string]string{
		"root":     "root",
		"snapshot": "repository",
		"targets":  "repository",
		"default":  "repository",
	}
	baseRetriever := passphrase.PromptRetrieverWithInOut(in, out, aliasMap)
	env := map[string]string{
		"root":     os.Getenv("DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE"),
		"snapshot": os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"targets":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"default":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
	}

	return func(keyName string, alias string, createNew bool, numAttempts int) (string, bool, error) {
		if v := env[alias]; v != "" {
			return v, numAttempts > 1, nil
		}
		// For non-root roles, we can also try the "default" alias if it is specified
		if v := env["default"]; v != "" && alias != data.CanonicalRootRole.String() {
			return v, numAttempts > 1, nil
		}
		return baseRetriever(keyName, alias, createNew, numAttempts)
	}
}

// NotaryError formats an error message received from the notary service
func NotaryError(repoName string, err error) error {
	switch err.(type) {
	case *json.SyntaxError:
		logrus.Debugf("Notary syntax error: %s", err)
		return errors.Errorf("Error: no trust data available for remote repository %s. Try running notary server and setting DOCKER_CONTENT_TRUST_SERVER to its HTTPS address?", repoName)
	case signed.ErrExpired:
		return errors.Errorf("Error: remote repository %s out-of-date: %v", repoName, err)
	case trustmanager.ErrKeyNotFound:
		return errors.Errorf("Error: signing keys for remote repository %s not found: %v", repoName, err)
	case storage.NetworkError:
		return errors.Errorf("Error: error contacting notary server: %v", err)
	case storage.ErrMetaNotFound:
		return errors.Errorf("Error: trust data missing for remote repository %s or remote repository not found: %v", repoName, err)
	case trustpinning.ErrRootRotationFail, trustpinning.ErrValidationFail, signed.ErrInvalidKeyType:
		return errors.Errorf("Warning: potential malicious behavior - trust data mismatch for remote repository %s: %v", repoName, err)
	case signed.ErrNoKeys:
		return errors.Errorf("Error: could not find signing keys for remote repository %s, or could not decrypt signing key: %v", repoName, err)
	case signed.ErrLowVersion:
		return errors.Errorf("Warning: potential malicious behavior - trust data version is lower than expected for remote repository %s: %v", repoName, err)
	case signed.ErrRoleThreshold:
		return errors.Errorf("Warning: potential malicious behavior - trust data has insufficient signatures for remote repository %s: %v", repoName, err)
	case client.ErrRepositoryNotExist:
		return errors.Errorf("Error: remote trust data does not exist for %s: %v", repoName, err)
	case signed.ErrInsufficientSignatures:
		return errors.Errorf("Error: could not produce valid signature for %s.  If Yubikey was used, was touch input provided?: %v", repoName, err)
	}

	return err
}

// GetSignableRoles returns a list of roles for which we have valid signing
// keys, given a notary repository and a target
func GetSignableRoles(repo client.Repository, target *client.Target) ([]data.RoleName, error) {
	var signableRoles []data.RoleName

	// translate the full key names, which includes the GUN, into just the key IDs
	allCanonicalKeyIDs := make(map[string]struct{})
	for fullKeyID := range repo.GetCryptoService().ListAllKeys() {
		allCanonicalKeyIDs[path.Base(fullKeyID)] = struct{}{}
	}

	allDelegationRoles, err := repo.GetDelegationRoles()
	if err != nil {
		return signableRoles, err
	}

	// if there are no delegation roles, then just try to sign it into the targets role
	if len(allDelegationRoles) == 0 {
		signableRoles = append(signableRoles, data.CanonicalTargetsRole)
		return signableRoles, nil
	}

	// there are delegation roles, find every delegation role we have a key for, and
	// attempt to sign into into all those roles.
	for _, delegationRole := range allDelegationRoles {
		// We do not support signing any delegation role that isn't a direct child of the targets role.
		// Also don't bother checking the keys if we can't add the target
		// to this role due to path restrictions
		if path.Dir(delegationRole.Name.String()) != data.CanonicalTargetsRole.String() || !delegationRole.CheckPaths(target.Name) {
			continue
		}

		for _, canonicalKeyID := range delegationRole.KeyIDs {
			if _, ok := allCanonicalKeyIDs[canonicalKeyID]; ok {
				signableRoles = append(signableRoles, delegationRole.Name)
				break
			}
		}
	}

	if len(signableRoles) == 0 {
		return signableRoles, errors.Errorf("no valid signing keys for delegation roles")
	}

	return signableRoles, nil

}

// ImageRefAndAuth contains all reference information and the auth config for an image request
type ImageRefAndAuth struct {
	original   string
	authConfig *types.AuthConfig
	reference  reference.Named
	repoInfo   *registry.RepositoryInfo
	tag        string
	digest     digest.Digest
}

// GetImageReferencesAndAuth retrieves the necessary reference and auth information for an image name
// as an ImageRefAndAuth struct
func GetImageReferencesAndAuth(ctx context.Context, rs registry.Service,
	authResolver func(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig,
	imgName string,
) (ImageRefAndAuth, error) {
	ref, err := reference.ParseNormalizedNamed(imgName)
	if err != nil {
		return ImageRefAndAuth{}, err
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	var repoInfo *registry.RepositoryInfo
	if rs != nil {
		repoInfo, err = rs.ResolveRepository(ref)
	} else {
		repoInfo, err = registry.ParseRepositoryInfo(ref)
	}

	if err != nil {
		return ImageRefAndAuth{}, err
	}

	authConfig := authResolver(ctx, repoInfo.Index)
	return ImageRefAndAuth{
		original:   imgName,
		authConfig: &authConfig,
		reference:  ref,
		repoInfo:   repoInfo,
		tag:        getTag(ref),
		digest:     getDigest(ref),
	}, nil
}

func getTag(ref reference.Named) string {
	switch x := ref.(type) {
	case reference.Canonical, reference.Digested:
		return ""
	case reference.NamedTagged:
		return x.Tag()
	default:
		return ""
	}
}

func getDigest(ref reference.Named) digest.Digest {
	switch x := ref.(type) {
	case reference.Canonical:
		return x.Digest()
	case reference.Digested:
		return x.Digest()
	default:
		return digest.Digest("")
	}
}

// AuthConfig returns the auth information (username, etc) for a given ImageRefAndAuth
func (imgRefAuth *ImageRefAndAuth) AuthConfig() *types.AuthConfig {
	return imgRefAuth.authConfig
}

// Reference returns the Image reference for a given ImageRefAndAuth
func (imgRefAuth *ImageRefAndAuth) Reference() reference.Named {
	return imgRefAuth.reference
}

// RepoInfo returns the repository information for a given ImageRefAndAuth
func (imgRefAuth *ImageRefAndAuth) RepoInfo() *registry.RepositoryInfo {
	return imgRefAuth.repoInfo
}

// Tag returns the Image tag for a given ImageRefAndAuth
func (imgRefAuth *ImageRefAndAuth) Tag() string {
	return imgRefAuth.tag
}

// Digest returns the Image digest for a given ImageRefAndAuth
func (imgRefAuth *ImageRefAndAuth) Digest() digest.Digest {
	return imgRefAuth.digest
}

// Name returns the image name used to initialize the ImageRefAndAuth
func (imgRefAuth *ImageRefAndAuth) Name() string {
	return imgRefAuth.original

}
