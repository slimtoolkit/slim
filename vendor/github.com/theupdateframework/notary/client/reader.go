package client

import (
	"fmt"

	canonicaljson "github.com/docker/go/canonical/json"
	store "github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/tuf"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/utils"
)

// Target represents a simplified version of the data TUF operates on, so external
// applications don't have to depend on TUF data types.
type Target struct {
	Name   string                    // the name of the target
	Hashes data.Hashes               // the hash of the target
	Length int64                     // the size in bytes of the target
	Custom *canonicaljson.RawMessage // the custom data provided to describe the file at TARGETPATH
}

// TargetWithRole represents a Target that exists in a particular role - this is
// produced by ListTargets and GetTargetByName
type TargetWithRole struct {
	Target
	Role data.RoleName
}

// TargetSignedStruct is a struct that contains a Target, the role it was found in, and the list of signatures for that role
type TargetSignedStruct struct {
	Role       data.DelegationRole
	Target     Target
	Signatures []data.Signature
}

//ErrNoSuchTarget is returned when no valid trust data is found.
type ErrNoSuchTarget string

func (f ErrNoSuchTarget) Error() string {
	return fmt.Sprintf("No valid trust data for %s", string(f))
}

// RoleWithSignatures is a Role with its associated signatures
type RoleWithSignatures struct {
	Signatures []data.Signature
	data.Role
}

// NewReadOnly is the base method that returns a new notary repository for reading.
// It expects an initialized cache. In case of a nil remote store, a default
// offline store is used.
func NewReadOnly(repo *tuf.Repo) ReadOnly {
	return &reader{tufRepo: repo}
}

type reader struct {
	tufRepo *tuf.Repo
}

// ListTargets lists all targets for the current repository. The list of
// roles should be passed in order from highest to lowest priority.
//
// IMPORTANT: if you pass a set of roles such as [ "targets/a", "targets/x"
// "targets/a/b" ], even though "targets/a/b" is part of the "targets/a" subtree
// its entries will be strictly shadowed by those in other parts of the "targets/a"
// subtree and also the "targets/x" subtree, as we will defer parsing it until
// we explicitly reach it in our iteration of the provided list of roles.
func (r *reader) ListTargets(roles ...data.RoleName) ([]*TargetWithRole, error) {
	if len(roles) == 0 {
		roles = []data.RoleName{data.CanonicalTargetsRole}
	}
	targets := make(map[string]*TargetWithRole)
	for _, role := range roles {
		// Define an array of roles to skip for this walk (see IMPORTANT comment above)
		skipRoles := utils.RoleNameSliceRemove(roles, role)

		// Define a visitor function to populate the targets map in priority order
		listVisitorFunc := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
			// We found targets so we should try to add them to our targets map
			for targetName, targetMeta := range tgt.Signed.Targets {
				// Follow the priority by not overriding previously set targets
				// and check that this path is valid with this role
				if _, ok := targets[targetName]; ok || !validRole.CheckPaths(targetName) {
					continue
				}
				targets[targetName] = &TargetWithRole{
					Target: Target{
						Name:   targetName,
						Hashes: targetMeta.Hashes,
						Length: targetMeta.Length,
						Custom: targetMeta.Custom,
					},
					Role: validRole.Name,
				}
			}
			return nil
		}

		r.tufRepo.WalkTargets("", role, listVisitorFunc, skipRoles...)
	}

	var targetList []*TargetWithRole
	for _, v := range targets {
		targetList = append(targetList, v)
	}

	return targetList, nil
}

// GetTargetByName returns a target by the given name. If no roles are passed
// it uses the targets role and does a search of the entire delegation
// graph, finding the first entry in a breadth first search of the delegations.
// If roles are passed, they should be passed in descending priority and
// the target entry found in the subtree of the highest priority role
// will be returned.
// See the IMPORTANT section on ListTargets above. Those roles also apply here.
func (r *reader) GetTargetByName(name string, roles ...data.RoleName) (*TargetWithRole, error) {
	if len(roles) == 0 {
		roles = append(roles, data.CanonicalTargetsRole)
	}
	var resultMeta data.FileMeta
	var resultRoleName data.RoleName
	var foundTarget bool
	for _, role := range roles {
		// Define an array of roles to skip for this walk (see IMPORTANT comment above)
		skipRoles := utils.RoleNameSliceRemove(roles, role)

		// Define a visitor function to find the specified target
		getTargetVisitorFunc := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
			if tgt == nil {
				return nil
			}
			// We found the target and validated path compatibility in our walk,
			// so we should stop our walk and set the resultMeta and resultRoleName variables
			if resultMeta, foundTarget = tgt.Signed.Targets[name]; foundTarget {
				resultRoleName = validRole.Name
				return tuf.StopWalk{}
			}
			return nil
		}
		// Check that we didn't error, and that we assigned to our target
		if err := r.tufRepo.WalkTargets(name, role, getTargetVisitorFunc, skipRoles...); err == nil && foundTarget {
			return &TargetWithRole{Target: Target{Name: name, Hashes: resultMeta.Hashes, Length: resultMeta.Length, Custom: resultMeta.Custom}, Role: resultRoleName}, nil
		}
	}
	return nil, ErrNoSuchTarget(name)

}

// GetAllTargetMetadataByName searches the entire delegation role tree to find the specified target by name for all
// roles, and returns a list of TargetSignedStructs for each time it finds the specified target.
// If given an empty string for a target name, it will return back all targets signed into the repository in every role
func (r *reader) GetAllTargetMetadataByName(name string) ([]TargetSignedStruct, error) {
	var targetInfoList []TargetSignedStruct

	// Define a visitor function to find the specified target
	getAllTargetInfoByNameVisitorFunc := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
		if tgt == nil {
			return nil
		}
		// We found a target and validated path compatibility in our walk,
		// so add it to our list if we have a match
		// if we have an empty name, add all targets, else check if we have it
		var targetMetaToAdd data.Files
		if name == "" {
			targetMetaToAdd = tgt.Signed.Targets
		} else {
			if meta, ok := tgt.Signed.Targets[name]; ok {
				targetMetaToAdd = data.Files{name: meta}
			}
		}

		for targetName, resultMeta := range targetMetaToAdd {
			targetInfo := TargetSignedStruct{
				Role:       validRole,
				Target:     Target{Name: targetName, Hashes: resultMeta.Hashes, Length: resultMeta.Length, Custom: resultMeta.Custom},
				Signatures: tgt.Signatures,
			}
			targetInfoList = append(targetInfoList, targetInfo)
		}
		// continue walking to all child roles
		return nil
	}

	// Check that we didn't error, and that we found the target at least once
	if err := r.tufRepo.WalkTargets(name, "", getAllTargetInfoByNameVisitorFunc); err != nil {
		return nil, err
	}
	if len(targetInfoList) == 0 {
		return nil, ErrNoSuchTarget(name)
	}
	return targetInfoList, nil
}

// ListRoles returns a list of RoleWithSignatures objects for this repo
// This represents the latest metadata for each role in this repo
func (r *reader) ListRoles() ([]RoleWithSignatures, error) {
	// Get all role info from our updated keysDB, can be empty
	roles := r.tufRepo.GetAllLoadedRoles()

	var roleWithSigs []RoleWithSignatures

	// Populate RoleWithSignatures with Role from keysDB and signatures from TUF metadata
	for _, role := range roles {
		roleWithSig := RoleWithSignatures{Role: *role, Signatures: nil}
		switch role.Name {
		case data.CanonicalRootRole:
			roleWithSig.Signatures = r.tufRepo.Root.Signatures
		case data.CanonicalTargetsRole:
			roleWithSig.Signatures = r.tufRepo.Targets[data.CanonicalTargetsRole].Signatures
		case data.CanonicalSnapshotRole:
			roleWithSig.Signatures = r.tufRepo.Snapshot.Signatures
		case data.CanonicalTimestampRole:
			roleWithSig.Signatures = r.tufRepo.Timestamp.Signatures
		default:
			if !data.IsDelegation(role.Name) {
				continue
			}
			if _, ok := r.tufRepo.Targets[role.Name]; ok {
				// We'll only find a signature if we've published any targets with this delegation
				roleWithSig.Signatures = r.tufRepo.Targets[role.Name].Signatures
			}
		}
		roleWithSigs = append(roleWithSigs, roleWithSig)
	}
	return roleWithSigs, nil
}

// GetDelegationRoles returns the keys and roles of the repository's delegations
// Also converts key IDs to canonical key IDs to keep consistent with signing prompts
func (r *reader) GetDelegationRoles() ([]data.Role, error) {
	// All top level delegations (ex: targets/level1) are stored exclusively in targets.json
	_, ok := r.tufRepo.Targets[data.CanonicalTargetsRole]
	if !ok {
		return nil, store.ErrMetaNotFound{Resource: data.CanonicalTargetsRole.String()}
	}

	// make a copy for traversing nested delegations
	allDelegations := []data.Role{}

	// Define a visitor function to populate the delegations list and translate their key IDs to canonical IDs
	delegationCanonicalListVisitor := func(tgt *data.SignedTargets, validRole data.DelegationRole) interface{} {
		// For the return list, update with a copy that includes canonicalKeyIDs
		// These aren't validated by the validRole
		canonicalDelegations, err := translateDelegationsToCanonicalIDs(tgt.Signed.Delegations)
		if err != nil {
			return err
		}
		allDelegations = append(allDelegations, canonicalDelegations...)
		return nil
	}
	err := r.tufRepo.WalkTargets("", "", delegationCanonicalListVisitor)
	if err != nil {
		return nil, err
	}
	return allDelegations, nil
}
