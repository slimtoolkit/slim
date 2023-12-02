package merge

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Merge command flag names
const (
	FlagImage                = "image"
	FlagUseLastImageMetadata = "use-last-image-metadata"
	FlagTag                  = "tag"
)

// Merge command flag usage info
const (
	FlagImageUsage                = "Image to merge (flag instance position determines the merge order)"
	FlagUseLastImageMetadataUsage = "Use only the last image metadata for the merged image"
	FlagTagUsage                  = "Custom tags for the output image"
)

var Flags = map[string]cli.Flag{
	FlagImage: &cli.StringSliceFlag{
		Name:    FlagImage,
		Value:   cli.NewStringSlice(),
		Usage:   FlagImageUsage,
		EnvVars: []string{"DSLIM_MERGE_IMAGE"},
	},
	FlagUseLastImageMetadata: &cli.BoolFlag{
		Name:    FlagUseLastImageMetadata,
		Value:   false, //defaults to false
		Usage:   FlagUseLastImageMetadataUsage,
		EnvVars: []string{"DSLIM_MERGE_USE_LAST_IMAGE_META"},
	},
	FlagTag: &cli.StringSliceFlag{
		Name:    FlagTag,
		Value:   cli.NewStringSlice(),
		Usage:   FlagTagUsage,
		EnvVars: []string{"DSLIM_TARGET_TAG"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
