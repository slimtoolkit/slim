package client

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/client/changelist"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/utils"
)

// AddDelegation creates changelist entries to add provided delegation public keys and paths.
// This method composes AddDelegationRoleAndKeys and AddDelegationPaths (each creates one changelist if called).
func (r *repository) AddDelegation(name data.RoleName, delegationKeys []data.PublicKey, paths []string) error {
	if len(delegationKeys) > 0 {
		err := r.AddDelegationRoleAndKeys(name, delegationKeys)
		if err != nil {
			return err
		}
	}
	if len(paths) > 0 {
		err := r.AddDelegationPaths(name, paths)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddDelegationRoleAndKeys creates a changelist entry to add provided delegation public keys.
// This method is the simplest way to create a new delegation, because the delegation must have at least
// one key upon creation to be valid since we will reject the changelist while validating the threshold.
func (r *repository) AddDelegationRoleAndKeys(name data.RoleName, delegationKeys []data.PublicKey) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	logrus.Debugf(`Adding delegation "%s" with threshold %d, and %d keys\n`,
		name, notary.MinThreshold, len(delegationKeys))

	// Defaulting to threshold of 1, since we don't allow for larger thresholds at the moment.
	tdJSON, err := json.Marshal(&changelist.TUFDelegation{
		NewThreshold: notary.MinThreshold,
		AddKeys:      data.KeyList(delegationKeys),
	})
	if err != nil {
		return err
	}

	template := newCreateDelegationChange(name, tdJSON)
	return addChange(r.changelist, template, name)
}

// AddDelegationPaths creates a changelist entry to add provided paths to an existing delegation.
// This method cannot create a new delegation itself because the role must meet the key threshold upon creation.
func (r *repository) AddDelegationPaths(name data.RoleName, paths []string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	logrus.Debugf(`Adding %s paths to delegation %s\n`, paths, name)

	tdJSON, err := json.Marshal(&changelist.TUFDelegation{
		AddPaths: paths,
	})
	if err != nil {
		return err
	}

	template := newCreateDelegationChange(name, tdJSON)
	return addChange(r.changelist, template, name)
}

// RemoveDelegationKeysAndPaths creates changelist entries to remove provided delegation key IDs and paths.
// This method composes RemoveDelegationPaths and RemoveDelegationKeys (each creates one changelist entry if called).
func (r *repository) RemoveDelegationKeysAndPaths(name data.RoleName, keyIDs, paths []string) error {
	if len(paths) > 0 {
		err := r.RemoveDelegationPaths(name, paths)
		if err != nil {
			return err
		}
	}
	if len(keyIDs) > 0 {
		err := r.RemoveDelegationKeys(name, keyIDs)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveDelegationRole creates a changelist to remove all paths and keys from a role, and delete the role in its entirety.
func (r *repository) RemoveDelegationRole(name data.RoleName) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	logrus.Debugf(`Removing delegation "%s"\n`, name)

	template := newDeleteDelegationChange(name, nil)
	return addChange(r.changelist, template, name)
}

// RemoveDelegationPaths creates a changelist entry to remove provided paths from an existing delegation.
func (r *repository) RemoveDelegationPaths(name data.RoleName, paths []string) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	logrus.Debugf(`Removing %s paths from delegation "%s"\n`, paths, name)

	tdJSON, err := json.Marshal(&changelist.TUFDelegation{
		RemovePaths: paths,
	})
	if err != nil {
		return err
	}

	template := newUpdateDelegationChange(name, tdJSON)
	return addChange(r.changelist, template, name)
}

// RemoveDelegationKeys creates a changelist entry to remove provided keys from an existing delegation.
// When this changelist is applied, if the specified keys are the only keys left in the role,
// the role itself will be deleted in its entirety.
// It can also delete a key from all delegations under a parent using a name
// with a wildcard at the end.
func (r *repository) RemoveDelegationKeys(name data.RoleName, keyIDs []string) error {

	if !data.IsDelegation(name) && !data.IsWildDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	logrus.Debugf(`Removing %s keys from delegation "%s"\n`, keyIDs, name)

	tdJSON, err := json.Marshal(&changelist.TUFDelegation{
		RemoveKeys: keyIDs,
	})
	if err != nil {
		return err
	}

	template := newUpdateDelegationChange(name, tdJSON)
	return addChange(r.changelist, template, name)
}

// ClearDelegationPaths creates a changelist entry to remove all paths from an existing delegation.
func (r *repository) ClearDelegationPaths(name data.RoleName) error {

	if !data.IsDelegation(name) {
		return data.ErrInvalidRole{Role: name, Reason: "invalid delegation role name"}
	}

	logrus.Debugf(`Removing all paths from delegation "%s"\n`, name)

	tdJSON, err := json.Marshal(&changelist.TUFDelegation{
		ClearAllPaths: true,
	})
	if err != nil {
		return err
	}

	template := newUpdateDelegationChange(name, tdJSON)
	return addChange(r.changelist, template, name)
}

func newUpdateDelegationChange(name data.RoleName, content []byte) *changelist.TUFChange {
	return changelist.NewTUFChange(
		changelist.ActionUpdate,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path for delegations
		content,
	)
}

func newCreateDelegationChange(name data.RoleName, content []byte) *changelist.TUFChange {
	return changelist.NewTUFChange(
		changelist.ActionCreate,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path for delegations
		content,
	)
}

func newDeleteDelegationChange(name data.RoleName, content []byte) *changelist.TUFChange {
	return changelist.NewTUFChange(
		changelist.ActionDelete,
		name,
		changelist.TypeTargetsDelegation,
		"", // no path for delegations
		content,
	)
}

func translateDelegationsToCanonicalIDs(delegationInfo data.Delegations) ([]data.Role, error) {
	canonicalDelegations := make([]data.Role, len(delegationInfo.Roles))
	// Do a copy by value to ensure local delegation metadata is untouched
	for idx, origRole := range delegationInfo.Roles {
		canonicalDelegations[idx] = *origRole
	}
	delegationKeys := delegationInfo.Keys
	for i, delegation := range canonicalDelegations {
		canonicalKeyIDs := []string{}
		for _, keyID := range delegation.KeyIDs {
			pubKey, ok := delegationKeys[keyID]
			if !ok {
				return []data.Role{}, fmt.Errorf("Could not translate canonical key IDs for %s", delegation.Name)
			}
			canonicalKeyID, err := utils.CanonicalKeyID(pubKey)
			if err != nil {
				return []data.Role{}, fmt.Errorf("Could not translate canonical key IDs for %s: %v", delegation.Name, err)
			}
			canonicalKeyIDs = append(canonicalKeyIDs, canonicalKeyID)
		}
		canonicalDelegations[i].KeyIDs = canonicalKeyIDs
	}
	return canonicalDelegations, nil
}
