package certdiscover

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"log"
)

// Cert Files
var certFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
	"/etc/ssl/cert.pem",                                 // Alpine Linux
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
	"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
	"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
	"/etc/pki/tls/cacert.pem",                           // OpenELEC
}

// Cert File Env Vars
var certFileEnvVars = []string{
	"SSL_CERT_FILE",
	"PIP_CERT",
	"NODE_EXTRA_CA_CERTS",
}

// Cert Directories
var certDirectories = []string{
	"/etc/ssl/certs",     // Debian/Ubuntu/OpenSUSE
	"/etc/pki/tls/certs", // Fedora/RHEL/CentOS/Amazon Linux
}

// Cert Dir Env Vars
var certDirEnvVars = []string{
	"SSL_CERT_DIR",
}

//Private Key Directories
var certPKDirectories = []string{
	"/etc/ssl/private",     // Debian/Ubuntu/OpenSUSE
	"/etc/pki/tls/private", // Fedora/RHEL/CentOS/Amazon Linux
}

/*
/etc/ssl/ca/cacert.pem

/etc/ssl/ca/certs
/etc/pki/CA/certs

/etc/ssl/ca/private
/etc/pki/CA/private
*/

func IsCertFile(name string) bool {
	//exact match
	return false
}

func IsCertDirPath(name string) bool {
	//prefix match
	return false
}

func IsCertDir(name string) bool {
	//exact match
	return false
}

// certifi package cert file path matchers
var certifiCertPathMatchers = []string{
	"-packages/certifi/cacert.pem",
	"/node_modules/certifi/cacert.pem",
	"/lib/certifi/vendor/cacert.pem", //also should include "gems/"
}
