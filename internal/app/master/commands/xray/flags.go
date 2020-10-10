package xray

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Xray command flag names
const (
	FlagChanges          = "changes"
	FlagLayer            = "layer"
	FlagAddImageManifest = "add-image-manifest"
	FlagAddImageConfig   = "add-image-config"
)

// Xray command flag usage info
const (
	FlagChangesUsage          = "Show layer change details for the selected change type (values: none, all, delete, modify, add)"
	FlagLayerUsage            = "Show details for the selected layer (using layer index or ID)"
	FlagAddImageManifestUsage = "Add raw image manifest to the command execution report file"
	FlagAddImageConfigUsage   = "Add raw image config object to the command execution report file"
)

var Flags = map[string]cli.Flag{
	FlagChanges: cli.StringSliceFlag{
		Name:   FlagChanges,
		Value:  &cli.StringSlice{""},
		Usage:  FlagChangesUsage,
		EnvVar: "DSLIM_CHANGES",
	},
	FlagLayer: cli.StringSliceFlag{
		Name:   FlagLayer,
		Value:  &cli.StringSlice{},
		Usage:  FlagLayerUsage,
		EnvVar: "DSLIM_LAYER",
	},
	FlagAddImageManifest: cli.BoolFlag{
		Name:   FlagAddImageManifest,
		Usage:  FlagAddImageManifestUsage,
		EnvVar: "DSLIM_XRAY_IMAGE_MANIFEST",
	},
	FlagAddImageConfig: cli.BoolFlag{
		Name:   FlagAddImageConfig,
		Usage:  FlagAddImageConfigUsage,
		EnvVar: "DSLIM_XRAY_IMAGE_CONFIG",
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
