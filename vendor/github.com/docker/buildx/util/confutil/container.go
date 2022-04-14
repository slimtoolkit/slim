package confutil

import (
	"bytes"
	"io"
	"os"
	"path"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
)

const (
	// DefaultBuildKitStateDir and DefaultBuildKitConfigDir are the location
	// where buildkitd inside the container stores its state. Some drivers
	// create a Linux container, so this should match the location for Linux,
	// as defined in: https://github.com/moby/buildkit/blob/v0.9.0/util/appdefaults/appdefaults_unix.go#L11-L15
	DefaultBuildKitStateDir  = "/var/lib/buildkit"
	DefaultBuildKitConfigDir = "/etc/buildkit"
)

// LoadConfigFiles creates a temp directory with BuildKit config and
// registry certificates ready to be copied to a container.
func LoadConfigFiles(bkconfig string) (map[string][]byte, error) {
	if _, err := os.Stat(bkconfig); errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "buildkit configuration file not found: %s", bkconfig)
	} else if err != nil {
		return nil, errors.Wrapf(err, "invalid buildkit configuration file: %s", bkconfig)
	}

	// Load config tree
	btoml, err := loadConfigTree(bkconfig)
	if err != nil {
		return nil, err
	}

	m := make(map[string][]byte)

	// Iterate through registry config to copy certs and update
	// BuildKit config with the underlying certs' path in the container.
	//
	// The following BuildKit config:
	//
	// [registry."myregistry.io"]
	//   ca=["/etc/config/myca.pem"]
	//   [[registry."myregistry.io".keypair]]
	//     key="/etc/config/key.pem"
	//     cert="/etc/config/cert.pem"
	//
	// will be translated in the container as:
	//
	// [registry."myregistry.io"]
	//   ca=["/etc/buildkit/certs/myregistry.io/myca.pem"]
	//   [[registry."myregistry.io".keypair]]
	//     key="/etc/buildkit/certs/myregistry.io/key.pem"
	//     cert="/etc/buildkit/certs/myregistry.io/cert.pem"
	if btoml.Has("registry") {
		for regName := range btoml.GetArray("registry").(*toml.Tree).Values() {
			regConf := btoml.GetPath([]string{"registry", regName}).(*toml.Tree)
			if regConf == nil {
				continue
			}
			pfx := path.Join("certs", regName)
			if regConf.Has("ca") {
				regCAs := regConf.GetArray("ca").([]string)
				if len(regCAs) > 0 {
					var cas []string
					for _, ca := range regCAs {
						fp := path.Join(pfx, path.Base(ca))
						cas = append(cas, path.Join(DefaultBuildKitConfigDir, fp))

						dt, err := readFile(ca)
						if err != nil {
							return nil, errors.Wrapf(err, "failed to read CA file: %s", ca)
						}
						m[fp] = dt
					}
					regConf.Set("ca", cas)
				}
			}
			if regConf.Has("keypair") {
				regKeyPairs := regConf.GetArray("keypair").([]*toml.Tree)
				if len(regKeyPairs) == 0 {
					continue
				}
				for _, kp := range regKeyPairs {
					if kp == nil {
						continue
					}
					key := kp.Get("key").(string)
					if len(key) > 0 {
						fp := path.Join(pfx, path.Base(key))
						kp.Set("key", path.Join(DefaultBuildKitConfigDir, fp))
						dt, err := readFile(key)
						if err != nil {
							return nil, errors.Wrapf(err, "failed to read key file: %s", key)
						}
						m[fp] = dt
					}
					cert := kp.Get("cert").(string)
					if len(cert) > 0 {
						fp := path.Join(pfx, path.Base(cert))
						kp.Set("cert", path.Join(DefaultBuildKitConfigDir, fp))
						dt, err := readFile(cert)
						if err != nil {
							return nil, errors.Wrapf(err, "failed to read cert file: %s", cert)
						}
						m[fp] = dt
					}
				}
			}
		}
	}

	b := bytes.NewBuffer(nil)
	_, err = btoml.WriteTo(b)
	if err != nil {
		return nil, err
	}
	m["buildkitd.toml"] = b.Bytes()

	return m, nil
}

func readFile(fp string) ([]byte, error) {
	sf, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer sf.Close()
	return io.ReadAll(io.LimitReader(sf, 1024*1024))
}
