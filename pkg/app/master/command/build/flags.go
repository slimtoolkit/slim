package build

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Build command flag names
const (
	FlagImageBuildEngine = "image-build-engine"
	FlagImageBuildArch   = "image-build-arch"

	FlagDeleteFatImage = "delete-generated-fat-image"

	FlagShowBuildLogs = "show-blogs"

	FlagPathPerms        = "path-perms"
	FlagPathPermsFile    = "path-perms-file"
	FlagPreservePath     = "preserve-path"
	FlagPreservePathFile = "preserve-path-file"
	FlagIncludePath      = "include-path"
	FlagIncludePathFile  = "include-path-file"
	FlagIncludeBin       = "include-bin"
	FlagIncludeBinFile   = "include-bin-file"
	FlagIncludeExe       = "include-exe"
	FlagIncludeExeFile   = "include-exe-file"
	FlagIncludeShell     = "include-shell"

	FlagIncludeDirBins      = "include-dir-bins"
	FlagIncludeDirBinsUsage = "Keep binaries in the target directory (executables or shared objects) and their dependencies, which could be in other locations"

	FlagIncludeWorkdir      = "include-workdir"
	FlagIncludeWorkdirUsage = "Keep files in working directory"

	//TBD
	FlagWorkdirExclude      = "workdir-exclude"
	FlagWorkdirExcludeUsage = "Exclude filter for artifacts when working directory is included"

	FlagIncludeAppImageAddCopyAll = "include-app-image-addcopy-all" //TBD
	FlagIncludeAppImageRun        = "include-app-image-run"         //TBD

	FlagIncludeAppImageAll      = "include-app-image-all"
	FlagIncludeAppImageAllUsage = "Keep everything in the app part of the container image"

	FlagAppImageStartInst      = "app-image-start-instruction"
	FlagAppImageStartInstUsage = "Instruction (prefix) that indicates where the app starts in the container image"

	FlagAppImageStartLayerCount = "app-image-start-layer-count" //TBD

	FlagAppImageStartInstGroup      = "app-image-start-instruction-group"
	FlagAppImageStartInstGroupUsage = "Instruction group (reverse) index that indicates where the app starts in the container image"

	FlagAppImageStartDetect = "app-image-start-detect" //TBD

	FlagAppImageDockerfile      = "app-image-dockerfile" //TODO: make it work with FlagBuildFromDockerfile too
	FlagAppImageDockerfileUsage = "Path to app image Dockerfile (used to determine where the application part of the image starts)"

	FlagIncludePathsCreportFile      = "include-paths-creport-file"
	FlagIncludePathsCreportFileUsage = "Keep files from the referenced creport"

	FlagIncludeOSLibsNet      = "include-oslibs-net"
	FlagIncludeOSLibsNetUsage = "Keep the common networking OS libraries"

	FlagIncludeSSHClient           = "include-ssh-client"
	FlagIncludeSSHClientUsage      = "Keep the common SSH client components and configs"
	FlagIncludeSSHClientAll        = "include-ssh-client-all"
	FlagIncludeSSHClientAllUsage   = "Keep all SSH client components and configs"
	FlagIncludeSSHClientBasic      = "include-ssh-client-basic"
	FlagIncludeSSHClientBasicUsage = "Keep the basic SSH client components and configs"

	FlagIncludeZoneInfo = "include-zoneinfo"

	FlagIncludeCertAll     = "include-cert-all"
	FlagIncludeCertBundles = "include-cert-bundles-only"
	FlagIncludeCertDirs    = "include-cert-dirs"
	FlagIncludeCertPKAll   = "include-cert-pk-all"
	FlagIncludeCertPKDirs  = "include-cert-pk-dirs"

	FlagIncludeNew = "include-new"

	//FlagIncludeLicenses  = "include-licenses"

	FlagKeepTmpArtifacts = "keep-tmp-artifacts"

	FlagIncludeAppNuxtDir            = "include-app-nuxt-dir"
	FlagIncludeAppNuxtBuildDir       = "include-app-nuxt-build-dir"
	FlagIncludeAppNuxtDistDir        = "include-app-nuxt-dist-dir"
	FlagIncludeAppNuxtStaticDir      = "include-app-nuxt-static-dir"
	FlagIncludeAppNuxtNodeModulesDir = "include-app-nuxt-nodemodules-dir"

	FlagIncludeAppNextDir            = "include-app-next-dir"
	FlagIncludeAppNextBuildDir       = "include-app-next-build-dir"
	FlagIncludeAppNextDistDir        = "include-app-next-dist-dir"
	FlagIncludeAppNextStaticDir      = "include-app-next-static-dir"
	FlagIncludeAppNextNodeModulesDir = "include-app-next-nodemodules-dir"

	FlagIncludeNodePackage = "include-node-package"

	FlagKeepPerms = "keep-perms"

	//"EXCLUDE" FLAGS:

	FlagExcludePattern      = "exclude-pattern"
	FlagExcludePatternUsage = "Exclude path pattern (Glob/Match in Go and **) from image"

	FlagExcludeVarLockFiles      = "exclude-varlock-files"
	FlagExcludeVarLockFilesUsage = "Exclude the files in the var and run lock directory"
	//NOTES:
	// also "exclude-varlock-new-files" <- related to "include-new"

	FlagExcludeMounts      = "exclude-mounts"
	FlagExcludeMountsUsage = "Exclude mounted volumes from image"

	//Flags to edit (modify, add and remove) image metadata

	FlagNewEntrypoint = "new-entrypoint"
	FlagNewCmd        = "new-cmd"
	FlagNewLabel      = "new-label"
	FlagNewVolume     = "new-volume"
	FlagNewExpose     = "new-expose"
	FlagNewWorkdir    = "new-workdir"
	FlagNewEnv        = "new-env"
	FlagRemoveVolume  = "remove-volume"
	FlagRemoveExpose  = "remove-expose"
	FlagRemoveEnv     = "remove-env"
	FlagRemoveLabel   = "remove-label"

	FlagTag = "tag"

	FlagImageOverrides = "image-overrides"

	//Flags to build fat images from Dockerfile
	FlagTagFat              = "tag-fat"
	FlagBuildFromDockerfile = "dockerfile"
	FlagDockerfileContext   = "dockerfile-context"
	FlagCBOAddHost          = "cbo-add-host"
	FlagCBOBuildArg         = "cbo-build-arg"
	FlagCBOLabel            = "cbo-label"
	FlagCBOTarget           = "cbo-target"
	FlagCBONetwork          = "cbo-network"
	FlagCBOCacheFrom        = "cbo-cache-from"

	//Experimenal flags
	FlagObfuscateMetadata = "obfuscate-metadata"
)

// Build command flag usage info
const (
	FlagImageBuildEngineUsage = "Select image build engine: internal | docker | none"
	FlagImageBuildArchUsage   = "Select output image build architecture"

	FlagDeleteFatImageUsage = "Delete generated fat image requires --dockerfile flag"

	FlagShowBuildLogsUsage = "Show image build logs"

	FlagPathPermsUsage        = "Set path permissions in optimized image"
	FlagPathPermsFileUsage    = "File with path permissions to set"
	FlagPreservePathUsage     = "Keep path from orignal image in its initial state (changes to the selected container image files when it runs will be discarded)"
	FlagPreservePathFileUsage = "File with paths to keep from original image in their original state (changes to the selected container image files when it runs will be discarded)"
	FlagIncludePathUsage      = "Keep path from original image"
	FlagIncludePathFileUsage  = "File with paths to keep from original image"
	FlagIncludeBinUsage       = "Keep binary from original image (executable or shared object using its absolute path)"
	FlagIncludeExeUsage       = "Keep executable from original image (by executable name)"
	FlagIncludeShellUsage     = "Keep basic shell functionality"

	FlagIncludeZoneInfoUsage = "Keep the OS/libc zoneinfo data"

	FlagIncludeCertAllUsage     = "Keep all discovered cert files"
	FlagIncludeCertBundlesUsage = "Keep only cert bundles"
	FlagIncludeCertDirsUsage    = "Keep known cert directories and all files in them"
	FlagIncludeCertPKAllUsage   = "Keep all discovered cert private keys"
	FlagIncludeCertPKDirsUsage  = "Keep known cert private key directories and all files in them"

	FlagIncludeNewUsage = "Keep new files created by target during dynamic analysis"

	FlagKeepTmpArtifactsUsage = "Keep temporary artifacts when command is done"

	FlagIncludeAppNuxtDirUsage            = "Keep the root Nuxt.js app directory"
	FlagIncludeAppNuxtBuildDirUsage       = "Keep the build Nuxt.js app directory"
	FlagIncludeAppNuxtDistDirUsage        = "Keep the dist Nuxt.js app directory"
	FlagIncludeAppNuxtStaticDirUsage      = "Keep the static asset directory for Nuxt.js apps"
	FlagIncludeAppNuxtNodeModulesDirUsage = "Keep the node modules directory for Nuxt.js apps"

	FlagIncludeAppNextDirUsage            = "Keep the root Next.js app directory"
	FlagIncludeAppNextBuildDirUsage       = "Keep the build directory for Next.js app"
	FlagIncludeAppNextDistDirUsage        = "Keep the static SPA directory for Next.js apps"
	FlagIncludeAppNextStaticDirUsage      = "Keep the static public asset directory for Next.js apps"
	FlagIncludeAppNextNodeModulesDirUsage = "Keep the node modules directory for Next.js apps"

	FlagIncludeNodePackageUsage = "Keep node.js package by name"

	FlagKeepPermsUsage = "Keep artifact permissions as-is"

	FlagNewEntrypointUsage = "New ENTRYPOINT instruction for the optimized image"
	FlagNewCmdUsage        = "New CMD instruction for the optimized image"
	FlagNewVolumeUsage     = "New VOLUME instructions for the optimized image"
	FlagNewLabelUsage      = "New LABEL instructions for the optimized image"
	FlagNewExposeUsage     = "New EXPOSE instructions for the optimized image"
	FlagNewWorkdirUsage    = "New WORKDIR instruction for the optimized image"
	FlagNewEnvUsage        = "New ENV instructions for the optimized image"
	FlagRemoveExposeUsage  = "Remove EXPOSE instructions for the optimized image"
	FlagRemoveEnvUsage     = "Remove ENV instructions for the optimized image"
	FlagRemoveLabelUsage   = "Remove LABEL instructions for the optimized image"
	FlagRemoveVolumeUsage  = "Remove VOLUME instructions for the optimized image"

	FlagTagUsage = "Custom tags for the generated image"

	FlagImageOverridesUsage = "Save runtime overrides in generated image (values is 'all' or a comma delimited list of override types: 'entrypoint', 'cmd', 'workdir', 'env', 'expose', 'volume', 'label')"

	FlagIncludeBinFileUsage = "File with shared binary file names to include from image"
	FlagIncludeExeFileUsage = "File with executable file names to include from image"

	FlagTagFatUsage              = "Custom tag for the fat image built from Dockerfile"
	FlagBuildFromDockerfileUsage = "The source Dockerfile name to build the fat image before it's optimized"
	FlagDockerfileContextUsage   = "The build context directory when building source Dockerfile"
	FlagCBOAddHostUsage          = "Add an extra host-to-IP mapping in /etc/hosts to use when building an image"
	FlagCBOBuildArgUsage         = "Add a build-time variable"
	FlagCBOLabelUsage            = "Add a label when building from Dockerfiles"
	FlagCBOTargetUsage           = "Target stage to build for multi-stage Dockerfiles"
	FlagCBONetworkUsage          = "Networking mode to use for the RUN instructions at build-time"
	FlagCBOCacheFromUsage        = "Add an image to the build cache"

	FlagObfuscateMetadataUsage = "Obfuscate the standard system and application metadata to make it more challenging to identify the image components"
)

var Flags = map[string]cli.Flag{
	FlagShowBuildLogs: &cli.BoolFlag{
		Name:    FlagShowBuildLogs,
		Usage:   FlagShowBuildLogsUsage,
		EnvVars: []string{"DSLIM_SHOW_BLOGS"},
	},
	FlagPathPerms: &cli.StringSliceFlag{
		Name:    FlagPathPerms,
		Value:   cli.NewStringSlice(),
		Usage:   FlagPathPermsUsage,
		EnvVars: []string{"DSLIM_PATH_PERMS"},
	},
	FlagPathPermsFile: &cli.StringFlag{
		Name:    FlagPathPermsFile,
		Value:   "",
		Usage:   FlagPathPermsFileUsage,
		EnvVars: []string{"DSLIM_PATH_PERMS_FILE"},
	},
	FlagPreservePath: &cli.StringSliceFlag{
		Name:    FlagPreservePath,
		Value:   cli.NewStringSlice(),
		Usage:   FlagPreservePathUsage,
		EnvVars: []string{"DSLIM_PRESERVE_PATH"},
	},
	FlagPreservePathFile: &cli.StringFlag{
		Name:    FlagPreservePathFile,
		Value:   "",
		Usage:   FlagPreservePathFileUsage,
		EnvVars: []string{"DSLIM_PRESERVE_PATH_FILE"},
	},
	FlagIncludePath: &cli.StringSliceFlag{
		Name:    FlagIncludePath,
		Value:   cli.NewStringSlice(),
		Usage:   FlagIncludePathUsage,
		EnvVars: []string{"DSLIM_INCLUDE_PATH"},
	},
	FlagIncludePathFile: &cli.StringFlag{
		Name:    FlagIncludePathFile,
		Value:   "",
		Usage:   FlagIncludePathFileUsage,
		EnvVars: []string{"DSLIM_INCLUDE_PATH_FILE"},
	},
	FlagIncludeBin: &cli.StringSliceFlag{
		Name:    FlagIncludeBin,
		Value:   cli.NewStringSlice(),
		Usage:   FlagIncludeBinUsage,
		EnvVars: []string{"DSLIM_INCLUDE_BIN"},
	},
	FlagIncludeDirBins: &cli.StringSliceFlag{
		Name:    FlagIncludeDirBins,
		Value:   cli.NewStringSlice(),
		Usage:   FlagIncludeDirBinsUsage,
		EnvVars: []string{"DSLIM_INCLUDE_DIR_BINS"},
	},
	FlagIncludeExe: &cli.StringSliceFlag{
		Name:    FlagIncludeExe,
		Value:   cli.NewStringSlice(),
		Usage:   FlagIncludeExeUsage,
		EnvVars: []string{"DSLIM_INCLUDE_EXE"},
	},
	FlagIncludeShell: &cli.BoolFlag{
		Name:    FlagIncludeShell,
		Usage:   FlagIncludeShellUsage,
		EnvVars: []string{"DSLIM_INCLUDE_SHELL"},
	},
	////
	FlagIncludeWorkdir: &cli.BoolFlag{
		Name:    FlagIncludeWorkdir,
		Usage:   FlagIncludeWorkdirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_WORKDIR"},
	},
	FlagIncludeAppImageAll: &cli.BoolFlag{
		Name:    FlagIncludeAppImageAll,
		Usage:   FlagIncludeAppImageAllUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_IMAGE_ALL"},
	},
	FlagAppImageStartInstGroup: &cli.IntFlag{
		Name:    FlagAppImageStartInstGroup,
		Value:   -1,
		Usage:   FlagAppImageStartInstGroupUsage,
		EnvVars: []string{"DSLIM_APP_IMAGE_START_INST_GROUP"},
	},
	FlagAppImageStartInst: &cli.StringFlag{
		Name:    FlagAppImageStartInst,
		Usage:   FlagAppImageStartInstUsage,
		EnvVars: []string{"DSLIM_APP_IMAGE_START_INST"},
	},
	FlagAppImageDockerfile: &cli.StringFlag{
		Name:    FlagAppImageDockerfile,
		Usage:   FlagAppImageDockerfileUsage,
		EnvVars: []string{"DSLIM_APP_IMAGE_DOCKERFILE"},
	},
	////
	FlagIncludePathsCreportFile: &cli.StringFlag{
		Name:    FlagIncludePathsCreportFile,
		Value:   "",
		Usage:   FlagIncludePathsCreportFileUsage,
		EnvVars: []string{"DSLIM_INCLUDE_PATHS_CREPORT_FILE"},
	},
	////
	FlagIncludeOSLibsNet: &cli.BoolFlag{
		Name:    FlagIncludeOSLibsNet,
		Value:   true, //enabled by default
		Usage:   FlagIncludeOSLibsNetUsage,
		EnvVars: []string{"DSLIM_INCLUDE_OSLIBS_NET"},
	},
	////
	FlagIncludeSSHClient: &cli.BoolFlag{
		Name:    FlagIncludeSSHClient,
		Value:   false, //disabled by default (for now)
		Usage:   FlagIncludeSSHClientUsage,
		EnvVars: []string{"DSLIM_INCLUDE_SSH_CLIENT"},
	},
	////
	FlagIncludeZoneInfo: &cli.BoolFlag{
		Name:    FlagIncludeZoneInfo,
		Value:   false,
		Usage:   FlagIncludeZoneInfoUsage,
		EnvVars: []string{"DSLIM_INCLUDE_ZONEINFO"},
	},
	////
	FlagIncludeCertAll: &cli.BoolFlag{
		Name:    FlagIncludeCertAll,
		Value:   true, //enabled by default
		Usage:   FlagIncludeCertAllUsage,
		EnvVars: []string{"DSLIM_INCLUDE_CERT_ALL"},
	},
	FlagIncludeCertBundles: &cli.BoolFlag{
		Name:    FlagIncludeCertBundles,
		Usage:   FlagIncludeCertBundlesUsage,
		EnvVars: []string{"DSLIM_INCLUDE_CERT_BUNDLES"},
	},
	FlagIncludeCertDirs: &cli.BoolFlag{
		Name:    FlagIncludeCertDirs,
		Usage:   FlagIncludeCertDirsUsage,
		EnvVars: []string{"DSLIM_INCLUDE_CERT_DIRS"},
	},
	FlagIncludeCertPKAll: &cli.BoolFlag{
		Name:    FlagIncludeCertPKAll,
		Usage:   FlagIncludeCertPKAllUsage,
		EnvVars: []string{"DSLIM_INCLUDE_CERT_PK_ALL"},
	},
	FlagIncludeCertPKDirs: &cli.BoolFlag{
		Name:    FlagIncludeCertPKDirs,
		Usage:   FlagIncludeCertPKDirsUsage,
		EnvVars: []string{"DSLIM_INCLUDE_CERT_PK_DIRS"},
	},
	FlagIncludeNew: &cli.BoolFlag{
		Name:    FlagIncludeNew,
		Value:   true, //enabled by default for now to keep the original behavior until minification works the same
		Usage:   FlagIncludeNewUsage,
		EnvVars: []string{"DSLIM_INCLUDE_NEW"},
	},
	////
	FlagKeepTmpArtifacts: &cli.BoolFlag{
		Name:    FlagKeepTmpArtifacts,
		Usage:   FlagKeepTmpArtifactsUsage,
		EnvVars: []string{"DSLIM_KEEP_TMP_ARTIFACTS"},
	},
	FlagIncludeAppNuxtDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNuxtDir,
		Usage:   FlagIncludeAppNuxtDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NUXT_DIR"},
	},
	FlagIncludeAppNuxtBuildDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNuxtBuildDir,
		Usage:   FlagIncludeAppNuxtBuildDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NUXT_BUILD_DIR"},
	},
	FlagIncludeAppNuxtDistDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNuxtDistDir,
		Usage:   FlagIncludeAppNuxtDistDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NUXT_DIST_DIR"},
	},
	FlagIncludeAppNuxtStaticDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNuxtStaticDir,
		Usage:   FlagIncludeAppNuxtStaticDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NUXT_STATIC_DIR"},
	},
	FlagIncludeAppNuxtNodeModulesDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNuxtNodeModulesDir,
		Usage:   FlagIncludeAppNuxtNodeModulesDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NUXT_NM_DIR"},
	},
	FlagIncludeAppNextDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNextDir,
		Usage:   FlagIncludeAppNextDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NEXT_DIR"},
	},
	FlagIncludeAppNextBuildDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNextBuildDir,
		Usage:   FlagIncludeAppNextBuildDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NEXT_BUILD_DIR"},
	},
	FlagIncludeAppNextDistDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNextDistDir,
		Usage:   FlagIncludeAppNextDistDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NEXT_DIST_DIR"},
	},
	FlagIncludeAppNextStaticDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNextStaticDir,
		Usage:   FlagIncludeAppNextStaticDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NEXT_STATIC_DIR"},
	},
	FlagIncludeAppNextNodeModulesDir: &cli.BoolFlag{
		Name:    FlagIncludeAppNextNodeModulesDir,
		Usage:   FlagIncludeAppNextNodeModulesDirUsage,
		EnvVars: []string{"DSLIM_INCLUDE_APP_NEXT_NM_DIR"},
	},
	FlagIncludeNodePackage: &cli.StringSliceFlag{
		Name:    FlagIncludeNodePackage,
		Value:   cli.NewStringSlice(),
		Usage:   FlagIncludeNodePackageUsage,
		EnvVars: []string{"DSLIM_INCLUDE_NODE_PKG"},
	},
	FlagKeepPerms: &cli.BoolFlag{
		Name:    FlagKeepPerms,
		Value:   true, //enabled by default
		Usage:   FlagKeepPermsUsage,
		EnvVars: []string{"DSLIM_KEEP_PERMS"},
	},
	//"EXCLUDE" FLAGS - START
	FlagExcludePattern: &cli.StringSliceFlag{
		Name:    FlagExcludePattern,
		Value:   cli.NewStringSlice(),
		Usage:   FlagExcludePatternUsage,
		EnvVars: []string{"DSLIM_EXCLUDE_PATTERN"},
	},
	FlagExcludeVarLockFiles: &cli.BoolFlag{
		Name:    FlagExcludeVarLockFiles, //true by default
		Value:   true,
		Usage:   FlagExcludeVarLockFilesUsage,
		EnvVars: []string{"DSLIM_EXCLUDE_VARLOCK"},
	},
	FlagExcludeMounts: &cli.BoolFlag{
		Name:    FlagExcludeMounts, //true by default
		Value:   true,
		Usage:   FlagExcludeMountsUsage,
		EnvVars: []string{"DSLIM_EXCLUDE_MOUNTS"},
	},
	//"EXCLUDE" FLAGS - END
	FlagNewEntrypoint: &cli.StringFlag{
		Name:    FlagNewEntrypoint,
		Value:   "",
		Usage:   FlagNewEntrypointUsage,
		EnvVars: []string{"DSLIM_NEW_ENTRYPOINT"},
	},
	FlagNewCmd: &cli.StringFlag{
		Name:    FlagNewCmd,
		Value:   "",
		Usage:   FlagNewCmdUsage,
		EnvVars: []string{"DSLIM_NEW_CMD"},
	},
	FlagNewExpose: &cli.StringSliceFlag{
		Name:    FlagNewExpose,
		Value:   cli.NewStringSlice(),
		Usage:   FlagNewExposeUsage,
		EnvVars: []string{"DSLIM_NEW_EXPOSE"},
	},
	FlagNewWorkdir: &cli.StringFlag{
		Name:    FlagNewWorkdir,
		Value:   "",
		Usage:   FlagNewWorkdirUsage,
		EnvVars: []string{"DSLIM_NEW_WORKDIR"},
	},
	FlagNewEnv: &cli.StringSliceFlag{
		Name:    FlagNewEnv,
		Value:   cli.NewStringSlice(),
		Usage:   FlagNewEnvUsage,
		EnvVars: []string{"DSLIM_NEW_ENV"},
	},
	FlagNewVolume: &cli.StringSliceFlag{
		Name:    FlagNewVolume,
		Value:   cli.NewStringSlice(),
		Usage:   FlagNewVolumeUsage,
		EnvVars: []string{"DSLIM_NEW_VOLUME"},
	},
	FlagNewLabel: &cli.StringSliceFlag{
		Name:    FlagNewLabel,
		Value:   cli.NewStringSlice(),
		Usage:   FlagNewLabelUsage,
		EnvVars: []string{"DSLIM_NEW_LABEL"},
	},
	FlagTag: &cli.StringSliceFlag{
		Name:    FlagTag,
		Value:   cli.NewStringSlice(),
		Usage:   FlagTagUsage,
		EnvVars: []string{"DSLIM_TARGET_TAG"},
	},
	FlagImageOverrides: &cli.StringFlag{
		Name:    FlagImageOverrides,
		Value:   "",
		Usage:   FlagImageOverridesUsage,
		EnvVars: []string{"DSLIM_TARGET_OVERRIDES"},
	},
	//Container Build Options
	FlagBuildFromDockerfile: &cli.StringFlag{
		Name:    FlagBuildFromDockerfile,
		Value:   "",
		Usage:   FlagBuildFromDockerfileUsage,
		EnvVars: []string{"DSLIM_BUILD_DOCKERFILE"},
	},
	FlagDockerfileContext: &cli.StringFlag{
		Name:    FlagDockerfileContext,
		Value:   "",
		Usage:   FlagDockerfileContextUsage,
		EnvVars: []string{"DSLIM_BUILD_DOCKERFILE_CTX"},
	},
	FlagTagFat: &cli.StringFlag{
		Name:    FlagTagFat,
		Value:   "",
		Usage:   FlagTagFatUsage,
		EnvVars: []string{"DSLIM_TARGET_TAG_FAT"},
	},
	FlagCBOAddHost: &cli.StringSliceFlag{
		Name:    FlagCBOAddHost,
		Value:   cli.NewStringSlice(),
		Usage:   FlagCBOAddHostUsage,
		EnvVars: []string{"DSLIM_CBO_ADD_HOST"},
	},
	FlagCBOBuildArg: &cli.StringSliceFlag{
		Name:    FlagCBOBuildArg,
		Value:   cli.NewStringSlice(),
		Usage:   FlagCBOBuildArgUsage,
		EnvVars: []string{"DSLIM_CBO_BUILD_ARG"},
	},
	FlagCBOCacheFrom: &cli.StringSliceFlag{
		Name:    FlagCBOCacheFrom,
		Value:   cli.NewStringSlice(),
		Usage:   FlagCBOCacheFromUsage,
		EnvVars: []string{"DSLIM_CBO_CACHE_FROM"},
	},
	FlagCBOLabel: &cli.StringSliceFlag{
		Name:    FlagCBOLabel,
		Value:   cli.NewStringSlice(),
		Usage:   FlagCBOLabelUsage,
		EnvVars: []string{"DSLIM_CBO_LABEL"},
	},
	FlagCBOTarget: &cli.StringFlag{
		Name:    FlagCBOTarget,
		Value:   "",
		Usage:   FlagCBOTargetUsage,
		EnvVars: []string{"DSLIM_CBO_TARGET"},
	},
	FlagCBONetwork: &cli.StringFlag{
		Name:    FlagCBONetwork,
		Value:   "",
		Usage:   FlagCBONetworkUsage,
		EnvVars: []string{"DSLIM_CBO_NETWORK"},
	},
	FlagImageBuildEngine: &cli.StringFlag{
		Name:    FlagImageBuildEngine,
		Value:   IBEInternal,
		Usage:   FlagImageBuildEngineUsage,
		EnvVars: []string{"DSLIM_IMAGE_BUILD_ENG"},
	},
	FlagImageBuildArch: &cli.StringFlag{
		Name:    FlagImageBuildArch,
		Usage:   FlagImageBuildArchUsage,
		EnvVars: []string{"DSLIM_IMAGE_BUILD_ARCH"},
	},
	FlagDeleteFatImage: &cli.BoolFlag{
		Name:    FlagDeleteFatImage,
		Usage:   FlagDeleteFatImageUsage,
		EnvVars: []string{"DSLIM_DELETE_FAT"},
	},
	FlagRemoveExpose: &cli.StringSliceFlag{
		Name:    FlagRemoveExpose,
		Value:   cli.NewStringSlice(),
		Usage:   FlagRemoveExposeUsage,
		EnvVars: []string{"DSLIM_RM_EXPOSE"},
	},
	FlagRemoveEnv: &cli.StringSliceFlag{
		Name:    FlagRemoveEnv,
		Value:   cli.NewStringSlice(),
		Usage:   FlagRemoveEnvUsage,
		EnvVars: []string{"DSLIM_RM_ENV"},
	},
	FlagRemoveLabel: &cli.StringSliceFlag{
		Name:    FlagRemoveLabel,
		Value:   cli.NewStringSlice(),
		Usage:   FlagRemoveLabelUsage,
		EnvVars: []string{"DSLIM_RM_LABEL"},
	},
	FlagRemoveVolume: &cli.StringSliceFlag{
		Name:    FlagRemoveVolume,
		Value:   cli.NewStringSlice(),
		Usage:   FlagRemoveVolumeUsage,
		EnvVars: []string{"DSLIM_RM_VOLUME"},
	},
	FlagIncludeBinFile: &cli.StringFlag{
		Name:    FlagIncludeBinFile,
		Value:   "",
		Usage:   FlagIncludeBinFileUsage,
		EnvVars: []string{"DSLIM_INCLUDE_BIN_FILE"},
	},
	FlagIncludeExeFile: &cli.StringFlag{
		Name:    FlagIncludeExeFile,
		Value:   "",
		Usage:   FlagIncludeExeFileUsage,
		EnvVars: []string{"DSLIM_INCLUDE_EXE_FILE"},
	},
	////
	FlagObfuscateMetadata: &cli.BoolFlag{
		Name:    FlagObfuscateMetadata,
		Usage:   FlagObfuscateMetadataUsage,
		EnvVars: []string{"DSLIM_OBFUSCATE_METADATA"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}

func GetContainerBuildOptions(ctx *cli.Context) (*config.ContainerBuildOptions, error) {
	cbo := &config.ContainerBuildOptions{
		Labels: map[string]string{},
	}

	cbo.Dockerfile = ctx.String(FlagBuildFromDockerfile)
	cbo.DockerfileContext = ctx.String(FlagDockerfileContext)
	cbo.Tag = ctx.String(FlagTagFat)
	cbo.Target = ctx.String(FlagCBOTarget)
	cbo.NetworkMode = ctx.String(FlagCBONetwork)
	cbo.CacheFrom = ctx.StringSlice(FlagCBOCacheFrom)

	hosts := ctx.StringSlice(FlagCBOAddHost)
	//TODO: figure out how to encode multiple host entries to a string (docs are not helpful)
	cbo.ExtraHosts = strings.Join(hosts, ",")

	rawBuildArgs := ctx.StringSlice(FlagCBOBuildArg)
	for _, rba := range rawBuildArgs {
		//need to handle:
		//NAME=VALUE
		//"NAME"="VALUE"
		//NAME <- value is copied from the env var with the same name
		parts := strings.SplitN(rba, "=", 2)
		switch len(parts) {
		case 2:
			if strings.HasPrefix(parts[0], "\"") {
				parts[0] = strings.Trim(parts[0], "\"")
				parts[1] = strings.Trim(parts[1], "\"")
			} else {
				parts[0] = strings.Trim(parts[0], "'")
				parts[1] = strings.Trim(parts[1], "'")
			}
			ba := config.CBOBuildArg{
				Name:  parts[0],
				Value: parts[1],
			}

			cbo.BuildArgs = append(cbo.BuildArgs, ba)
		case 1:
			if envVal := os.Getenv(parts[0]); envVal != "" {
				ba := config.CBOBuildArg{
					Name:  parts[0],
					Value: envVal,
				}

				cbo.BuildArgs = append(cbo.BuildArgs, ba)
			}
		default:
			fmt.Printf("GetContainerBuildOptions(): unexpected build arg format - '%v'\n", rba)
		}
	}

	rawLabels := ctx.StringSlice(FlagCBOLabel)
	for _, rlabel := range rawLabels {
		parts := strings.SplitN(rlabel, "=", 2)
		switch len(parts) {
		case 2:
			if strings.HasPrefix(parts[0], "\"") {
				parts[0] = strings.Trim(parts[0], "\"")
				parts[1] = strings.Trim(parts[1], "\"")
			} else {
				parts[0] = strings.Trim(parts[0], "'")
				parts[1] = strings.Trim(parts[1], "'")
			}

			cbo.Labels[parts[0]] = parts[1]
		case 1:
			if envVal := os.Getenv(parts[0]); envVal != "" {
				cbo.Labels[parts[0]] = envVal
			}
		default:
			fmt.Printf("GetContainerBuildOptions(): unexpected label format - '%v'\n", rlabel)
		}
	}

	return cbo, nil
}

//TODO: move/share when the 'edit' command needs these flags too

func GetImageInstructions(ctx *cli.Context) (*config.ImageNewInstructions, error) {
	entrypoint := ctx.String(FlagNewEntrypoint)
	cmd := ctx.String(FlagNewCmd)
	expose := ctx.StringSlice(FlagNewExpose)
	removeExpose := ctx.StringSlice(FlagRemoveExpose)

	instructions := &config.ImageNewInstructions{
		Workdir: ctx.String(FlagNewWorkdir),
		Env:     ctx.StringSlice(FlagNewEnv),
	}

	volumes, err := command.ParseTokenSet(ctx.StringSlice(FlagNewVolume))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid new volume options %v\n", err)
		return nil, err
	}

	instructions.Volumes = volumes

	labels, err := command.ParseTokenMap(ctx.StringSlice(FlagNewLabel))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid new label options %v\n", err)
		return nil, err
	}

	instructions.Labels = labels

	removeLabels, err := command.ParseTokenSet(ctx.StringSlice(FlagRemoveLabel))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid remove label options %v\n", err)
		return nil, err
	}

	instructions.RemoveLabels = removeLabels

	removeEnvs, err := command.ParseTokenSet(ctx.StringSlice(FlagRemoveEnv))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid remove env options %v\n", err)
		return nil, err
	}

	instructions.RemoveEnvs = removeEnvs

	removeVolumes, err := command.ParseTokenSet(ctx.StringSlice(FlagRemoveVolume))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid remove volume options %v\n", err)
		return nil, err
	}

	instructions.RemoveVolumes = removeVolumes

	//TODO(future): also load instructions from a file

	if len(expose) > 0 {
		instructions.ExposedPorts, err = command.ParseDockerExposeOpt(expose)
		if err != nil {
			log.Errorf("getImageInstructions(): invalid expose options => %v", err)
			return nil, err
		}
	}

	if len(removeExpose) > 0 {
		instructions.RemoveExposedPorts, err = command.ParseDockerExposeOpt(removeExpose)
		if err != nil {
			log.Errorf("getImageInstructions(): invalid remove-expose options => %v", err)
			return nil, err
		}
	}

	instructions.Entrypoint, err = command.ParseExec(entrypoint)
	if err != nil {
		log.Errorf("getImageInstructions(): invalid entrypoint option => %v", err)
		return nil, err
	}

	//one space is a hacky way to indicate that you want to remove this instruction from the image
	instructions.ClearEntrypoint = command.IsOneSpace(entrypoint)

	instructions.Cmd, err = command.ParseExec(cmd)
	if err != nil {
		log.Errorf("getImageInstructions(): invalid cmd option => %v", err)
		return nil, err
	}

	//same hack to indicate you want to remove this instruction
	instructions.ClearCmd = command.IsOneSpace(cmd)

	return instructions, nil
}

func GetAppNodejsInspectOptions(ctx *cli.Context) config.AppNodejsInspectOptions {
	return config.AppNodejsInspectOptions{
		IncludePackages: ctx.StringSlice(FlagIncludeNodePackage),
		NextOpts:        getAppNextInspectOptions(ctx),
		NuxtOpts:        getAppNuxtInspectOptions(ctx),
	}
}

func getAppNextInspectOptions(ctx *cli.Context) config.NodejsWebFrameworkInspectOptions {
	return config.NodejsWebFrameworkInspectOptions{
		IncludeAppDir:         ctx.Bool(FlagIncludeAppNextDir),
		IncludeBuildDir:       ctx.Bool(FlagIncludeAppNextBuildDir),
		IncludeDistDir:        ctx.Bool(FlagIncludeAppNextDistDir),
		IncludeStaticDir:      ctx.Bool(FlagIncludeAppNextStaticDir),
		IncludeNodeModulesDir: ctx.Bool(FlagIncludeAppNextNodeModulesDir),
	}
}

func getAppNuxtInspectOptions(ctx *cli.Context) config.NodejsWebFrameworkInspectOptions {
	return config.NodejsWebFrameworkInspectOptions{
		IncludeAppDir:         ctx.Bool(FlagIncludeAppNuxtDir),
		IncludeBuildDir:       ctx.Bool(FlagIncludeAppNuxtBuildDir),
		IncludeDistDir:        ctx.Bool(FlagIncludeAppNuxtDistDir),
		IncludeStaticDir:      ctx.Bool(FlagIncludeAppNuxtStaticDir),
		IncludeNodeModulesDir: ctx.Bool(FlagIncludeAppNuxtNodeModulesDir),
	}
}

func GetKubernetesOptions(ctx *cli.Context) (config.KubernetesOptions, error) {
	cfg := config.KubernetesOptions{
		Target: config.KubernetesTarget{
			Workload:  ctx.String(command.FlagTargetKubeWorkload),
			Namespace: ctx.String(command.FlagTargetKubeWorkloadNamespace),
			Container: ctx.String(command.FlagTargetKubeWorkloadContainer),
		},
		TargetOverride: config.KubernetesTargetOverride{
			Image: ctx.String(command.FlagTargetKubeWorkloadImage),
		},
		Manifests:  ctx.StringSlice(command.FlagKubeManifestFile),
		Kubeconfig: ctx.String(command.FlagKubeKubeconfigFile),
	}

	if len(cfg.Target.Namespace)+len(cfg.Target.Container)+len(cfg.TargetOverride.Image)+len(cfg.Manifests) > 0 && cfg.Target.Workload == "" {
		return cfg, errors.New("--target-kube-workload flag must be provided")
	}

	return cfg, nil
}

const (
	IBENone     = "none"
	IBEInternal = "internal"
	IBEDocker   = "docker"
	IBEBuildKit = "buildkit"
)

func getImageBuildEngine(ctx *cli.Context) (string, error) {
	value := ctx.String(FlagImageBuildEngine)
	switch value {
	case IBENone, IBEInternal, IBEDocker, IBEBuildKit:
		return value, nil
	default:
		return "", fmt.Errorf("bad value")
	}
}

const (
	ArchEmpty = ""
	ArchAmd64 = "amd64"
	ArchArm64 = "arm64"
)

func getImageBuildArch(ctx *cli.Context) (string, error) {
	value := ctx.String(FlagImageBuildArch)
	switch value {
	case ArchEmpty, ArchAmd64, ArchArm64:
		return value, nil
	default:
		return "", fmt.Errorf("bad value")
	}
}
