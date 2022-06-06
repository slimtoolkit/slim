package trustmanager

import (
	"github.com/theupdateframework/notary/tuf/data"
)

// Storage implements the bare bones primitives (no hierarchy)
type Storage interface {
	// Add writes a file to the specified location, returning an error if this
	// is not possible (reasons may include permissions errors). The path is cleaned
	// before being made absolute against the store's base dir.
	Set(fileName string, data []byte) error

	// Remove deletes a file from the store relative to the store's base directory.
	// The path is cleaned before being made absolute to ensure no path traversal
	// outside the base directory is possible.
	Remove(fileName string) error

	// Get returns the file content found at fileName relative to the base directory
	// of the file store. The path is cleaned before being made absolute to ensure
	// path traversal outside the store is not possible. If the file is not found
	// an error to that effect is returned.
	Get(fileName string) ([]byte, error)

	// ListFiles returns a list of paths relative to the base directory of the
	// filestore. Any of these paths must be retrievable via the
	// Storage.Get method.
	ListFiles() []string

	// Location returns a human readable name indicating where the implementer
	// is storing keys
	Location() string
}

// KeyInfo stores the role and gun for a corresponding private key ID
// It is assumed that each private key ID is unique
type KeyInfo struct {
	Gun  data.GUN
	Role data.RoleName
}

// KeyStore is a generic interface for private key storage
type KeyStore interface {
	// AddKey adds a key to the KeyStore, and if the key already exists,
	// succeeds.  Otherwise, returns an error if it cannot add.
	AddKey(keyInfo KeyInfo, privKey data.PrivateKey) error
	// Should fail with ErrKeyNotFound if the keystore is operating normally
	// and knows that it does not store the requested key.
	GetKey(keyID string) (data.PrivateKey, data.RoleName, error)
	GetKeyInfo(keyID string) (KeyInfo, error)
	ListKeys() map[string]KeyInfo
	RemoveKey(keyID string) error
	Name() string
}
