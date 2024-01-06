package registry

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Registry command flag names
const (
	FlagUseDockerCreds      = "use-docker-credentials"
	FlagUseDockerCredsUsage = "Use the registry credentials from the default Docker config file"

	FlagCredsAccount      = "account"
	FlagCredsAccountUsage = "Registry credentials account"

	FlagCredsSecret      = "secret"
	FlagCredsSecretUsage = "Registry credentials secret"

	// Pull Flags

	FlagSaveToDocker      = "save-to-docker"
	FlagSaveToDockerUsage = "Save pulled image to docker"

	// Push Flags

	FlagDocker      = "docker"
	FlagDockerUsage = "Push local docker image"

	FlagTar      = "tar"
	FlagTarUsage = "Push image from a local tar file"

	FlagOCI      = "oci"
	FlagOCIUsage = "Push image from a local OCI Image Layout directory"

	FlagAs      = "as"
	FlagAsUsage = "Tag the selected image with the specified name before pushing"

	// Image Index Flags

	FlagImageIndexName      = "image-index-name"
	FlagImageIndexNameUsage = "Image index name to use"

	FlagImageName      = "image-name"
	FlagImageNameUsage = "Target image name to include in image index"

	FlagAsManifestList      = "as-manifest-list"
	FlagAsManifestListUsage = "Create image index with the manifest list media type instead of the default OCI image index type"

	FlagInsecureRefs      = "insecure-refs"
	FlagInsecureRefsUsage = "Allow the referenced images from insecure registry connections"

	FlagDumpRawManifest      = "dump-raw-manifest"
	FlagDumpRawManifestUsage = "Dump raw manifest for the created image index"

	// Registry Server Flags

	FlagAddress      = "address"
	FlagAddressUsage = "Registry server address"

	FlagPort      = "port"
	FlagPortUsage = "Registry server port"

	FlagDomain      = "domain"
	FlagDomainUsage = "Domain to use for registry server (to get certs)"

	FlagHTTPS      = "https"
	FlagHTTPSUsage = "Use HTTPS"

	FlagCertPath      = "cert-path"
	FlagCertPathUsage = "Cert path for use with HTTPS"

	FlagKeyPath      = "key-path"
	FlagKeyPathUsage = "Key path for use with HTTPS"

	FlagReferrersAPI      = "referrers-api"
	FlagReferrersAPIUsage = "Enables the referrers API endpoint (OCI 1.1+) for the registry server"

	FlagStorePath      = "store-path"
	FlagStorePathUsage = "Directory to store registry blobs"

	FlagMemStore      = "mem-store"
	FlagMemStoreUsage = "Use memory registry blob store"
)

var Flags = map[string]cli.Flag{
	FlagUseDockerCreds: &cli.BoolFlag{
		Name:    FlagUseDockerCreds,
		Value:   false, //defaults to false
		Usage:   FlagUseDockerCredsUsage,
		EnvVars: []string{"DSLIM_REG_DOCKER_CREDS"},
	},
	FlagCredsAccount: &cli.StringFlag{
		Name:    FlagCredsAccount,
		Value:   "",
		Usage:   FlagCredsAccountUsage,
		EnvVars: []string{"DSLIM_REG_ACCOUNT"},
	},
	FlagCredsSecret: &cli.StringFlag{
		Name:    FlagCredsSecret,
		Value:   "",
		Usage:   FlagCredsSecretUsage,
		EnvVars: []string{"DSLIM_REG_SECRET"},
	},
	// Pull Flags:
	FlagSaveToDocker: &cli.BoolFlag{
		Name:    FlagSaveToDocker,
		Value:   true, //defaults to true
		Usage:   FlagSaveToDockerUsage,
		EnvVars: []string{"DSLIM_REG_PULL_SAVE_TO_DOCKER"},
	},
	// Push Flags:
	FlagDocker: &cli.StringFlag{
		Name:    FlagDocker,
		Value:   "",
		Usage:   FlagDockerUsage,
		EnvVars: []string{"DSLIM_REG_PUSH_DOCKER"},
	},
	FlagTar: &cli.StringFlag{
		Name:    FlagTar,
		Value:   "",
		Usage:   FlagTarUsage,
		EnvVars: []string{"DSLIM_REG_PUSH_TAR"},
	},
	FlagOCI: &cli.StringFlag{
		Name:    FlagOCI,
		Value:   "",
		Usage:   FlagOCIUsage,
		EnvVars: []string{"DSLIM_REG_PUSH_OCI"},
	},
	FlagAs: &cli.StringFlag{
		Name:    FlagAs,
		Value:   "",
		Usage:   FlagAsUsage,
		EnvVars: []string{"DSLIM_REG_PUSH_AS"},
	},
	// Image Index Flags:
	FlagImageIndexName: &cli.StringFlag{
		Name:    FlagImageIndexName,
		Value:   "",
		Usage:   FlagImageIndexNameUsage,
		EnvVars: []string{"DSLIM_REG_IIC_INDEX_NAME"},
	},
	FlagImageName: &cli.StringSliceFlag{
		Name:    FlagImageName,
		Value:   cli.NewStringSlice(),
		Usage:   FlagImageNameUsage,
		EnvVars: []string{"DSLIM_REG_IIC_IMAGE_NAME"},
	},
	FlagAsManifestList: &cli.BoolFlag{
		Name:    FlagAsManifestList,
		Value:   false, //defaults to false
		Usage:   FlagAsManifestListUsage,
		EnvVars: []string{"DSLIM_REG_IIC_AS_MLIST"},
	},
	FlagInsecureRefs: &cli.BoolFlag{
		Name:    FlagInsecureRefs,
		Value:   false, //defaults to false
		Usage:   FlagInsecureRefsUsage,
		EnvVars: []string{"DSLIM_REG_IIC_INSECURE_REFS"},
	},
	FlagDumpRawManifest: &cli.BoolFlag{
		Name:    FlagDumpRawManifest,
		Value:   false, //defaults to false
		Usage:   FlagDumpRawManifestUsage,
		EnvVars: []string{"DSLIM_REG_IIC_DUMP_MANIFEST"},
	},
	// Registry Server Flags:
	FlagReferrersAPI: &cli.BoolFlag{
		Name:    FlagReferrersAPI,
		Value:   true, //defaults to true
		Usage:   FlagReferrersAPIUsage,
		EnvVars: []string{"DSLIM_REG_SRV_REFERRERS_API"},
	},
	FlagDomain: &cli.StringFlag{
		Name:    FlagDomain,
		Value:   "",
		Usage:   FlagDomainUsage,
		EnvVars: []string{"DSLIM_REG_SRV_DOMAIN"},
	},
	FlagAddress: &cli.StringFlag{
		Name:    FlagAddress,
		Value:   "0.0.0.0",
		Usage:   FlagAddressUsage,
		EnvVars: []string{"DSLIM_REG_SRV_ADDR"},
	},
	FlagPort: &cli.UintFlag{
		Name:    FlagPort,
		Value:   5000,
		Usage:   FlagPortUsage,
		EnvVars: []string{"DSLIM_REG_SRV_PORT"},
	},
	FlagHTTPS: &cli.BoolFlag{
		Name:    FlagHTTPS,
		Value:   false, //defaults to false
		Usage:   FlagHTTPSUsage,
		EnvVars: []string{"DSLIM_REG_SRV_HTTPS"},
	},
	FlagCertPath: &cli.StringFlag{
		Name:    FlagCertPath,
		Value:   "",
		Usage:   FlagCertPathUsage,
		EnvVars: []string{"DSLIM_REG_SRV_CERT"},
	},
	FlagKeyPath: &cli.StringFlag{
		Name:    FlagKeyPath,
		Value:   "",
		Usage:   FlagKeyPathUsage,
		EnvVars: []string{"DSLIM_REG_SRV_KEY"},
	},
	FlagStorePath: &cli.StringFlag{
		Name:    FlagStorePath,
		Value:   "registry_server_data",
		Usage:   FlagStorePathUsage,
		EnvVars: []string{"DSLIM_REG_SRV_STORE_PATH"},
	},
	FlagMemStore: &cli.BoolFlag{
		Name:    FlagMemStore,
		Value:   false, //defaults to false
		Usage:   FlagMemStoreUsage,
		EnvVars: []string{"DSLIM_REG_SRV_MEM_STORE"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
