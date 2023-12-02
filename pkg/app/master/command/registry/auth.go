package registry

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	//log "github.com/sirupsen/logrus"
)

func ConfigureAuth(cparams *CommonCommandParams, remoteOpts []remote.Option) ([]remote.Option, error) {
	if cparams.UseDockerCreds {
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		return remoteOpts, nil
	}

	if cparams.CredsAccount != "" && cparams.CredsSecret != "" {
		remoteOpts = append(remoteOpts, remote.WithAuth(&authn.Basic{
			Username: cparams.CredsAccount,
			Password: cparams.CredsSecret,
		}))

		return remoteOpts, nil
	}

	//it's authn.Anonymous by default, but good to be explicit
	return append(remoteOpts, remote.WithAuth(authn.Anonymous)), nil
}
