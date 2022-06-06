package client

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/cryptoservice"
	store "github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/signed"
)

// tufClient is a usability wrapper around a raw TUF repo
type tufClient struct {
	remote     store.RemoteStore
	cache      store.MetadataStore
	oldBuilder tuf.RepoBuilder
	newBuilder tuf.RepoBuilder
}

// Update performs an update to the TUF repo as defined by the TUF spec
func (c *tufClient) Update() (*tuf.Repo, *tuf.Repo, error) {
	// 1. Get timestamp
	//   a. If timestamp error (verification, expired, etc...) download new root and return to 1.
	// 2. Check if local snapshot is up to date
	//   a. If out of date, get updated snapshot
	//     i. If snapshot error, download new root and return to 1.
	// 3. Check if root correct against snapshot
	//   a. If incorrect, download new root and return to 1.
	// 4. Iteratively download and search targets and delegations to find target meta
	logrus.Debug("updating TUF client")
	err := c.update()
	if err != nil {
		logrus.Debug("Error occurred. Root will be downloaded and another update attempted")
		logrus.Debug("Resetting the TUF builder...")

		c.newBuilder = c.newBuilder.BootstrapNewBuilder()

		if err := c.updateRoot(); err != nil {
			logrus.Debug("Client Update (Root): ", err)
			return nil, nil, err
		}
		// If we error again, we now have the latest root and just want to fail
		// out as there's no expectation the problem can be resolved automatically
		logrus.Debug("retrying TUF client update")
		if err := c.update(); err != nil {
			return nil, nil, err
		}
	}
	return c.newBuilder.Finish()
}

func (c *tufClient) update() error {
	if err := c.downloadTimestamp(); err != nil {
		logrus.Debugf("Client Update (Timestamp): %s", err.Error())
		return err
	}
	if err := c.downloadSnapshot(); err != nil {
		logrus.Debugf("Client Update (Snapshot): %s", err.Error())
		return err
	}
	// will always need top level targets at a minimum
	if err := c.downloadTargets(); err != nil {
		logrus.Debugf("Client Update (Targets): %s", err.Error())
		return err
	}
	return nil
}

// updateRoot checks if there is a newer version of the root available, and if so
// downloads all intermediate root files to allow proper key rotation.
func (c *tufClient) updateRoot() error {
	// Get current root version
	currentRootConsistentInfo := c.oldBuilder.GetConsistentInfo(data.CanonicalRootRole)
	currentVersion := c.oldBuilder.GetLoadedVersion(currentRootConsistentInfo.RoleName)

	// Get new root version
	raw, err := c.downloadRoot()

	switch err.(type) {
	case *trustpinning.ErrRootRotationFail:
		// Rotation errors are okay since we haven't yet downloaded
		// all intermediate root files
		break
	case nil:
		// No error updating root - we were at most 1 version behind
		return nil
	default:
		// Return any non-rotation error.
		return err
	}

	// Load current version into newBuilder
	currentRaw, err := c.cache.GetSized(data.CanonicalRootRole.String(), -1)
	if err != nil {
		logrus.Debugf("error loading %d.%s: %s", currentVersion, data.CanonicalRootRole, err)
		return err
	}
	if err := c.newBuilder.LoadRootForUpdate(currentRaw, currentVersion, false); err != nil {
		logrus.Debugf("%d.%s is invalid: %s", currentVersion, data.CanonicalRootRole, err)
		return err
	}

	// Extract newest version number
	signedRoot := &data.Signed{}
	if err := json.Unmarshal(raw, signedRoot); err != nil {
		return err
	}
	newestRoot, err := data.RootFromSigned(signedRoot)
	if err != nil {
		return err
	}
	newestVersion := newestRoot.Signed.SignedCommon.Version

	// Update from current + 1 (current already loaded) to newest - 1 (newest loaded below)
	if err := c.updateRootVersions(currentVersion+1, newestVersion-1); err != nil {
		return err
	}

	// Already downloaded newest, verify it against newest - 1
	if err := c.newBuilder.LoadRootForUpdate(raw, newestVersion, true); err != nil {
		logrus.Debugf("downloaded %d.%s is invalid: %s", newestVersion, data.CanonicalRootRole, err)
		return err
	}
	logrus.Debugf("successfully verified downloaded %d.%s", newestVersion, data.CanonicalRootRole)

	// Write newest to cache
	if err := c.cache.Set(data.CanonicalRootRole.String(), raw); err != nil {
		logrus.Debugf("unable to write %d.%s to cache: %s", newestVersion, data.CanonicalRootRole, err)
	}
	logrus.Debugf("finished updating root files")
	return nil
}

// updateRootVersions updates the root from it's current version to a target, rotating keys
// as they are found
func (c *tufClient) updateRootVersions(fromVersion, toVersion int) error {
	for v := fromVersion; v <= toVersion; v++ {
		logrus.Debugf("updating root from version %d to version %d, currently fetching %d", fromVersion, toVersion, v)

		versionedRole := fmt.Sprintf("%d.%s", v, data.CanonicalRootRole)

		raw, err := c.remote.GetSized(versionedRole, -1)
		if err != nil {
			logrus.Debugf("error downloading %s: %s", versionedRole, err)
			return err
		}
		if err := c.newBuilder.LoadRootForUpdate(raw, v, false); err != nil {
			logrus.Debugf("downloaded %s is invalid: %s", versionedRole, err)
			return err
		}
		logrus.Debugf("successfully verified downloaded %s", versionedRole)
	}
	return nil
}

// downloadTimestamp is responsible for downloading the timestamp.json
// Timestamps are special in that we ALWAYS attempt to download and only
// use cache if the download fails (and the cache is still valid).
func (c *tufClient) downloadTimestamp() error {
	logrus.Debug("Loading timestamp...")
	role := data.CanonicalTimestampRole
	consistentInfo := c.newBuilder.GetConsistentInfo(role)

	// always get the remote timestamp, since it supersedes the local one
	cachedTS, cachedErr := c.cache.GetSized(role.String(), notary.MaxTimestampSize)
	_, remoteErr := c.tryLoadRemote(consistentInfo, cachedTS)

	// check that there was no remote error, or if there was a network problem
	// If there was a validation error, we should error out so we can download a new root or fail the update
	switch remoteErr.(type) {
	case nil:
		return nil
	case store.ErrMetaNotFound, store.ErrServerUnavailable, store.ErrOffline, store.NetworkError:
		break
	default:
		return remoteErr
	}

	// since it was a network error: get the cached timestamp, if it exists
	if cachedErr != nil {
		logrus.Debug("no cached or remote timestamp available")
		return remoteErr
	}

	logrus.Warn("Error while downloading remote metadata, using cached timestamp - this might not be the latest version available remotely")
	err := c.newBuilder.Load(role, cachedTS, 1, false)
	if err == nil {
		logrus.Debug("successfully verified cached timestamp")
	}
	return err

}

// downloadSnapshot is responsible for downloading the snapshot.json
func (c *tufClient) downloadSnapshot() error {
	logrus.Debug("Loading snapshot...")
	role := data.CanonicalSnapshotRole
	consistentInfo := c.newBuilder.GetConsistentInfo(role)

	_, err := c.tryLoadCacheThenRemote(consistentInfo)
	return err
}

// downloadTargets downloads all targets and delegated targets for the repository.
// It uses a pre-order tree traversal as it's necessary to download parents first
// to obtain the keys to validate children.
func (c *tufClient) downloadTargets() error {
	toDownload := []data.DelegationRole{{
		BaseRole: data.BaseRole{Name: data.CanonicalTargetsRole},
		Paths:    []string{""},
	}}

	for len(toDownload) > 0 {
		role := toDownload[0]
		toDownload = toDownload[1:]

		consistentInfo := c.newBuilder.GetConsistentInfo(role.Name)
		if !consistentInfo.ChecksumKnown() {
			logrus.Debugf("skipping %s because there is no checksum for it", role.Name)
			continue
		}

		children, err := c.getTargetsFile(role, consistentInfo)
		switch err.(type) {
		case signed.ErrExpired, signed.ErrRoleThreshold:
			if role.Name == data.CanonicalTargetsRole {
				return err
			}
			logrus.Warnf("Error getting %s: %s", role.Name, err)
			break
		case nil:
			toDownload = append(children, toDownload...)
		default:
			return err
		}
	}
	return nil
}

func (c tufClient) getTargetsFile(role data.DelegationRole, ci tuf.ConsistentInfo) ([]data.DelegationRole, error) {
	logrus.Debugf("Loading %s...", role.Name)
	tgs := &data.SignedTargets{}

	raw, err := c.tryLoadCacheThenRemote(ci)
	if err != nil {
		return nil, err
	}

	// we know it unmarshals because if `tryLoadCacheThenRemote` didn't fail, then
	// the raw has already been loaded into the builder
	json.Unmarshal(raw, tgs)
	return tgs.GetValidDelegations(role), nil
}

// downloadRoot is responsible for downloading the root.json
func (c *tufClient) downloadRoot() ([]byte, error) {
	role := data.CanonicalRootRole
	consistentInfo := c.newBuilder.GetConsistentInfo(role)

	// We can't read an exact size for the root metadata without risking getting stuck in the TUF update cycle
	// since it's possible that downloading timestamp/snapshot metadata may fail due to a signature mismatch
	if !consistentInfo.ChecksumKnown() {
		logrus.Debugf("Loading root with no expected checksum")

		// get the cached root, if it exists, just for version checking
		cachedRoot, _ := c.cache.GetSized(role.String(), -1)
		// prefer to download a new root
		return c.tryLoadRemote(consistentInfo, cachedRoot)
	}
	return c.tryLoadCacheThenRemote(consistentInfo)
}

func (c *tufClient) tryLoadCacheThenRemote(consistentInfo tuf.ConsistentInfo) ([]byte, error) {
	cachedTS, err := c.cache.GetSized(consistentInfo.RoleName.String(), consistentInfo.Length())
	if err != nil {
		logrus.Debugf("no %s in cache, must download", consistentInfo.RoleName)
		return c.tryLoadRemote(consistentInfo, nil)
	}

	if err = c.newBuilder.Load(consistentInfo.RoleName, cachedTS, 1, false); err == nil {
		logrus.Debugf("successfully verified cached %s", consistentInfo.RoleName)
		return cachedTS, nil
	}

	logrus.Debugf("cached %s is invalid (must download): %s", consistentInfo.RoleName, err)
	return c.tryLoadRemote(consistentInfo, cachedTS)
}

func (c *tufClient) tryLoadRemote(consistentInfo tuf.ConsistentInfo, old []byte) ([]byte, error) {
	consistentName := consistentInfo.ConsistentName()
	raw, err := c.remote.GetSized(consistentName, consistentInfo.Length())
	if err != nil {
		logrus.Debugf("error downloading %s: %s", consistentName, err)
		return old, err
	}

	// try to load the old data into the old builder - only use it to validate
	// versions if it loads successfully.  If it errors, then the loaded version
	// will be 1
	c.oldBuilder.Load(consistentInfo.RoleName, old, 1, true)
	minVersion := c.oldBuilder.GetLoadedVersion(consistentInfo.RoleName)
	if err := c.newBuilder.Load(consistentInfo.RoleName, raw, minVersion, false); err != nil {
		logrus.Debugf("downloaded %s is invalid: %s", consistentName, err)
		return raw, err
	}
	logrus.Debugf("successfully verified downloaded %s", consistentName)
	if err := c.cache.Set(consistentInfo.RoleName.String(), raw); err != nil {
		logrus.Debugf("Unable to write %s to cache: %s", consistentInfo.RoleName, err)
	}
	return raw, nil
}

// TUFLoadOptions are provided to LoadTUFRepo, which loads a TUF repo from cache,
// from a remote store, or both
type TUFLoadOptions struct {
	GUN                    data.GUN
	TrustPinning           trustpinning.TrustPinConfig
	CryptoService          signed.CryptoService
	Cache                  store.MetadataStore
	RemoteStore            store.RemoteStore
	AlwaysCheckInitialized bool
}

// bootstrapClient attempts to bootstrap a root.json to be used as the trust
// anchor for a repository. The checkInitialized argument indicates whether
// we should always attempt to contact the server to determine if the repository
// is initialized or not. If set to true, we will always attempt to download
// and return an error if the remote repository errors.
//
// Populates a tuf.RepoBuilder with this root metadata. If the root metadata
// downloaded is a newer version than what is on disk, then intermediate
// versions will be downloaded and verified in order to rotate trusted keys
// properly. Newer root metadata must always be signed with the previous
// threshold and keys.
//
// Fails if the remote server is reachable and does not know the repo
// (i.e. before any metadata has been published), in which case the error is
// store.ErrMetaNotFound, or if the root metadata (from whichever source is used)
// is not trusted.
//
// Returns a TUFClient for the remote server, which may not be actually
// operational (if the URL is invalid but a root.json is cached).
func bootstrapClient(l TUFLoadOptions) (*tufClient, error) {
	minVersion := 1
	// the old root on disk should not be validated against any trust pinning configuration
	// because if we have an old root, it itself is the thing that pins trust
	oldBuilder := tuf.NewRepoBuilder(l.GUN, l.CryptoService, trustpinning.TrustPinConfig{})

	// by default, we want to use the trust pinning configuration on any new root that we download
	newBuilder := tuf.NewRepoBuilder(l.GUN, l.CryptoService, l.TrustPinning)

	// Try to read root from cache first. We will trust this root until we detect a problem
	// during update which will cause us to download a new root and perform a rotation.
	// If we have an old root, and it's valid, then we overwrite the newBuilder to be one
	// preloaded with the old root or one which uses the old root for trust bootstrapping.
	if rootJSON, err := l.Cache.GetSized(data.CanonicalRootRole.String(), store.NoSizeLimit); err == nil {
		// if we can't load the cached root, fail hard because that is how we pin trust
		if err := oldBuilder.Load(data.CanonicalRootRole, rootJSON, minVersion, true); err != nil {
			return nil, err
		}

		// again, the root on disk is the source of trust pinning, so use an empty trust
		// pinning configuration
		newBuilder = tuf.NewRepoBuilder(l.GUN, l.CryptoService, trustpinning.TrustPinConfig{})

		if err := newBuilder.Load(data.CanonicalRootRole, rootJSON, minVersion, false); err != nil {
			// Ok, the old root is expired - we want to download a new one.  But we want to use the
			// old root to verify the new root, so bootstrap a new builder with the old builder
			// but use the trustpinning to validate the new root
			minVersion = oldBuilder.GetLoadedVersion(data.CanonicalRootRole)
			newBuilder = oldBuilder.BootstrapNewBuilderWithNewTrustpin(l.TrustPinning)
		}
	}

	if !newBuilder.IsLoaded(data.CanonicalRootRole) || l.AlwaysCheckInitialized {
		// remoteErr was nil and we were not able to load a root from cache or
		// are specifically checking for initialization of the repo.

		// if remote store successfully set up, try and get root from remote
		// We don't have any local data to determine the size of root, so try the maximum (though it is restricted at 100MB)
		tmpJSON, err := l.RemoteStore.GetSized(data.CanonicalRootRole.String(), store.NoSizeLimit)
		if err != nil {
			// we didn't have a root in cache and were unable to load one from
			// the server. Nothing we can do but error.
			return nil, err
		}

		if !newBuilder.IsLoaded(data.CanonicalRootRole) {
			// we always want to use the downloaded root if we couldn't load from cache
			if err := newBuilder.Load(data.CanonicalRootRole, tmpJSON, minVersion, false); err != nil {
				return nil, err
			}

			err = l.Cache.Set(data.CanonicalRootRole.String(), tmpJSON)
			if err != nil {
				// if we can't write cache we should still continue, just log error
				logrus.Errorf("could not save root to cache: %s", err.Error())
			}
		}
	}

	// We can only get here if remoteErr != nil (hence we don't download any new root),
	// and there was no root on disk
	if !newBuilder.IsLoaded(data.CanonicalRootRole) {
		return nil, ErrRepoNotInitialized{}
	}

	return &tufClient{
		oldBuilder: oldBuilder,
		newBuilder: newBuilder,
		remote:     l.RemoteStore,
		cache:      l.Cache,
	}, nil
}

// LoadTUFRepo bootstraps a trust anchor (root.json) from cache (if provided) before updating
// all the metadata for the repo from the remote (if provided). It loads a TUF repo from cache,
// from a remote store, or both.
func LoadTUFRepo(options TUFLoadOptions) (*tuf.Repo, *tuf.Repo, error) {
	// set some sane defaults, so nothing has to be provided necessarily
	if options.RemoteStore == nil {
		options.RemoteStore = store.OfflineStore{}
	}
	if options.Cache == nil {
		options.Cache = store.NewMemoryStore(nil)
	}
	if options.CryptoService == nil {
		options.CryptoService = cryptoservice.EmptyService
	}

	c, err := bootstrapClient(options)
	if err != nil {
		if _, ok := err.(store.ErrMetaNotFound); ok {
			return nil, nil, ErrRepositoryNotExist{
				remote: options.RemoteStore.Location(),
				gun:    options.GUN,
			}
		}
		return nil, nil, err
	}
	repo, invalid, err := c.Update()
	if err != nil {
		// notFound.Resource may include a version or checksum so when the role is root,
		// it will be root, <version>.root or root.<checksum>.
		notFound, ok := err.(store.ErrMetaNotFound)
		isRoot, _ := regexp.MatchString(`\.?`+data.CanonicalRootRole.String()+`\.?`, notFound.Resource)
		if ok && isRoot {
			return nil, nil, ErrRepositoryNotExist{
				remote: options.RemoteStore.Location(),
				gun:    options.GUN,
			}
		}
		return nil, nil, err
	}
	warnRolesNearExpiry(repo)
	return repo, invalid, nil
}
