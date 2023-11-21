package registry

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Registry command flag names
const (
	FlagUseDockerCreds  = "use-docker-credentials"
	FlagCredsAccount    = "account"
	FlagCredsSecret     = "secret"
	FlagSaveToDocker    = "save-to-docker"
	FlagImageIndexName  = "image-index-name"
	FlagImageName       = "image-name"
	FlagAsManifestList  = "as-manifest-list"
	FlagInsecureRefs    = "insecure-refs"
	FlagDumpRawManifest = "dump-raw-manifest"
)

// Registry command flag usage info
const (
	FlagUseDockerCredsUsage  = "Use the registry credentials from the default Docker config file"
	FlagCredsAccountUsage    = "Registry credentials account"
	FlagCredsSecretUsage     = "Registry credentials secret"
	FlagSaveToDockerUsage    = "Save pulled image to docker"
	FlagImageIndexNameUsage  = "Image index name to use"
	FlagImageNameUsage       = "Target image name to include in image index"
	FlagAsManifestListUsage  = "Create image index with the manifest list media type instead of the default OCI image index type"
	FlagInsecureRefsUsage    = "Allow the referenced images from insecure registry connections"
	FlagDumpRawManifestUsage = "Dump raw manifest for the created image index"
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
	FlagSaveToDocker: &cli.BoolFlag{
		Name:    FlagSaveToDocker,
		Value:   true, //defaults to true
		Usage:   FlagSaveToDockerUsage,
		EnvVars: []string{"DSLIM_REG_PULL_SAVE_TO_DOCKER"},
	},
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
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
