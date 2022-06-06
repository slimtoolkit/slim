package build

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	pluginmanager "github.com/docker/cli/cli-plugins/manager"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerclient"
)

// Build command flag names
const (
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

	// Buildx flags.
	FlagUseBuildx       = "use-buildx"
	FlagBuildxPlatforms = "buildx-platforms"
	FlagBuildxBuilder   = "buildx-builder"
	FlagBuildxPush      = "buildx-push"
)

// Build command flag usage info
const (
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
	FlagUseBuildx: &cli.BoolFlag{
		Name:  FlagUseBuildx,
		Value: false,
		Usage: "Build with 'docker buildx'",
		EnvVars: []string{
			"DSLIM_USE_BUILDX",
			// https://docs.docker.com/develop/develop-images/build_enhancements/#to-enable-buildx-builds
			"DOCKER_BUILDX",
		},
	},
	FlagBuildxPlatforms: &cli.StringSliceFlag{
		Name:  FlagBuildxPlatforms,
		Value: cli.NewStringSlice(),
		Usage: "Platforms to build outputs for; currenly only one is allowed without --buildx-push. " +
			"Can only be used with --use-buildx. " +
			"If --dockerfile is set, only the first platform is used to build that Dockerfile's image. " +
			"All platforms are used when building the output image.",
		EnvVars: []string{"DSLIM_BUILDX_PLATFORMS"},
	},
	FlagBuildxBuilder: &cli.StringFlag{
		Name:  FlagBuildxBuilder,
		Usage: "The buildx builder to use instead of the default or current context's. Can only be used with --use-buildx",
		EnvVars: []string{
			"DSLIM_BUILDX_BUILDER",
			// https://github.com/docker/buildx/blob/a2d5bc7/commands/root.go#L82
			"BUILDX_BUILDER",
		},
	},
	FlagBuildxPush: &cli.BoolFlag{
		Name:    FlagBuildxPush,
		Value:   false,
		Usage:   "Push the resulting images to a registry. Can only be used with --use-buildx",
		EnvVars: []string{"DSLIM_BUILDX_PUSH"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}

func GetContainerBuildOptions(ctx *cli.Context, gparams *commands.GenericParams) (cbo *config.ContainerBuildOptions, err error) {
	cbo = &config.ContainerBuildOptions{
		Labels: map[string]string{},
	}

	cbo.Dockerfile = ctx.String(FlagBuildFromDockerfile)
	cbo.DockerfileContext = ctx.String(FlagDockerfileContext)
	cbo.Tag = ctx.String(FlagTagFat)
	cbo.Target = ctx.String(FlagCBOTarget)
	cbo.NetworkMode = ctx.String(FlagCBONetwork)
	cbo.CacheFrom = ctx.StringSlice(FlagCBOCacheFrom)

	// Since there are components of buildx init that may fail if buildx is not configured,
	// do not init buildx opts if not turned on.
	if ctx.Bool(FlagUseBuildx) {
		if cbo.Buildx, err = makeBuildxOptions(ctx, gparams, cbo.DockerfileContext); err != nil {
			return nil, err
		}
	}

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

func makeBuildxOptions(ctx *cli.Context, gparams *commands.GenericParams, ctxDir string) (bxOpts config.BuildxOptions, err error) {
	// A DockerCli is just an entrypoint into some convenience functions used by buildx.
	cliOpts := []command.DockerCliOption{
		command.WithInputStream(os.Stdin),
		command.WithErrorStream(os.Stderr),
		command.WithOutputStream(os.Stdout),
	}
	dockerCLI, err := command.NewDockerCli(cliOpts...)
	if err != nil {
		return config.BuildxOptions{}, err
	}

	makeClient := func(*command.DockerCli) (client.APIClient, error) {
		return dockerclient.NewAPIClient(gparams.ClientConfig)
	}
	initOpts := []command.InitializeOpt{
		command.WithInitializeClient(makeClient),
	}
	var hosts []string
	if gparams.ClientConfig.Host != "" {
		hosts = append(hosts, gparams.ClientConfig.Host)
	}
	var tlsOpts *tlsconfig.Options
	if gparams.ClientConfig.TLSCertPath != "" {
		tlsOpts = &tlsconfig.Options{
			CAFile:   filepath.Join(gparams.ClientConfig.TLSCertPath, "ca.pem"),
			CertFile: filepath.Join(gparams.ClientConfig.TLSCertPath, "cert.pem"),
			KeyFile:  filepath.Join(gparams.ClientConfig.TLSCertPath, "key.pem"),
		}
	}
	clientOpts := flags.ClientOptions{
		Common: &flags.CommonOptions{
			Debug:      gparams.Debug,
			Hosts:      hosts,
			LogLevel:   gparams.LogLevel,
			TLS:        gparams.ClientConfig.UseTLS,
			TLSVerify:  gparams.ClientConfig.VerifyTLS,
			TLSOptions: tlsOpts,
		},
	}
	if err := dockerCLI.Initialize(&clientOpts, initOpts...); err != nil {
		return config.BuildxOptions{}, err
	}
	bxOpts.CLI = dockerCLI

	if bxOpts.Plugin, err = pluginmanager.GetPlugin("buildx", dockerCLI, nil); err != nil {
		return config.BuildxOptions{}, err
	}
	if perr := bxOpts.Plugin.Err; perr != nil {
		return config.BuildxOptions{}, perr
	}

	bxOpts.Builder = ctx.String(FlagBuildxBuilder)
	bxOpts.Pull = ctx.Bool(commands.FlagPull)
	bxOpts.NoCache = false

	// Since logs are proxied via docker-slim, align with other options.
	if ctx.Bool(FlagShowBuildLogs) {
		bxOpts.ProgressMode = "plain"
	} else {
		bxOpts.ProgressMode = "quiet"
	}

	bxOpts.Push = ctx.Bool(FlagBuildxPush)
	// TODO(estroz): make outputs options
	if bxOpts.Push {
		bxOpts.Exports = append(bxOpts.Exports, "type=registry")
	} else {
		bxOpts.Exports = append(bxOpts.Exports, "type=docker")
	}

	platforms := ctx.StringSlice(FlagBuildxPlatforms)
	if len(platforms) == 0 {
		// Just use the current arch.
		bxOpts.Platforms = append(bxOpts.Platforms, path.Join(runtime.GOOS, runtime.GOARCH))
	} else {
		for _, pfs := range platforms {
			for _, pf := range strings.Split(pfs, ",") {
				if pf != "" {
					bxOpts.Platforms = append(bxOpts.Platforms, pf)
				}
			}
		}
	}
	if !bxOpts.Push && len(bxOpts.Platforms) > 1 {
		// TODO(estroz): remove this requirement if multi-platform builds are enabled.
		return bxOpts, fmt.Errorf("only one platform is allowed without --buildx-push")
	}

	return bxOpts, nil
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

	volumes, err := commands.ParseTokenSet(ctx.StringSlice(FlagNewVolume))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid new volume options %v\n", err)
		return nil, err
	}

	instructions.Volumes = volumes

	labels, err := commands.ParseTokenMap(ctx.StringSlice(FlagNewLabel))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid new label options %v\n", err)
		return nil, err
	}

	instructions.Labels = labels

	removeLabels, err := commands.ParseTokenSet(ctx.StringSlice(FlagRemoveLabel))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid remove label options %v\n", err)
		return nil, err
	}

	instructions.RemoveLabels = removeLabels

	removeEnvs, err := commands.ParseTokenSet(ctx.StringSlice(FlagRemoveEnv))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid remove env options %v\n", err)
		return nil, err
	}

	instructions.RemoveEnvs = removeEnvs

	removeVolumes, err := commands.ParseTokenSet(ctx.StringSlice(FlagRemoveVolume))
	if err != nil {
		fmt.Printf("getImageInstructions(): invalid remove volume options %v\n", err)
		return nil, err
	}

	instructions.RemoveVolumes = removeVolumes

	//TODO(future): also load instructions from a file

	if len(expose) > 0 {
		instructions.ExposedPorts, err = commands.ParseDockerExposeOpt(expose)
		if err != nil {
			log.Errorf("getImageInstructions(): invalid expose options => %v", err)
			return nil, err
		}
	}

	if len(removeExpose) > 0 {
		instructions.RemoveExposedPorts, err = commands.ParseDockerExposeOpt(removeExpose)
		if err != nil {
			log.Errorf("getImageInstructions(): invalid remove-expose options => %v", err)
			return nil, err
		}
	}

	instructions.Entrypoint, err = commands.ParseExec(entrypoint)
	if err != nil {
		log.Errorf("getImageInstructions(): invalid entrypoint option => %v", err)
		return nil, err
	}

	//one space is a hacky way to indicate that you want to remove this instruction from the image
	instructions.ClearEntrypoint = commands.IsOneSpace(entrypoint)

	instructions.Cmd, err = commands.ParseExec(cmd)
	if err != nil {
		log.Errorf("getImageInstructions(): invalid cmd option => %v", err)
		return nil, err
	}

	//same hack to indicate you want to remove this instruction
	instructions.ClearCmd = commands.IsOneSpace(cmd)

	return instructions, nil
}
