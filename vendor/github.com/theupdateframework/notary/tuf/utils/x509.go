package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/tuf/data"
	"golang.org/x/crypto/ed25519"
)

// CanonicalKeyID returns the ID of the public bytes version of a TUF key.
// On regular RSA/ECDSA TUF keys, this is just the key ID.  On X509 RSA/ECDSA
// TUF keys, this is the key ID of the public key part of the key in the leaf cert
func CanonicalKeyID(k data.PublicKey) (string, error) {
	if k == nil {
		return "", errors.New("public key is nil")
	}
	switch k.Algorithm() {
	case data.ECDSAx509Key, data.RSAx509Key:
		return X509PublicKeyID(k)
	default:
		return k.ID(), nil
	}
}

// LoadCertFromPEM returns the first certificate found in a bunch of bytes or error
// if nothing is found. Taken from https://golang.org/src/crypto/x509/cert_pool.go#L85.
func LoadCertFromPEM(pemBytes []byte) (*x509.Certificate, error) {
	for len(pemBytes) > 0 {
		var block *pem.Block
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			return nil, errors.New("no certificates found in PEM data")
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		return cert, nil
	}

	return nil, errors.New("no certificates found in PEM data")
}

// X509PublicKeyID returns a public key ID as a string, given a
// data.PublicKey that contains an X509 Certificate
func X509PublicKeyID(certPubKey data.PublicKey) (string, error) {
	// Note that this only loads the first certificate from the public key
	cert, err := LoadCertFromPEM(certPubKey.Public())
	if err != nil {
		return "", err
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return "", err
	}

	var key data.PublicKey
	switch certPubKey.Algorithm() {
	case data.ECDSAx509Key:
		key = data.NewECDSAPublicKey(pubKeyBytes)
	case data.RSAx509Key:
		key = data.NewRSAPublicKey(pubKeyBytes)
	}

	return key.ID(), nil
}

func parseLegacyPrivateKey(block *pem.Block, passphrase string) (data.PrivateKey, error) {
	var privKeyBytes []byte
	var err error
	if x509.IsEncryptedPEMBlock(block) {
		privKeyBytes, err = x509.DecryptPEMBlock(block, []byte(passphrase))
		if err != nil {
			return nil, errors.New("could not decrypt private key")
		}
	} else {
		privKeyBytes = block.Bytes
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		rsaPrivKey, err := x509.ParsePKCS1PrivateKey(privKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse DER encoded key: %v", err)
		}

		tufRSAPrivateKey, err := RSAToPrivateKey(rsaPrivKey)
		if err != nil {
			return nil, fmt.Errorf("could not convert rsa.PrivateKey to data.PrivateKey: %v", err)
		}

		return tufRSAPrivateKey, nil
	case "EC PRIVATE KEY":
		ecdsaPrivKey, err := x509.ParseECPrivateKey(privKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse DER encoded private key: %v", err)
		}

		tufECDSAPrivateKey, err := ECDSAToPrivateKey(ecdsaPrivKey)
		if err != nil {
			return nil, fmt.Errorf("could not convert ecdsa.PrivateKey to data.PrivateKey: %v", err)
		}

		return tufECDSAPrivateKey, nil
	case "ED25519 PRIVATE KEY":
		// We serialize ED25519 keys by concatenating the private key
		// to the public key and encoding with PEM. See the
		// ED25519ToPrivateKey function.
		tufECDSAPrivateKey, err := ED25519ToPrivateKey(privKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not convert ecdsa.PrivateKey to data.PrivateKey: %v", err)
		}

		return tufECDSAPrivateKey, nil

	default:
		return nil, fmt.Errorf("unsupported key type %q", block.Type)
	}
}

// ParsePEMPrivateKey returns a data.PrivateKey from a PEM encoded private key. It
// supports PKCS#8 as well as RSA/ECDSA (PKCS#1) only in non-FIPS mode and
// attempts to decrypt using the passphrase, if encrypted.
func ParsePEMPrivateKey(pemBytes []byte, passphrase string) (data.PrivateKey, error) {
	return parsePEMPrivateKey(pemBytes, passphrase, notary.FIPSEnabled())
}

func parsePEMPrivateKey(pemBytes []byte, passphrase string, fips bool) (data.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no valid private key found")
	}

	switch block.Type {
	case "RSA PRIVATE KEY", "EC PRIVATE KEY", "ED25519 PRIVATE KEY":
		if fips {
			return nil, fmt.Errorf("%s not supported in FIPS mode", block.Type)
		}
		return parseLegacyPrivateKey(block, passphrase)
	case "ENCRYPTED PRIVATE KEY", "PRIVATE KEY":
		if passphrase == "" {
			return ParsePKCS8ToTufKey(block.Bytes, nil)
		}
		return ParsePKCS8ToTufKey(block.Bytes, []byte(passphrase))
	default:
		return nil, fmt.Errorf("unsupported key type %q", block.Type)
	}
}

// CertToPEM is a utility function returns a PEM encoded x509 Certificate
func CertToPEM(cert *x509.Certificate) []byte {
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})

	return pemCert
}

// CertChainToPEM is a utility function returns a PEM encoded chain of x509 Certificates, in the order they are passed
func CertChainToPEM(certChain []*x509.Certificate) ([]byte, error) {
	var pemBytes bytes.Buffer
	for _, cert := range certChain {
		if err := pem.Encode(&pemBytes, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
			return nil, err
		}
	}
	return pemBytes.Bytes(), nil
}

// LoadCertFromFile loads the first certificate from the file provided. The
// data is expected to be PEM Encoded and contain one of more certificates
// with PEM type "CERTIFICATE"
func LoadCertFromFile(filename string) (*x509.Certificate, error) {
	certs, err := LoadCertBundleFromFile(filename)
	if err != nil {
		return nil, err
	}
	return certs[0], nil
}

// LoadCertBundleFromFile loads certificates from the []byte provided. The
// data is expected to be PEM Encoded and contain one of more certificates
// with PEM type "CERTIFICATE"
func LoadCertBundleFromFile(filename string) ([]*x509.Certificate, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return LoadCertBundleFromPEM(b)
}

// LoadCertBundleFromPEM loads certificates from the []byte provided. The
// data is expected to be PEM Encoded and contain one of more certificates
// with PEM type "CERTIFICATE"
func LoadCertBundleFromPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	certificates := []*x509.Certificate{}
	var block *pem.Block
	block, pemBytes = pem.Decode(pemBytes)
	for ; block != nil; block, pemBytes = pem.Decode(pemBytes) {
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certificates = append(certificates, cert)
		} else {
			return nil, fmt.Errorf("invalid pem block type: %s", block.Type)
		}
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("no valid certificates found")
	}

	return certificates, nil
}

// GetLeafCerts parses a list of x509 Certificates and returns all of them
// that aren't CA
func GetLeafCerts(certs []*x509.Certificate) []*x509.Certificate {
	var leafCerts []*x509.Certificate
	for _, cert := range certs {
		if cert.IsCA {
			continue
		}
		leafCerts = append(leafCerts, cert)
	}
	return leafCerts
}

// GetIntermediateCerts parses a list of x509 Certificates and returns all of the
// ones marked as a CA, to be used as intermediates
func GetIntermediateCerts(certs []*x509.Certificate) []*x509.Certificate {
	var intCerts []*x509.Certificate
	for _, cert := range certs {
		if cert.IsCA {
			intCerts = append(intCerts, cert)
		}
	}
	return intCerts
}

// ParsePEMPublicKey returns a data.PublicKey from a PEM encoded public key or certificate.
func ParsePEMPublicKey(pubKeyBytes []byte) (data.PublicKey, error) {
	pemBlock, _ := pem.Decode(pubKeyBytes)
	if pemBlock == nil {
		return nil, errors.New("no valid public key found")
	}

	switch pemBlock.Type {
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse provided certificate: %v", err)
		}
		err = ValidateCertificate(cert, true)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate: %v", err)
		}
		return CertToKey(cert), nil
	case "PUBLIC KEY":
		keyType, err := keyTypeForPublicKey(pemBlock.Bytes)
		if err != nil {
			return nil, err
		}
		return data.NewPublicKey(keyType, pemBlock.Bytes), nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q, expected CERTIFICATE or PUBLIC KEY", pemBlock.Type)
	}
}

func keyTypeForPublicKey(pubKeyBytes []byte) (string, error) {
	pub, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return "", fmt.Errorf("unable to parse pem encoded public key: %v", err)
	}
	switch pub.(type) {
	case *ecdsa.PublicKey:
		return data.ECDSAKey, nil
	case *rsa.PublicKey:
		return data.RSAKey, nil
	}
	return "", fmt.Errorf("unknown public key format")
}

// ValidateCertificate returns an error if the certificate is not valid for notary
// Currently this is only ensuring the public key has a large enough modulus if RSA,
// using a non SHA1 signature algorithm, and an optional time expiry check
func ValidateCertificate(c *x509.Certificate, checkExpiry bool) error {
	if (c.NotBefore).After(c.NotAfter) {
		return fmt.Errorf("certificate validity window is invalid")
	}
	// Can't have SHA1 sig algorithm
	if c.SignatureAlgorithm == x509.SHA1WithRSA || c.SignatureAlgorithm == x509.DSAWithSHA1 || c.SignatureAlgorithm == x509.ECDSAWithSHA1 {
		return fmt.Errorf("certificate with CN %s uses invalid SHA1 signature algorithm", c.Subject.CommonName)
	}
	// If we have an RSA key, make sure it's long enough
	if c.PublicKeyAlgorithm == x509.RSA {
		rsaKey, ok := c.PublicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("unable to parse RSA public key")
		}
		if rsaKey.N.BitLen() < notary.MinRSABitSize {
			return fmt.Errorf("RSA bit length is too short")
		}
	}
	if checkExpiry {
		now := time.Now()
		tomorrow := now.AddDate(0, 0, 1)
		// Give one day leeway on creation "before" time, check "after" against today
		if (tomorrow).Before(c.NotBefore) || now.After(c.NotAfter) {
			return data.ErrCertExpired{CN: c.Subject.CommonName}
		}
		// If this certificate is expiring within 6 months, put out a warning
		if (c.NotAfter).Before(time.Now().AddDate(0, 6, 0)) {
			logrus.Warnf("certificate with CN %s is near expiry", c.Subject.CommonName)
		}
	}
	return nil
}

// GenerateKey returns a new private key using the provided algorithm or an
// error detailing why the key could not be generated
func GenerateKey(algorithm string) (data.PrivateKey, error) {
	switch algorithm {
	case data.ECDSAKey:
		return GenerateECDSAKey(rand.Reader)
	case data.ED25519Key:
		return GenerateED25519Key(rand.Reader)
	}
	return nil, fmt.Errorf("private key type not supported for key generation: %s", algorithm)
}

// RSAToPrivateKey converts an rsa.Private key to a TUF data.PrivateKey type
func RSAToPrivateKey(rsaPrivKey *rsa.PrivateKey) (data.PrivateKey, error) {
	// Get a DER-encoded representation of the PublicKey
	rsaPubBytes, err := x509.MarshalPKIXPublicKey(&rsaPrivKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}

	// Get a DER-encoded representation of the PrivateKey
	rsaPrivBytes := x509.MarshalPKCS1PrivateKey(rsaPrivKey)

	pubKey := data.NewRSAPublicKey(rsaPubBytes)
	return data.NewRSAPrivateKey(pubKey, rsaPrivBytes)
}

// GenerateECDSAKey generates an ECDSA Private key and returns a TUF PrivateKey
func GenerateECDSAKey(random io.Reader) (data.PrivateKey, error) {
	ecdsaPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), random)
	if err != nil {
		return nil, err
	}

	tufPrivKey, err := ECDSAToPrivateKey(ecdsaPrivKey)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("generated ECDSA key with keyID: %s", tufPrivKey.ID())

	return tufPrivKey, nil
}

// GenerateED25519Key generates an ED25519 private key and returns a TUF
// PrivateKey. The serialization format we use is just the public key bytes
// followed by the private key bytes
func GenerateED25519Key(random io.Reader) (data.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(random)
	if err != nil {
		return nil, err
	}

	var serialized [ed25519.PublicKeySize + ed25519.PrivateKeySize]byte
	copy(serialized[:], pub[:])
	copy(serialized[ed25519.PublicKeySize:], priv[:])

	tufPrivKey, err := ED25519ToPrivateKey(serialized[:])
	if err != nil {
		return nil, err
	}

	logrus.Debugf("generated ED25519 key with keyID: %s", tufPrivKey.ID())

	return tufPrivKey, nil
}

// ECDSAToPrivateKey converts an ecdsa.Private key to a TUF data.PrivateKey type
func ECDSAToPrivateKey(ecdsaPrivKey *ecdsa.PrivateKey) (data.PrivateKey, error) {
	// Get a DER-encoded representation of the PublicKey
	ecdsaPubBytes, err := x509.MarshalPKIXPublicKey(&ecdsaPrivKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}

	// Get a DER-encoded representation of the PrivateKey
	ecdsaPrivKeyBytes, err := x509.MarshalECPrivateKey(ecdsaPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %v", err)
	}

	pubKey := data.NewECDSAPublicKey(ecdsaPubBytes)
	return data.NewECDSAPrivateKey(pubKey, ecdsaPrivKeyBytes)
}

// ED25519ToPrivateKey converts a serialized ED25519 key to a TUF
// data.PrivateKey type
func ED25519ToPrivateKey(privKeyBytes []byte) (data.PrivateKey, error) {
	if len(privKeyBytes) != ed25519.PublicKeySize+ed25519.PrivateKeySize {
		return nil, errors.New("malformed ed25519 private key")
	}

	pubKey := data.NewED25519PublicKey(privKeyBytes[:ed25519.PublicKeySize])
	return data.NewED25519PrivateKey(*pubKey, privKeyBytes)
}

// ExtractPrivateKeyAttributes extracts role and gun values from private key bytes
func ExtractPrivateKeyAttributes(pemBytes []byte) (data.RoleName, data.GUN, error) {
	return extractPrivateKeyAttributes(pemBytes, notary.FIPSEnabled())
}

func extractPrivateKeyAttributes(pemBytes []byte, fips bool) (data.RoleName, data.GUN, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", "", errors.New("PEM block is empty")
	}

	switch block.Type {
	case "RSA PRIVATE KEY", "EC PRIVATE KEY", "ED25519 PRIVATE KEY":
		if fips {
			return "", "", fmt.Errorf("%s not supported in FIPS mode", block.Type)
		}
	case "PRIVATE KEY", "ENCRYPTED PRIVATE KEY":
		// do nothing for PKCS#8 keys
	default:
		return "", "", errors.New("unknown key format")
	}
	return data.RoleName(block.Headers["role"]), data.GUN(block.Headers["gun"]), nil
}

// ConvertPrivateKeyToPKCS8 converts a data.PrivateKey to PKCS#8 Format
func ConvertPrivateKeyToPKCS8(key data.PrivateKey, role data.RoleName, gun data.GUN, passphrase string) ([]byte, error) {
	var (
		err       error
		der       []byte
		blockType = "PRIVATE KEY"
	)

	if passphrase == "" {
		der, err = ConvertTUFKeyToPKCS8(key, nil)
	} else {
		blockType = "ENCRYPTED PRIVATE KEY"
		der, err = ConvertTUFKeyToPKCS8(key, []byte(passphrase))
	}
	if err != nil {
		return nil, fmt.Errorf("unable to convert to PKCS8 key")
	}

	headers := make(map[string]string)
	if role != "" {
		headers["role"] = role.String()
	}

	if gun != "" {
		headers["gun"] = gun.String()
	}

	return pem.EncodeToMemory(&pem.Block{Bytes: der, Type: blockType, Headers: headers}), nil
}

// CertToKey transforms a single input certificate into its corresponding
// PublicKey
func CertToKey(cert *x509.Certificate) data.PublicKey {
	block := pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	pemdata := pem.EncodeToMemory(&block)

	switch cert.PublicKeyAlgorithm {
	case x509.RSA:
		return data.NewRSAx509PublicKey(pemdata)
	case x509.ECDSA:
		return data.NewECDSAx509PublicKey(pemdata)
	default:
		logrus.Debugf("Unknown key type parsed from certificate: %v", cert.PublicKeyAlgorithm)
		return nil
	}
}

// CertsToKeys transforms each of the input certificate chains into its corresponding
// PublicKey
func CertsToKeys(leafCerts map[string]*x509.Certificate, intCerts map[string][]*x509.Certificate) map[string]data.PublicKey {
	keys := make(map[string]data.PublicKey)
	for id, leafCert := range leafCerts {
		if key, err := CertBundleToKey(leafCert, intCerts[id]); err == nil {
			keys[key.ID()] = key
		}
	}
	return keys
}

// CertBundleToKey creates a TUF key from a leaf certs and a list of
// intermediates
func CertBundleToKey(leafCert *x509.Certificate, intCerts []*x509.Certificate) (data.PublicKey, error) {
	certBundle := []*x509.Certificate{leafCert}
	certBundle = append(certBundle, intCerts...)
	certChainPEM, err := CertChainToPEM(certBundle)
	if err != nil {
		return nil, err
	}
	var newKey data.PublicKey
	// Use the leaf cert's public key algorithm for typing
	switch leafCert.PublicKeyAlgorithm {
	case x509.RSA:
		newKey = data.NewRSAx509PublicKey(certChainPEM)
	case x509.ECDSA:
		newKey = data.NewECDSAx509PublicKey(certChainPEM)
	default:
		logrus.Debugf("Unknown key type parsed from certificate: %v", leafCert.PublicKeyAlgorithm)
		return nil, x509.ErrUnsupportedAlgorithm
	}
	return newKey, nil
}

// NewCertificate returns an X509 Certificate following a template, given a Common Name and validity interval.
func NewCertificate(commonName string, startTime, endTime time.Time) (*x509.Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new certificate: %v", err)
	}

	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore: startTime,
		NotAfter:  endTime,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}, nil
}
