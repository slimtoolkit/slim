package xray

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
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

	FlagDetectIdentities        = "detect-identities"
	FlagDetectIdentitiesParam   = "detect-identities-param"
	FlagDetectIdentitiesDumpRaw = "detect-identities-dump-raw"

	FlagDetectScheduledTasks        = "detect-scheduled-tasks"
	FlagDetectScheduledTasksParam   = "detect-scheduled-tasks-param"
	FlagDetectScheduledTasksDumpRaw = "detect-scheduled-tasks-dump-raw"

	FlagDetectServices        = "detect-services"
	FlagDetectServicesParam   = "detect-services-param"
	FlagDetectServicesDumpRaw = "detect-services-dump-raw"

	FlagDetectSystemHooks        = "detect-system-hooks"
	FlagDetectSystemHooksParam   = "detect-system-hooks-param"
	FlagDetectSystemHooksDumpRaw = "detect-system-hooks-dump-raw"
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
	FlagExportAllDataArtifactsUsage = "TAR archive file path to export all text data artifacts (if value is set to `.` then the archive file path defaults to `./data-artifacts.tar`)"
	FlagDetectAllCertFilesUsage     = "Detect all certifcate files"
	FlagDetectAllCertPKFilesUsage   = "Detect all certifcate private key files"

	FlagDetectIdentitiesUsage        = "Detect system identities (users, groups) and their properties"
	FlagDetectIdentitiesParamUsage   = "Input parameters for system identities detection"
	FlagDetectIdentitiesDumpRawUsage = "Raw data dump options for system identities detection (values: no, console, directory or a tar archive file path where setting value to `.` defaults tar file path to `./raw-identities.tar`"

	FlagDetectScheduledTasksUsage        = "Detect scheduled tasks and their properties"
	FlagDetectScheduledTasksParamUsage   = "Input parameters for scheduled tasks detection"
	FlagDetectScheduledTasksDumpRawUsage = "Raw data dump options for scheduled tasks detection (values: `no`, `console`, directory or a tar archive file path where setting value to `.` defaults tar file path to `./raw-scheduled-tasks.tar`"

	FlagDetectServicesUsage        = "Detect services and their properties"
	FlagDetectServicesParamUsage   = "Input parameters for services detection"
	FlagDetectServicesDumpRawUsage = "Raw data dump options for services detection (values: no, console, directory or a tar archive file path where setting value to `.` defaults tar file path to `./raw-services.tar`"

	FlagDetectSystemHooksUsage        = "Detect system hooks and their properties"
	FlagDetectSystemHooksParamUsage   = "Input parameters for system hooks detection"
	FlagDetectSystemHooksDumpRawUsage = "Raw data dump options for system hooks detection (values: no, console, directory or a tar archive file path where setting value to `.` defaults tar file path to `./raw-system-hooks.tar`"
)

var Flags = map[string]cli.Flag{
	FlagChanges: &cli.StringSliceFlag{
		Name:    FlagChanges,
		Value:   cli.NewStringSlice(""),
		Usage:   FlagChangesUsage,
		EnvVars: []string{"DSLIM_CHANGES"},
	},
	FlagChangesOutput: &cli.StringSliceFlag{
		Name:    FlagChangesOutput,
		Value:   cli.NewStringSlice("all"),
		Usage:   FlagChangesOutputUsage,
		EnvVars: []string{"DSLIM_CHANGES_OUTPUT"},
	},
	FlagLayer: &cli.StringSliceFlag{
		Name:    FlagLayer,
		Value:   cli.NewStringSlice(),
		Usage:   FlagLayerUsage,
		EnvVars: []string{"DSLIM_LAYER"},
	},
	FlagAddImageManifest: &cli.BoolFlag{
		Name:    FlagAddImageManifest,
		Usage:   FlagAddImageManifestUsage,
		EnvVars: []string{"DSLIM_XRAY_IMAGE_MANIFEST"},
	},
	FlagAddImageConfig: &cli.BoolFlag{
		Name:    FlagAddImageConfig,
		Usage:   FlagAddImageConfigUsage,
		EnvVars: []string{"DSLIM_XRAY_IMAGE_CONFIG"},
	},
	FlagLayerChangesMax: &cli.IntFlag{
		Name:    FlagLayerChangesMax,
		Value:   -1,
		Usage:   FlagLayerChangesMaxUsage,
		EnvVars: []string{"DSLIM_XRAY_LAYER_CHANGES_MAX"},
	},
	FlagAllChangesMax: &cli.IntFlag{
		Name:    FlagAllChangesMax,
		Value:   -1,
		Usage:   FlagAllChangesMaxUsage,
		EnvVars: []string{"DSLIM_XRAY_ALL_CHANGES_MAX"},
	},
	FlagAddChangesMax: &cli.IntFlag{
		Name:    FlagAddChangesMax,
		Value:   -1,
		Usage:   FlagAddChangesMaxUsage,
		EnvVars: []string{"DSLIM_XRAY_ADD_CHANGES_MAX"},
	},
	FlagModifyChangesMax: &cli.IntFlag{
		Name:    FlagModifyChangesMax,
		Value:   -1,
		Usage:   FlagModifyChangesMaxUsage,
		EnvVars: []string{"DSLIM_XRAY_MODIFY_CHANGES_MAX"},
	},
	FlagDeleteChangesMax: &cli.IntFlag{
		Name:    FlagDeleteChangesMax,
		Value:   -1,
		Usage:   FlagDeleteChangesMaxUsage,
		EnvVars: []string{"DSLIM_XRAY_DELETE_CHANGES_MAX"},
	},
	FlagChangePath: &cli.StringSliceFlag{
		Name:    FlagChangePath,
		Value:   cli.NewStringSlice(),
		Usage:   FlagChangePathUsage,
		EnvVars: []string{"DSLIM_XRAY_CHANGE_PATH"},
	},
	FlagChangeData: &cli.StringSliceFlag{
		Name:    FlagChangeData,
		Value:   cli.NewStringSlice(),
		Usage:   FlagChangeDataUsage,
		EnvVars: []string{"DSLIM_XRAY_CHANGE_DATA"},
	},
	FlagReuseSavedImage: &cli.BoolFlag{
		Name:    FlagReuseSavedImage,
		Value:   true, //enabled by default
		Usage:   FlagReuseSavedImageUsage,
		EnvVars: []string{"DSLIM_XRAY_REUSE_SAVED"},
	},
	FlagTopChangesMax: &cli.IntFlag{
		Name:    FlagTopChangesMax,
		Value:   20,
		Usage:   FlagTopChangesMaxUsage,
		EnvVars: []string{"DSLIM_XRAY_TOP_CHANGES_MAX"},
	},
	FlagHashData: &cli.BoolFlag{
		Name:    FlagHashData,
		Usage:   FlagHashDataUsage,
		EnvVars: []string{"DSLIM_XRAY_HASH_DATA"},
	},
	FlagDetectUTF8: &cli.StringFlag{
		Name:    FlagDetectUTF8,
		Usage:   FlagDetectUTF8Usage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_UTF8"},
	},
	FlagDetectDuplicates: &cli.BoolFlag{
		Name:    FlagDetectDuplicates,
		Value:   true, //enabled by default
		Usage:   FlagDetectDuplicatesUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_DUP"},
	},
	FlagShowDuplicates: &cli.BoolFlag{
		Name:    FlagShowDuplicates,
		Usage:   FlagShowDuplicatesUsage,
		EnvVars: []string{"DSLIM_XRAY_SHOW_DUP"},
	},
	FlagShowSpecialPerms: &cli.BoolFlag{
		Name:    FlagShowSpecialPerms,
		Value:   true, //enabled by default
		Usage:   FlagShowSpecialPermsUsage,
		EnvVars: []string{"DSLIM_XRAY_SHOW_SPECIAL"},
	},
	FlagChangeDataHash: &cli.StringSliceFlag{
		Name:    FlagChangeDataHash,
		Value:   cli.NewStringSlice(),
		Usage:   FlagChangeDataHashUsage,
		EnvVars: []string{"DSLIM_XRAY_CHANGE_DATA_HASH"},
	},
	FlagChangeMatchLayersOnly: &cli.BoolFlag{
		Name:    FlagChangeMatchLayersOnly,
		Usage:   FlagChangeMatchLayersOnlyUsage,
		EnvVars: []string{"DSLIM_XRAY_CHANGE_MATCH_LAYERS_ONLY"},
	},
	FlagExportAllDataArtifacts: &cli.StringFlag{
		Name:    FlagExportAllDataArtifacts,
		Usage:   FlagExportAllDataArtifactsUsage,
		EnvVars: []string{"DSLIM_XRAY_EXPORT_ALL_DARTIFACTS"},
	},
	FlagDetectAllCertFiles: &cli.BoolFlag{
		Name:    FlagDetectAllCertFiles,
		Usage:   FlagDetectAllCertFilesUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_ALL_CERTS"},
	},
	FlagDetectAllCertPKFiles: &cli.BoolFlag{
		Name:    FlagDetectAllCertPKFiles,
		Usage:   FlagDetectAllCertPKFilesUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_ALL_CERT_PKS"},
	},

	FlagDetectIdentities: &cli.BoolFlag{
		Name:    FlagDetectIdentities,
		Value:   true, //enabled by default
		Usage:   FlagDetectIdentitiesUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_IDENTITIES"},
	},
	FlagDetectIdentitiesParam: &cli.StringSliceFlag{
		Name:    FlagDetectIdentitiesParam,
		Value:   cli.NewStringSlice(),
		Usage:   FlagDetectIdentitiesParamUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_IDENTITIES_PARAM"},
	},
	FlagDetectIdentitiesDumpRaw: &cli.StringFlag{
		Name:    FlagDetectIdentitiesDumpRaw,
		Usage:   FlagDetectIdentitiesDumpRawUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_IDENTITIES_DUMP_RAW"},
	},

	FlagDetectScheduledTasks: &cli.BoolFlag{
		Name:    FlagDetectScheduledTasks,
		Value:   true, //enabled by default
		Usage:   FlagDetectScheduledTasksUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SCHEDULED_TASKS"},
	},
	FlagDetectScheduledTasksParam: &cli.StringSliceFlag{
		Name:    FlagDetectScheduledTasksParam,
		Value:   cli.NewStringSlice(),
		Usage:   FlagDetectScheduledTasksParamUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SCHEDULED_TASKS_PARAM"},
	},
	FlagDetectScheduledTasksDumpRaw: &cli.StringFlag{
		Name:    FlagDetectScheduledTasksDumpRaw,
		Usage:   FlagDetectScheduledTasksDumpRawUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SCHEDULED_TASKS_DUMP_RAW"},
	},

	FlagDetectServices: &cli.BoolFlag{
		Name:    FlagDetectServices,
		Value:   true, //enabled by default
		Usage:   FlagDetectServicesUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SERVICES"},
	},
	FlagDetectServicesParam: &cli.StringSliceFlag{
		Name:    FlagDetectServicesParam,
		Value:   cli.NewStringSlice(),
		Usage:   FlagDetectServicesParamUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SERVICES_PARAM"},
	},
	FlagDetectServicesDumpRaw: &cli.StringFlag{
		Name:    FlagDetectServicesDumpRaw,
		Usage:   FlagDetectServicesDumpRawUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SERVICES_DUMP_RAW"},
	},

	FlagDetectSystemHooks: &cli.BoolFlag{
		Name:    FlagDetectSystemHooks,
		Value:   true, //enabled by default
		Usage:   FlagDetectSystemHooksUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SYSTEM_HOOKS"},
	},
	FlagDetectSystemHooksParam: &cli.StringSliceFlag{
		Name:    FlagDetectSystemHooksParam,
		Value:   cli.NewStringSlice(),
		Usage:   FlagDetectSystemHooksParamUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SYSTEM_HOOKS_PARAM"},
	},
	FlagDetectSystemHooksDumpRaw: &cli.StringFlag{
		Name:    FlagDetectSystemHooksDumpRaw,
		Usage:   FlagDetectSystemHooksDumpRawUsage,
		EnvVars: []string{"DSLIM_XRAY_DETECT_SYSTEM_HOOKS_DUMP_RAW"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
