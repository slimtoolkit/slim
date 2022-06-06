package cryptoservice

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/utils"
)

// GenerateCertificate generates an X509 Certificate from a template, given a GUN and validity interval
func GenerateCertificate(rootKey data.PrivateKey, gun data.GUN, startTime, endTime time.Time) (*x509.Certificate, error) {
	signer := rootKey.CryptoSigner()
	if signer == nil {
		return nil, fmt.Errorf("key type not supported for Certificate generation: %s", rootKey.Algorithm())
	}

	return generateCertificate(signer, gun, startTime, endTime)
}

func generateCertificate(signer crypto.Signer, gun data.GUN, startTime, endTime time.Time) (*x509.Certificate, error) {
	template, err := utils.NewCertificate(gun.String(), startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create the certificate template for: %s (%v)", gun, err)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, signer.Public(), signer)
	if err != nil {
		return nil, fmt.Errorf("failed to create the certificate for: %s (%v)", gun, err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the certificate for key: %s (%v)", gun, err)
	}

	return cert, nil
}
