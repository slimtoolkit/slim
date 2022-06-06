package client

import (
	"github.com/theupdateframework/notary/client/changelist"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/signed"
)

// ReadOnly represents the set of options that must be supported over a TUF repo for
// reading
type ReadOnly interface {
	// ListTargets lists all targets for the current repository. The list of
	// roles should be passed in order from highest to lowest priority.
	//
	// IMPORTANT: if you pass a set of roles such as [ "targets/a", "targets/x"
	// "targets/a/b" ], even though "targets/a/b" is part of the "targets/a" subtree
	// its entries will be strictly shadowed by those in other parts of the "targets/a"
	// subtree and also the "targets/x" subtree, as we will defer parsing it until
	// we explicitly reach it in our iteration of the provided list of roles.
	ListTargets(roles ...data.RoleName) ([]*TargetWithRole, error)

	// GetTargetByName returns a target by the given name. If no roles are passed
	// it uses the targets role and does a search of the entire delegation
	// graph, finding the first entry in a breadth first search of the delegations.
	// If roles are passed, they should be passed in descending priority and
	// the target entry found in the subtree of the highest priority role
	// will be returned.
	// See the IMPORTANT section on ListTargets above. Those roles also apply here.
	GetTargetByName(name string, roles ...data.RoleName) (*TargetWithRole, error)

	// GetAllTargetMetadataByName searches the entire delegation role tree to find
	// the specified target by name for all roles, and returns a list of
	// TargetSignedStructs for each time it finds the specified target.
	// If given an empty string for a target name, it will return back all targets
	// signed into the repository in every role
	GetAllTargetMetadataByName(name string) ([]TargetSignedStruct, error)

	// ListRoles returns a list of RoleWithSignatures objects for this repo
	// This represents the latest metadata for each role in this repo
	ListRoles() ([]RoleWithSignatures, error)

	// GetDelegationRoles returns the keys and roles of the repository's delegations
	// Also converts key IDs to canonical key IDs to keep consistent with signing prompts
	GetDelegationRoles() ([]data.Role, error)
}

// Repository represents the set of options that must be supported over a TUF repo
// for both reading and writing.
type Repository interface {
	ReadOnly

	// ------------------- Publishing operations -------------------

	// GetGUN returns the GUN associated with the repository
	GetGUN() data.GUN

	// SetLegacyVersion sets the number of versions back to fetch roots to sign with
	SetLegacyVersions(int)

	// ----- General management operations -----

	// Initialize creates a new repository by using rootKey as the root Key for the
	// TUF repository. The remote store/server must be reachable (and is asked to
	// generate a timestamp key and possibly other serverManagedRoles), but the
	// created repository result is only stored on local cache, not published to
	// the remote store. To do that, use r.Publish() eventually.
	Initialize(rootKeyIDs []string, serverManagedRoles ...data.RoleName) error

	// InitializeWithCertificate initializes the repository with root keys and their
	// corresponding certificates
	InitializeWithCertificate(rootKeyIDs []string, rootCerts []data.PublicKey, serverManagedRoles ...data.RoleName) error

	// Publish pushes the local changes in signed material to the remote notary-server
	// Conceptually it performs an operation similar to a `git rebase`
	Publish() error

	// ----- Target Operations -----

	// AddTarget creates new changelist entries to add a target to the given roles
	// in the repository when the changelist gets applied at publish time.
	// If roles are unspecified, the default role is "targets"
	AddTarget(target *Target, roles ...data.RoleName) error

	// RemoveTarget creates new changelist entries to remove a target from the given
	// roles in the repository when the changelist gets applied at publish time.
	// If roles are unspecified, the default role is "target".
	RemoveTarget(targetName string, roles ...data.RoleName) error

	// ----- Changelist operations -----

	// GetChangelist returns the list of the repository's unpublished changes
	GetChangelist() (changelist.Changelist, error)

	// ----- Role operations -----

	// AddDelegation creates changelist entries to add provided delegation public keys and paths.
	// This method composes AddDelegationRoleAndKeys and AddDelegationPaths (each creates one changelist if called).
	AddDelegation(name data.RoleName, delegationKeys []data.PublicKey, paths []string) error

	// AddDelegationRoleAndKeys creates a changelist entry to add provided delegation public keys.
	// This method is the simplest way to create a new delegation, because the delegation must have at least
	// one key upon creation to be valid since we will reject the changelist while validating the threshold.
	AddDelegationRoleAndKeys(name data.RoleName, delegationKeys []data.PublicKey) error

	// AddDelegationPaths creates a changelist entry to add provided paths to an existing delegation.
	// This method cannot create a new delegation itself because the role must meet the key threshold upon
	// creation.
	AddDelegationPaths(name data.RoleName, paths []string) error

	// RemoveDelegationKeysAndPaths creates changelist entries to remove provided delegation key IDs and
	// paths. This method composes RemoveDelegationPaths and RemoveDelegationKeys (each creates one
	// changelist entry if called).
	RemoveDelegationKeysAndPaths(name data.RoleName, keyIDs, paths []string) error

	// RemoveDelegationRole creates a changelist to remove all paths and keys from a role, and delete the
	// role in its entirety.
	RemoveDelegationRole(name data.RoleName) error

	// RemoveDelegationPaths creates a changelist entry to remove provided paths from an existing delegation.
	RemoveDelegationPaths(name data.RoleName, paths []string) error

	// RemoveDelegationKeys creates a changelist entry to remove provided keys from an existing delegation.
	// When this changelist is applied, if the specified keys are the only keys left in the role,
	// the role itself will be deleted in its entirety.
	// It can also delete a key from all delegations under a parent using a name
	// with a wildcard at the end.
	RemoveDelegationKeys(name data.RoleName, keyIDs []string) error

	// ClearDelegationPaths creates a changelist entry to remove all paths from an existing delegation.
	ClearDelegationPaths(name data.RoleName) error

	// ----- Witness and other re-signing operations -----

	// Witness creates change objects to witness (i.e. re-sign) the given
	// roles on the next publish. One change is created per role
	Witness(roles ...data.RoleName) ([]data.RoleName, error)

	// ----- Key Operations -----

	// RotateKey removes all existing keys associated with the role. If no keys are
	// specified in keyList, then this creates and adds one new key or delegates
	// managing the key to the server. If key(s) are specified by keyList, then they are
	// used for signing the role.
	// These changes are staged in a changelist until publish is called.
	RotateKey(role data.RoleName, serverManagesKey bool, keyList []string) error

	// GetCryptoService is the getter for the repository's CryptoService, which is used
	// to sign all updates.
	GetCryptoService() signed.CryptoService
}
