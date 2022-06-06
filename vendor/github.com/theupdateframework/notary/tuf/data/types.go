package data

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"path"
	"strings"
	"time"

	"github.com/docker/go/canonical/json"
	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary"
)

// GUN is a Globally Unique Name. It is used to identify trust collections.
// An example usage of this is for container image repositories.
// For example: myregistry.io/myuser/myimage
type GUN string

func (g GUN) String() string {
	return string(g)
}

// RoleName type for specifying role
type RoleName string

func (r RoleName) String() string {
	return string(r)
}

// Parent provides the parent path role from the provided child role
func (r RoleName) Parent() RoleName {
	return RoleName(path.Dir(r.String()))
}

// MetadataRoleMapToStringMap generates a map string of bytes from a map RoleName of bytes
func MetadataRoleMapToStringMap(roles map[RoleName][]byte) map[string][]byte {
	metadata := make(map[string][]byte)
	for k, v := range roles {
		metadata[k.String()] = v
	}
	return metadata
}

// NewRoleList generates an array of RoleName objects from a slice of strings
func NewRoleList(roles []string) []RoleName {
	var roleNames []RoleName
	for _, role := range roles {
		roleNames = append(roleNames, RoleName(role))
	}
	return roleNames
}

// RolesListToStringList generates an array of string objects from a slice of roles
func RolesListToStringList(roles []RoleName) []string {
	var roleNames []string
	for _, role := range roles {
		roleNames = append(roleNames, role.String())
	}
	return roleNames
}

// SigAlgorithm for types of signatures
type SigAlgorithm string

func (k SigAlgorithm) String() string {
	return string(k)
}

const defaultHashAlgorithm = "sha256"

// NotaryDefaultExpiries is the construct used to configure the default expiry times of
// the various role files.
var NotaryDefaultExpiries = map[RoleName]time.Duration{
	CanonicalRootRole:      notary.NotaryRootExpiry,
	CanonicalTargetsRole:   notary.NotaryTargetsExpiry,
	CanonicalSnapshotRole:  notary.NotarySnapshotExpiry,
	CanonicalTimestampRole: notary.NotaryTimestampExpiry,
}

// Signature types
const (
	EDDSASignature       SigAlgorithm = "eddsa"
	RSAPSSSignature      SigAlgorithm = "rsapss"
	RSAPKCS1v15Signature SigAlgorithm = "rsapkcs1v15"
	ECDSASignature       SigAlgorithm = "ecdsa"
	PyCryptoSignature    SigAlgorithm = "pycrypto-pkcs#1 pss"
)

// Key types
const (
	ED25519Key   = "ed25519"
	RSAKey       = "rsa"
	RSAx509Key   = "rsa-x509"
	ECDSAKey     = "ecdsa"
	ECDSAx509Key = "ecdsa-x509"
)

// TUFTypes is the set of metadata types
var TUFTypes = map[RoleName]string{
	CanonicalRootRole:      "Root",
	CanonicalTargetsRole:   "Targets",
	CanonicalSnapshotRole:  "Snapshot",
	CanonicalTimestampRole: "Timestamp",
}

// ValidTUFType checks if the given type is valid for the role
func ValidTUFType(typ string, role RoleName) bool {
	if ValidRole(role) {
		// All targets delegation roles must have
		// the valid type is for targets.
		if role == "" {
			// role is unknown and does not map to
			// a type
			return false
		}
		if strings.HasPrefix(role.String(), CanonicalTargetsRole.String()+"/") {
			role = CanonicalTargetsRole
		}
	}
	// most people will just use the defaults so have this optimal check
	// first. Do comparison just in case there is some unknown vulnerability
	// if a key and value in the map differ.
	if v, ok := TUFTypes[role]; ok {
		return typ == v
	}
	return false
}

// Signed is the high level, partially deserialized metadata object
// used to verify signatures before fully unpacking, or to add signatures
// before fully packing
type Signed struct {
	Signed     *json.RawMessage `json:"signed"`
	Signatures []Signature      `json:"signatures"`
}

// SignedCommon contains the fields common to the Signed component of all
// TUF metadata files
type SignedCommon struct {
	Type    string    `json:"_type"`
	Expires time.Time `json:"expires"`
	Version int       `json:"version"`
}

// SignedMeta is used in server validation where we only need signatures
// and common fields
type SignedMeta struct {
	Signed     SignedCommon `json:"signed"`
	Signatures []Signature  `json:"signatures"`
}

// Signature is a signature on a piece of metadata
type Signature struct {
	KeyID     string       `json:"keyid"`
	Method    SigAlgorithm `json:"method"`
	Signature []byte       `json:"sig"`
	IsValid   bool         `json:"-"`
}

// Files is the map of paths to file meta container in targets and delegations
// metadata files
type Files map[string]FileMeta

// Hashes is the map of hash type to digest created for each metadata
// and target file
type Hashes map[string][]byte

// NotaryDefaultHashes contains the default supported hash algorithms.
var NotaryDefaultHashes = []string{notary.SHA256, notary.SHA512}

// FileMeta contains the size and hashes for a metadata or target file. Custom
// data can be optionally added.
type FileMeta struct {
	Length int64            `json:"length"`
	Hashes Hashes           `json:"hashes"`
	Custom *json.RawMessage `json:"custom,omitempty"`
}

// Equals returns true if the other FileMeta object is equivalent to this one
func (f FileMeta) Equals(o FileMeta) bool {
	if o.Length != f.Length || len(o.Hashes) != len(f.Hashes) {
		return false
	}
	if f.Custom == nil && o.Custom != nil || f.Custom != nil && o.Custom == nil {
		return false
	}
	// we don't care if these are valid hashes, just that they are equal
	for key, val := range f.Hashes {
		if !bytes.Equal(val, o.Hashes[key]) {
			return false
		}
	}
	if f.Custom == nil && o.Custom == nil {
		return true
	}
	fBytes, err := f.Custom.MarshalJSON()
	if err != nil {
		return false
	}
	oBytes, err := o.Custom.MarshalJSON()
	if err != nil {
		return false
	}
	return bytes.Equal(fBytes, oBytes)
}

// CheckHashes verifies all the checksums specified by the "hashes" of the payload.
func CheckHashes(payload []byte, name string, hashes Hashes) error {
	cnt := 0

	// k, v indicate the hash algorithm and the corresponding value
	for k, v := range hashes {
		switch k {
		case notary.SHA256:
			checksum := sha256.Sum256(payload)
			if subtle.ConstantTimeCompare(checksum[:], v) == 0 {
				return ErrMismatchedChecksum{alg: notary.SHA256, name: name, expected: hex.EncodeToString(v)}
			}
			cnt++
		case notary.SHA512:
			checksum := sha512.Sum512(payload)
			if subtle.ConstantTimeCompare(checksum[:], v) == 0 {
				return ErrMismatchedChecksum{alg: notary.SHA512, name: name, expected: hex.EncodeToString(v)}
			}
			cnt++
		}
	}

	if cnt == 0 {
		return ErrMissingMeta{Role: name}
	}

	return nil
}

// CompareMultiHashes verifies that the two Hashes passed in can represent the same data.
// This means that both maps must have at least one key defined for which they map, and no conflicts.
// Note that we check the intersection of map keys, which adds support for non-default hash algorithms in notary
func CompareMultiHashes(hashes1, hashes2 Hashes) error {
	// First check if the two hash structures are valid
	if err := CheckValidHashStructures(hashes1); err != nil {
		return err
	}
	if err := CheckValidHashStructures(hashes2); err != nil {
		return err
	}
	// Check if they have at least one matching hash, and no conflicts
	cnt := 0
	for hashAlg, hash1 := range hashes1 {

		hash2, ok := hashes2[hashAlg]
		if !ok {
			continue
		}

		if subtle.ConstantTimeCompare(hash1[:], hash2[:]) == 0 {
			return fmt.Errorf("mismatched %s checksum", hashAlg)
		}
		// If we reached here, we had a match
		cnt++
	}

	if cnt == 0 {
		return fmt.Errorf("at least one matching hash needed")
	}

	return nil
}

// CheckValidHashStructures returns an error, or nil, depending on whether
// the content of the hashes is valid or not.
func CheckValidHashStructures(hashes Hashes) error {
	cnt := 0

	for k, v := range hashes {
		switch k {
		case notary.SHA256:
			if len(v) != sha256.Size {
				return ErrInvalidChecksum{alg: notary.SHA256}
			}
			cnt++
		case notary.SHA512:
			if len(v) != sha512.Size {
				return ErrInvalidChecksum{alg: notary.SHA512}
			}
			cnt++
		}
	}

	if cnt == 0 {
		return fmt.Errorf("at least one supported hash needed")
	}

	return nil
}

// NewFileMeta generates a FileMeta object from the reader, using the
// hash algorithms provided
func NewFileMeta(r io.Reader, hashAlgorithms ...string) (FileMeta, error) {
	if len(hashAlgorithms) == 0 {
		hashAlgorithms = []string{defaultHashAlgorithm}
	}
	hashes := make(map[string]hash.Hash, len(hashAlgorithms))
	for _, hashAlgorithm := range hashAlgorithms {
		var h hash.Hash
		switch hashAlgorithm {
		case notary.SHA256:
			h = sha256.New()
		case notary.SHA512:
			h = sha512.New()
		default:
			return FileMeta{}, fmt.Errorf("Unknown hash algorithm: %s", hashAlgorithm)
		}
		hashes[hashAlgorithm] = h
		r = io.TeeReader(r, h)
	}
	n, err := io.Copy(ioutil.Discard, r)
	if err != nil {
		return FileMeta{}, err
	}
	m := FileMeta{Length: n, Hashes: make(Hashes, len(hashes))}
	for hashAlgorithm, h := range hashes {
		m.Hashes[hashAlgorithm] = h.Sum(nil)
	}
	return m, nil
}

// Delegations holds a tier of targets delegations
type Delegations struct {
	Keys  Keys    `json:"keys"`
	Roles []*Role `json:"roles"`
}

// NewDelegations initializes an empty Delegations object
func NewDelegations() *Delegations {
	return &Delegations{
		Keys:  make(map[string]PublicKey),
		Roles: make([]*Role, 0),
	}
}

// These values are recommended TUF expiry times.
var defaultExpiryTimes = map[RoleName]time.Duration{
	CanonicalRootRole:      notary.Year,
	CanonicalTargetsRole:   90 * notary.Day,
	CanonicalSnapshotRole:  7 * notary.Day,
	CanonicalTimestampRole: notary.Day,
}

// SetDefaultExpiryTimes allows one to change the default expiries.
func SetDefaultExpiryTimes(times map[RoleName]time.Duration) {
	for key, value := range times {
		if _, ok := defaultExpiryTimes[key]; !ok {
			logrus.Errorf("Attempted to set default expiry for an unknown role: %s", key.String())
			continue
		}
		defaultExpiryTimes[key] = value
	}
}

// DefaultExpires gets the default expiry time for the given role
func DefaultExpires(role RoleName) time.Time {
	if d, ok := defaultExpiryTimes[role]; ok {
		return time.Now().Add(d)
	}
	var t time.Time
	return t.UTC().Round(time.Second)
}

type unmarshalledSignature Signature

// UnmarshalJSON does a custom unmarshalling of the signature JSON
func (s *Signature) UnmarshalJSON(data []byte) error {
	uSignature := unmarshalledSignature{}
	err := json.Unmarshal(data, &uSignature)
	if err != nil {
		return err
	}
	uSignature.Method = SigAlgorithm(strings.ToLower(string(uSignature.Method)))
	*s = Signature(uSignature)
	return nil
}
