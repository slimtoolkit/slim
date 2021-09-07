package xray

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Xray command flag names
const (
	FlagChanges                = "changes"
	FlagChangesOutput          = "changes-output"
	FlagLayer                  = "layer"
	FlagAddImageManifest       = "add-image-manifest"
	FlagAddImageConfig         = "add-image-config"
	FlagLayerChangesMax        = "layer-changes-max"
	FlagAllChangesMax          = "all-changes-max"
	FlagAddChangesMax          = "add-changes-max"
	FlagModifyChangesMax       = "modify-changes-max"
	FlagDeleteChangesMax       = "delete-changes-max"
	FlagChangePath             = "change-path"
	FlagChangeData             = "change-data"
	FlagChangeDataHash         = "change-data-hash"
	FlagReuseSavedImage        = "reuse-saved-image"
	FlagTopChangesMax          = "top-changes-max"
	FlagHashData               = "hash-data"
	FlagDetectUTF8             = "detect-utf8"
	FlagDetectDuplicates       = "detect-duplicates"
	FlagShowDuplicates         = "show-duplicates"
	FlagShowSpecialPerms       = "show-special-perms"
	FlagChangeMatchLayersOnly  = "change-match-layers-only"
	FlagExportAllDataArtifacts = "export-all-data-artifacts"
	FlagDetectAllCertFiles     = "detect-all-certs"
	FlagDetectAllCertPKFiles   = "detect-all-cert-pks"
)

// Xray command flag usage info
const (
	FlagChangesUsage                = "Show layer change details for the selected change type (values: none, all, delete, modify, add)"
	FlagChangesOutputUsage          = "Where to show the changes (values: all, report, console)"
	FlagLayerUsage                  = "Show details for the selected layer (using layer index or ID)"
	FlagAddImageManifestUsage       = "Add raw image manifest to the command execution report file"
	FlagAddImageConfigUsage         = "Add raw image config object to the command execution report file"
	FlagLayerChangesMaxUsage        = "Maximum number of changes to show for each layer"
	FlagAllChangesMaxUsage          = "Maximum number of changes to show for all layers"
	FlagAddChangesMaxUsage          = "Maximum number of 'add' changes to show for all layers"
	FlagModifyChangesMaxUsage       = "Maximum number of 'modify' changes to show for all layers"
	FlagDeleteChangesMaxUsage       = "Maximum number of 'delete' changes to show for all layers"
	FlagChangePathUsage             = "Include changes for the files that match the path pattern (Glob/Match in Go and **)"
	FlagChangeDataUsage             = "Include changes for the files that match the data pattern (regex)"
	FlagReuseSavedImageUsage        = "Reuse saved container image"
	FlagTopChangesMaxUsage          = "Maximum number of top changes to track"
	FlagChangeDataHashUsage         = "Include changes for the files that match the provided data hashes (sha1)"
	FlagHashDataUsage               = "Generate file data hashes"
	FlagDetectUTF8Usage             = "Detect utf8 files and optionally extract the discovered utf8 file content"
	FlagDetectDuplicatesUsage       = "Detect duplicate files based on their hashes"
	FlagShowDuplicatesUsage         = "Show discovered duplicate file paths"
	FlagShowSpecialPermsUsage       = "Show files with special permissions (setuid,setgid,sticky)"
	FlagChangeMatchLayersOnlyUsage  = "Show only layers with change matches"
	FlagExportAllDataArtifactsUsage = "Archive path to export all data artifacts enabling the related flags if not set (if set to . then path defaults to ./data-artifacts.tar)"
	FlagDetectAllCertFilesUsage     = "Detect all certifcate files"
	FlagDetectAllCertPKFilesUsage   = "Detect all certifcate private key files"
)

var Flags = map[string]cli.Flag{
	FlagChanges: cli.StringSliceFlag{
		Name:   FlagChanges,
		Value:  &cli.StringSlice{""},
		Usage:  FlagChangesUsage,
		EnvVar: "DSLIM_CHANGES",
	},
	FlagChangesOutput: cli.StringSliceFlag{
		Name:   FlagChangesOutput,
		Value:  &cli.StringSlice{"all"},
		Usage:  FlagChangesOutputUsage,
		EnvVar: "DSLIM_CHANGES_OUTPUT",
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
	FlagLayerChangesMax: cli.IntFlag{
		Name:   FlagLayerChangesMax,
		Value:  -1,
		Usage:  FlagLayerChangesMaxUsage,
		EnvVar: "DSLIM_XRAY_LAYER_CHANGES_MAX",
	},
	FlagAllChangesMax: cli.IntFlag{
		Name:   FlagAllChangesMax,
		Value:  -1,
		Usage:  FlagAllChangesMaxUsage,
		EnvVar: "DSLIM_XRAY_ALL_CHANGES_MAX",
	},
	FlagAddChangesMax: cli.IntFlag{
		Name:   FlagAddChangesMax,
		Value:  -1,
		Usage:  FlagAddChangesMaxUsage,
		EnvVar: "DSLIM_XRAY_ADD_CHANGES_MAX",
	},
	FlagModifyChangesMax: cli.IntFlag{
		Name:   FlagModifyChangesMax,
		Value:  -1,
		Usage:  FlagModifyChangesMaxUsage,
		EnvVar: "DSLIM_XRAY_MODIFY_CHANGES_MAX",
	},
	FlagDeleteChangesMax: cli.IntFlag{
		Name:   FlagDeleteChangesMax,
		Value:  -1,
		Usage:  FlagDeleteChangesMaxUsage,
		EnvVar: "DSLIM_XRAY_DELETE_CHANGES_MAX",
	},
	FlagChangePath: cli.StringSliceFlag{
		Name:   FlagChangePath,
		Value:  &cli.StringSlice{},
		Usage:  FlagChangePathUsage,
		EnvVar: "DSLIM_XRAY_CHANGE_PATH",
	},
	FlagChangeData: cli.StringSliceFlag{
		Name:   FlagChangeData,
		Value:  &cli.StringSlice{},
		Usage:  FlagChangeDataUsage,
		EnvVar: "DSLIM_XRAY_CHANGE_DATA",
	},
	FlagReuseSavedImage: cli.BoolTFlag{
		Name:   FlagReuseSavedImage,
		Usage:  FlagReuseSavedImageUsage,
		EnvVar: "DSLIM_XRAY_REUSE_SAVED",
	},
	FlagTopChangesMax: cli.IntFlag{
		Name:   FlagTopChangesMax,
		Value:  20,
		Usage:  FlagTopChangesMaxUsage,
		EnvVar: "DSLIM_XRAY_TOP_CHANGES_MAX",
	},
	FlagHashData: cli.BoolFlag{
		Name:   FlagHashData,
		Usage:  FlagHashDataUsage,
		EnvVar: "DSLIM_XRAY_HASH_DATA",
	},
	FlagDetectUTF8: cli.StringFlag{
		Name:   FlagDetectUTF8,
		Usage:  FlagDetectUTF8Usage,
		EnvVar: "DSLIM_XRAY_DETECT_UTF8",
	},
	FlagDetectDuplicates: cli.BoolFlag{
		Name:   FlagDetectDuplicates,
		Usage:  FlagDetectDuplicatesUsage,
		EnvVar: "DSLIM_XRAY_DETECT_DUP",
	},
	FlagShowDuplicates: cli.BoolTFlag{
		Name:   FlagShowDuplicates,
		Usage:  FlagShowDuplicatesUsage,
		EnvVar: "DSLIM_XRAY_SHOW_DUP",
	},
	FlagShowSpecialPerms: cli.BoolTFlag{
		Name:   FlagShowSpecialPerms,
		Usage:  FlagShowSpecialPermsUsage,
		EnvVar: "DSLIM_XRAY_SHOW_SPECIAL",
	},
	FlagChangeDataHash: cli.StringSliceFlag{
		Name:   FlagChangeDataHash,
		Value:  &cli.StringSlice{},
		Usage:  FlagChangeDataHashUsage,
		EnvVar: "DSLIM_XRAY_CHANGE_DATA_HASH",
	},
	FlagChangeMatchLayersOnly: cli.BoolFlag{
		Name:   FlagChangeMatchLayersOnly,
		Usage:  FlagChangeMatchLayersOnlyUsage,
		EnvVar: "DSLIM_XRAY_CHANGE_MATCH_LAYERS_ONLY",
	},
	FlagExportAllDataArtifacts: cli.StringFlag{
		Name:   FlagExportAllDataArtifacts,
		Usage:  FlagExportAllDataArtifactsUsage,
		EnvVar: "DSLIM_XRAY_EXPORT_ALL_DARTIFACTS",
	},
	FlagDetectAllCertFiles: cli.BoolFlag{
		Name:   FlagDetectAllCertFiles,
		Usage:  FlagDetectAllCertFilesUsage,
		EnvVar: "DSLIM_XRAY_DETECT_ALL_CERTS",
	},
	FlagDetectAllCertPKFiles: cli.BoolFlag{
		Name:   FlagDetectAllCertPKFiles,
		Usage:  FlagDetectAllCertPKFilesUsage,
		EnvVar: "DSLIM_XRAY_DETECT_ALL_CERT_PKS",
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
