package certdiscover

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Cert Files (standard system cert bundles) and Java Keystore
var certFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",                // Debian / Ubuntu / Gentoo / etc.
	"/etc/ssl/cert.pem",                                 // Alpine / Arch / RHEL 9
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS / RHEL 7
	"/etc/ssl/certs/ca-bundle.crt",                      // RHEL / Fedora
	"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora / RHEL 6
	"/etc/pki/tls/cert.pem",                             // CentOS 7, 8 / RHEL 7, 8 / Fedora <= 33 (can be a symlnk to /etc/pki/tls/certs/ca-bundle.crt)
	"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
	"/etc/pki/tls/cacert.pem",                           // OpenELEC
	"/etc/ssl/certs/java/cacerts",                       // Java Keystore Alpine / Ubuntu
	"/etc/pki/java/cacerts",                             // Java Keystore RHEL
}

var certFilesSet map[string]struct{}

// Cert Directories
var certDirectories = []string{
	"/etc/ssl/certs",       // Debian/Ubuntu/OpenSUSE
	"/etc/pki/tls/certs",   // Fedora/RHEL/CentOS/Amazon Linux
	"/usr/lib/ssl/certs",   // OpenSSL, usually linked to the OS-specific location (e.g., /etc/ssl/certs)
	"/usr/local/ssl/certs", // OpenSSL
}

var certDirsSet map[string]struct{}

// Cert File Env Vars (TODO: use these)
var certFileEnvVars = []string{
	"SSL_CERT_FILE",
	"PIP_CERT",
	"NODE_EXTRA_CA_CERTS",
	"CURL_CA_BUNDLE",
}

var certFileEnvVarsSet map[string]struct{}

// Cert Dir Env Vars
var certDirEnvVars = []string{
	"SSL_CERT_DIR", // ":" separated list of directories
}

var certDirEnvVarsSet map[string]struct{}

// Cert Private Key Directories
var certPKDirectories = []string{
	"/etc/ssl/private",       // Debian/Ubuntu/OpenSUSE
	"/etc/pki/tls/private",   // Fedora/RHEL/CentOS/Amazon Linux
	"/usr/lib/ssl/private",   // OpenSSL, usually linked to the OS-specific location (e.g., /etc/ssl/private)
	"/usr/local/ssl/private", // OpenSSL
}

var certPKDirsSet map[string]struct{}

// CA Cert Files (standard CA cert bundles)
var caCertFiles = []string{
	"/etc/ssl/ca/certs/ca.crt",
	"/etc/pki/CA/certs/ca.crt",
	"/etc/ssl/ca/cacert.pem",
	//"/etc/letsencrypt/live/<domain>/fullchain.pem" (also: cert.pem, chain.pem)
}

var caCertFilesSet map[string]struct{}

// CA Cert Directories
var caCertDirectories = []string{
	"/etc/ssl/ca/certs",
	"/etc/pki/CA/certs",
	"/etc/letsencrypt/live",
}

var caCertDirsSet map[string]struct{}

// CA Private Key Files
var caCertPKFiles = []string{
	"/etc/ssl/ca/private/ca.key",
	"/etc/pki/CA/private/ca.key",
	"/etc/ssl/ca/private/cakey.pem",
	//"/etc/letsencrypt/live/<domain>/privkey.pem"
}

var caCertPKFilesSet map[string]struct{}

// CA Private Key Directories
var caCertPKDirectories = []string{
	"/etc/ssl/ca/private",
	"/etc/pki/CA/private",
	"/etc/letsencrypt/live",
}

var caCertPKDirsSet map[string]struct{}

func IsCertFile(name string) bool {
	_, found := certFilesSet[name]
	return found
}

func IsCertDirPath(name string) bool {
	for _, dir := range certDirectories {
		dir := fmt.Sprintf("%s/", dir)
		if strings.HasPrefix(name, dir) {
			return true
		}
	}

	return false
}

func IsCertDir(name string) bool {
	_, found := certDirsSet[name]
	return found
}

func IsCertPKDir(name string) bool {
	_, found := certPKDirsSet[name]
	return found
}

func IsCertPKDirPath(name string) bool {
	for _, dir := range certPKDirectories {
		dir := fmt.Sprintf("%s/", dir)
		if strings.HasPrefix(name, dir) {
			return true
		}
	}

	return false
}

func IsCACertFile(name string) bool {
	_, found := caCertFilesSet[name]
	return found
}

func IsCACertDir(name string) bool {
	_, found := caCertDirsSet[name]
	return found
}

func IsCACertDirPath(name string) bool {
	for _, dir := range caCertDirectories {
		dir := fmt.Sprintf("%s/", dir)
		if strings.HasPrefix(name, dir) {
			return true
		}
	}

	return false
}

func IsCACertPKFile(name string) bool {
	_, found := caCertPKFilesSet[name]
	return found
}

func IsCACertPKDir(name string) bool {
	_, found := caCertPKDirsSet[name]
	return found
}

func IsCACertPKDirPath(name string) bool {
	for _, dir := range caCertPKDirectories {
		dir := fmt.Sprintf("%s/", dir)
		if strings.HasPrefix(name, dir) {
			return true
		}
	}

	return false
}

// Certifi package cert file (bundle) path matchers (+ Java Keystore)
var certifiCertPathMatchers = []string{
	"-packages/certifi/cacert.pem",
	"/node_modules/certifi/cacert.pem",
	"/lib/certifi/vendor/cacert.pem", //also should include "gems/"
	"/jre/lib/security/cacerts",      //Java Keystore
}

func IsAppCertFile(name string) bool {
	for _, pat := range certifiCertPathMatchers {
		if strings.HasSuffix(name, pat) {
			return true
		}
	}

	return false
}

const (
	beginCert = "-----BEGIN CERTIFICATE-----"
	endCert   = "-----END CERTIFICATE-----"
)

func IsCertData(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}

	if bytes.Contains(data, []byte(beginCert)) &&
		bytes.Contains(data, []byte(endCert)) {
		return true
	}

	return false
}

const (
	beginRSAPK = "-----BEGIN RSA PRIVATE KEY-----"
	endRSAPK   = "-----END RSA PRIVATE KEY-----"

	beginEncPK = "-----BEGIN ENCRYPTED PRIVATE KEY-----"
	endEncPK   = "-----END ENCRYPTED PRIVATE KEY-----"

	beginPK = "-----BEGIN PRIVATE KEY-----"
	endPK   = "-----END PRIVATE KEY-----"

	pkPart = " PRIVATE KEY-----"
)

func IsPrivateKeyData(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}

	//Basic PEM detection (TODO: detect other formats like DER)
	if bytes.Contains(data, []byte(pkPart)) {
		if (bytes.Contains(data, []byte(beginPK)) && bytes.Contains(data, []byte(endPK))) ||
			(bytes.Contains(data, []byte(beginRSAPK)) && bytes.Contains(data, []byte(endRSAPK))) ||
			(bytes.Contains(data, []byte(beginEncPK)) && bytes.Contains(data, []byte(endEncPK))) {
			return true
		}
	}

	return false
}

func IsCertHashName(name string) bool {
	if len(name) == 10 &&
		name[8] == '.' &&
		(name[9] >= '0' && name[9] <= '9') {
		return true
	}

	return false
}

func init() {
	certFilesSet = initItemSet(certFiles)
	certDirsSet = initItemSet(certDirectories)
	certPKDirsSet = initItemSet(certPKDirectories)

	certFileEnvVarsSet = initItemSet(certFileEnvVars)
	certDirEnvVarsSet = initItemSet(certDirEnvVars)

	caCertFilesSet = initItemSet(caCertFiles)
	caCertDirsSet = initItemSet(caCertDirectories)
	caCertPKFilesSet = initItemSet(caCertPKFiles)
	caCertPKDirsSet = initItemSet(caCertPKDirectories)
}

func initItemSet(items []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, item := range items {
		set[item] = struct{}{}
	}

	return set
}

//todo: handle symlinks
