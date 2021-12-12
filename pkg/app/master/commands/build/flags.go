package build

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
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

	//FlagIncludeLicenses  = "include-licenses"

	FlagKeepTmpArtifacts = "keep-tmp-artifacts"

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

	FlagKeepTmpArtifactsUsage = "Keep temporary artifacts when command is done"

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
	FlagShowBuildLogs: cli.BoolFlag{
		Name:   FlagShowBuildLogs,
		Usage:  FlagShowBuildLogsUsage,
		EnvVar: "DSLIM_SHOW_BLOGS",
	},
	FlagPathPerms: cli.StringSliceFlag{
		Name:   FlagPathPerms,
		Value:  &cli.StringSlice{},
		Usage:  FlagPathPermsUsage,
		EnvVar: "DSLIM_PATH_PERMS",
	},
	FlagPathPermsFile: cli.StringFlag{
		Name:   FlagPathPermsFile,
		Value:  "",
		Usage:  FlagPathPermsFileUsage,
		EnvVar: "DSLIM_PATH_PERMS_FILE",
	},
	FlagPreservePath: cli.StringSliceFlag{
		Name:   FlagPreservePath,
		Value:  &cli.StringSlice{},
		Usage:  FlagPreservePathUsage,
		EnvVar: "DSLIM_PRESERVE_PATH",
	},
	FlagPreservePathFile: cli.StringFlag{
		Name:   FlagPreservePathFile,
		Value:  "",
		Usage:  FlagPreservePathFileUsage,
		EnvVar: "DSLIM_PRESERVE_PATH_FILE",
	},
	FlagIncludePath: cli.StringSliceFlag{
		Name:   FlagIncludePath,
		Value:  &cli.StringSlice{},
		Usage:  FlagIncludePathUsage,
		EnvVar: "DSLIM_INCLUDE_PATH",
	},
	FlagIncludePathFile: cli.StringFlag{
		Name:   FlagIncludePathFile,
		Value:  "",
		Usage:  FlagIncludePathFileUsage,
		EnvVar: "DSLIM_INCLUDE_PATH_FILE",
	},
	FlagIncludeBin: cli.StringSliceFlag{
		Name:   FlagIncludeBin,
		Value:  &cli.StringSlice{},
		Usage:  FlagIncludeBinUsage,
		EnvVar: "DSLIM_INCLUDE_BIN",
	},
	FlagIncludeExe: cli.StringSliceFlag{
		Name:   FlagIncludeExe,
		Value:  &cli.StringSlice{},
		Usage:  FlagIncludeExeUsage,
		EnvVar: "DSLIM_INCLUDE_EXE",
	},
	FlagIncludeShell: cli.BoolFlag{
		Name:   FlagIncludeShell,
		Usage:  FlagIncludeShellUsage,
		EnvVar: "DSLIM_INCLUDE_SHELL",
	},
	////
	FlagIncludeCertAll: cli.BoolFlag{
		Name:   FlagIncludeCertAll,
		Usage:  FlagIncludeCertAllUsage,
		EnvVar: "DSLIM_INCLUDE_CERT_ALL",
	},
	FlagIncludeCertBundles: cli.BoolFlag{
		Name:   FlagIncludeCertBundles,
		Usage:  FlagIncludeCertBundlesUsage,
		EnvVar: "DSLIM_INCLUDE_CERT_BUNDLES",
	},
	FlagIncludeCertDirs: cli.BoolFlag{
		Name:   FlagIncludeCertDirs,
		Usage:  FlagIncludeCertDirsUsage,
		EnvVar: "DSLIM_INCLUDE_CERT_DIRS",
	},
	FlagIncludeCertPKAll: cli.BoolFlag{
		Name:   FlagIncludeCertPKAll,
		Usage:  FlagIncludeCertPKAllUsage,
		EnvVar: "DSLIM_INCLUDE_CERT_PK_ALL",
	},
	FlagIncludeCertPKDirs: cli.BoolFlag{
		Name:   FlagIncludeCertPKDirs,
		Usage:  FlagIncludeCertPKDirsUsage,
		EnvVar: "DSLIM_INCLUDE_CERT_PK_DIRS",
	},
	////
	FlagKeepTmpArtifacts: cli.BoolFlag{
		Name:   FlagKeepTmpArtifacts,
		Usage:  FlagKeepTmpArtifactsUsage,
		EnvVar: "DSLIM_KEEP_TMP_ARTIFACTS",
	},
	FlagKeepPerms: cli.BoolTFlag{
		Name:   FlagKeepPerms,
		Usage:  FlagKeepPermsUsage,
		EnvVar: "DSLIM_KEEP_PERMS",
	},
	FlagNewEntrypoint: cli.StringFlag{
		Name:   FlagNewEntrypoint,
		Value:  "",
		Usage:  FlagNewEntrypointUsage,
		EnvVar: "DSLIM_NEW_ENTRYPOINT",
	},
	FlagNewCmd: cli.StringFlag{
		Name:   FlagNewCmd,
		Value:  "",
		Usage:  FlagNewCmdUsage,
		EnvVar: "DSLIM_NEW_CMD",
	},
	FlagNewExpose: cli.StringSliceFlag{
		Name:   FlagNewExpose,
		Value:  &cli.StringSlice{},
		Usage:  FlagNewExposeUsage,
		EnvVar: "DSLIM_NEW_EXPOSE",
	},
	FlagNewWorkdir: cli.StringFlag{
		Name:   FlagNewWorkdir,
		Value:  "",
		Usage:  FlagNewWorkdirUsage,
		EnvVar: "DSLIM_NEW_WORKDIR",
	},
	FlagNewEnv: cli.StringSliceFlag{
		Name:   FlagNewEnv,
		Value:  &cli.StringSlice{},
		Usage:  FlagNewEnvUsage,
		EnvVar: "DSLIM_NEW_ENV",
	},
	FlagNewVolume: cli.StringSliceFlag{
		Name:   FlagNewVolume,
		Value:  &cli.StringSlice{},
		Usage:  FlagNewVolumeUsage,
		EnvVar: "DSLIM_NEW_VOLUME",
	},
	FlagNewLabel: cli.StringSliceFlag{
		Name:   FlagNewLabel,
		Value:  &cli.StringSlice{},
		Usage:  FlagNewLabelUsage,
		EnvVar: "DSLIM_NEW_LABEL",
	},
	FlagTag: cli.StringSliceFlag{
		Name:   FlagTag,
		Value:  &cli.StringSlice{},
		Usage:  FlagTagUsage,
		EnvVar: "DSLIM_TARGET_TAG",
	},
	FlagImageOverrides: cli.StringFlag{
		Name:   FlagImageOverrides,
		Value:  "",
		Usage:  FlagImageOverridesUsage,
		EnvVar: "DSLIM_TARGET_OVERRIDES",
	},
	//Container Build Options
	FlagBuildFromDockerfile: cli.StringFlag{
		Name:   FlagBuildFromDockerfile,
		Value:  "",
		Usage:  FlagBuildFromDockerfileUsage,
		EnvVar: "DSLIM_BUILD_DOCKERFILE",
	},
	FlagDockerfileContext: cli.StringFlag{
		Name:   FlagDockerfileContext,
		Value:  "",
		Usage:  FlagDockerfileContextUsage,
		EnvVar: "DSLIM_BUILD_DOCKERFILE_CTX",
	},
	FlagTagFat: cli.StringFlag{
		Name:   FlagTagFat,
		Value:  "",
		Usage:  FlagTagFatUsage,
		EnvVar: "DSLIM_TARGET_TAG_FAT",
	},
	FlagCBOAddHost: cli.StringSliceFlag{
		Name:   FlagCBOAddHost,
		Value:  &cli.StringSlice{},
		Usage:  FlagCBOAddHostUsage,
		EnvVar: "DSLIM_CBO_ADD_HOST",
	},
	FlagCBOBuildArg: cli.StringSliceFlag{
		Name:   FlagCBOBuildArg,
		Value:  &cli.StringSlice{},
		Usage:  FlagCBOBuildArgUsage,
		EnvVar: "DSLIM_CBO_BUILD_ARG",
	},
	FlagCBOCacheFrom: cli.StringSliceFlag{
		Name:   FlagCBOCacheFrom,
		Value:  &cli.StringSlice{},
		Usage:  FlagCBOCacheFromUsage,
		EnvVar: "DSLIM_CBO_CACHE_FROM",
	},
	FlagCBOLabel: cli.StringSliceFlag{
		Name:   FlagCBOLabel,
		Value:  &cli.StringSlice{},
		Usage:  FlagCBOLabelUsage,
		EnvVar: "DSLIM_CBO_LABEL",
	},
	FlagCBOTarget: cli.StringFlag{
		Name:   FlagCBOTarget,
		Value:  "",
		Usage:  FlagCBOTargetUsage,
		EnvVar: "DSLIM_CBO_TARGET",
	},
	FlagCBONetwork: cli.StringFlag{
		Name:   FlagCBONetwork,
		Value:  "",
		Usage:  FlagCBONetworkUsage,
		EnvVar: "DSLIM_CBO_NETWORK",
	},
	FlagDeleteFatImage: cli.BoolFlag{
		Name:   FlagDeleteFatImage,
		Usage:  FlagDeleteFatImageUsage,
		EnvVar: "DSLIM_DELETE_FAT",
	},
	FlagRemoveExpose: cli.StringSliceFlag{
		Name:   FlagRemoveExpose,
		Value:  &cli.StringSlice{},
		Usage:  FlagRemoveExposeUsage,
		EnvVar: "DSLIM_RM_EXPOSE",
	},
	FlagRemoveEnv: cli.StringSliceFlag{
		Name:   FlagRemoveEnv,
		Value:  &cli.StringSlice{},
		Usage:  FlagRemoveEnvUsage,
		EnvVar: "DSLIM_RM_ENV",
	},
	FlagRemoveLabel: cli.StringSliceFlag{
		Name:   FlagRemoveLabel,
		Value:  &cli.StringSlice{},
		Usage:  FlagRemoveLabelUsage,
		EnvVar: "DSLIM_RM_LABEL",
	},
	FlagRemoveVolume: cli.StringSliceFlag{
		Name:   FlagRemoveVolume,
		Value:  &cli.StringSlice{},
		Usage:  FlagRemoveVolumeUsage,
		EnvVar: "DSLIM_RM_VOLUME",
	},
	FlagIncludeBinFile: cli.StringFlag{
		Name:   FlagIncludeBinFile,
		Value:  "",
		Usage:  FlagIncludeBinFileUsage,
		EnvVar: "DSLIM_INCLUDE_BIN_FILE",
	},
	FlagIncludeExeFile: cli.StringFlag{
		Name:   FlagIncludeExeFile,
		Value:  "",
		Usage:  FlagIncludeExeFileUsage,
		EnvVar: "DSLIM_INCLUDE_EXE_FILE",
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
