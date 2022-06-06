package changelist

import (
	"github.com/theupdateframework/notary/tuf/data"
)

// Scopes for TUFChanges are simply the TUF roles.
// Unfortunately because of targets delegations, we can only
// cover the base roles.
const (
	ScopeRoot    = "root"
	ScopeTargets = "targets"
)

// Types for TUFChanges are namespaced by the Role they
// are relevant for. The Root and Targets roles are the
// only ones for which user action can cause a change, as
// all changes in Snapshot and Timestamp are programmatically
// generated base on Root and Targets changes.
const (
	TypeBaseRole          = "role"
	TypeTargetsTarget     = "target"
	TypeTargetsDelegation = "delegation"
	TypeWitness           = "witness"
)

// TUFChange represents a change to a TUF repo
type TUFChange struct {
	// Abbreviated because Go doesn't permit a field and method of the same name
	Actn       string        `json:"action"`
	Role       data.RoleName `json:"role"`
	ChangeType string        `json:"type"`
	ChangePath string        `json:"path"`
	Data       []byte        `json:"data"`
}

// TUFRootData represents a modification of the keys associated
// with a role that appears in the root.json
type TUFRootData struct {
	Keys     data.KeyList  `json:"keys"`
	RoleName data.RoleName `json:"role"`
}

// NewTUFChange initializes a TUFChange object
func NewTUFChange(action string, role data.RoleName, changeType, changePath string, content []byte) *TUFChange {
	return &TUFChange{
		Actn:       action,
		Role:       role,
		ChangeType: changeType,
		ChangePath: changePath,
		Data:       content,
	}
}

// Action return c.Actn
func (c TUFChange) Action() string {
	return c.Actn
}

// Scope returns c.Role
func (c TUFChange) Scope() data.RoleName {
	return c.Role
}

// Type returns c.ChangeType
func (c TUFChange) Type() string {
	return c.ChangeType
}

// Path return c.ChangePath
func (c TUFChange) Path() string {
	return c.ChangePath
}

// Content returns c.Data
func (c TUFChange) Content() []byte {
	return c.Data
}

// TUFDelegation represents a modification to a target delegation
// this includes creating a delegations. This format is used to avoid
// unexpected race conditions between humans modifying the same delegation
type TUFDelegation struct {
	NewName       data.RoleName `json:"new_name,omitempty"`
	NewThreshold  int           `json:"threshold,omitempty"`
	AddKeys       data.KeyList  `json:"add_keys,omitempty"`
	RemoveKeys    []string      `json:"remove_keys,omitempty"`
	AddPaths      []string      `json:"add_paths,omitempty"`
	RemovePaths   []string      `json:"remove_paths,omitempty"`
	ClearAllPaths bool          `json:"clear_paths,omitempty"`
}

// ToNewRole creates a fresh role object from the TUFDelegation data
func (td TUFDelegation) ToNewRole(scope data.RoleName) (*data.Role, error) {
	name := scope
	if td.NewName != "" {
		name = td.NewName
	}
	return data.NewRole(name, td.NewThreshold, td.AddKeys.IDs(), td.AddPaths)
}
