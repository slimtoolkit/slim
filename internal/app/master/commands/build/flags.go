package build

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Build command flag names
const (
	FlagShowBuildLogs = "show-blogs"

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

	FlagTag    = "tag"
	FlagTagFat = "tag-fat"

	FlagImageOverrides = "image-overrides"

	FlagBuildFromDockerfile = "dockerfile"

	FlagIncludeBinFile = "include-bin-file"
	FlagIncludeExeFile = "include-exe-file"
)

// Build command flag usage info
const (
	FlagShowBuildLogsUsage = "Show build logs"

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

	FlagTagUsage    = "Custom tag for the generated image"
	FlagTagFatUsage = "Custom tag for the fat image built from Dockerfile"

	FlagImageOverridesUsage = "Save runtime overrides in generated image (values is 'all' or a comma delimited list of override types: 'entrypoint', 'cmd', 'workdir', 'env', 'expose', 'volume', 'label')"

	FlagBuildFromDockerfileUsage = "The source Dockerfile name to build the fat image before it's optimized"

	FlagIncludeBinFileUsage = "File with shared binary file names to include from image"
	FlagIncludeExeFileUsage = "File with executable file names to include from image"
)

var Flags = map[string]cli.Flag{
	FlagShowBuildLogs: cli.BoolFlag{
		Name:   FlagShowBuildLogs,
		Usage:  FlagShowBuildLogsUsage,
		EnvVar: "DSLIM_SHOW_BLOGS",
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
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
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
