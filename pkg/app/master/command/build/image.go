package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/builder"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/consts"
	"github.com/slimtoolkit/slim/pkg/imagebuilder"
	"github.com/slimtoolkit/slim/pkg/imagebuilder/internalbuilder"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

func inspectFatImage(
	xc *app.ExecutionContext,
	targetRef string,
	doPull bool,
	doShowPullLogs bool,
	rtaOnbuildBaseImage bool,
	dockerConfigPath string,
	registryAccount string,
	registrySecret string,
	paramsStatePath string,
	client *dockerapi.Client,
	logger *log.Entry,
	cmdReport *report.BuildCommand,
) (*image.Inspector, string, string, string) {
	imageInspector, err := image.NewInspector(client, targetRef)
	xc.FailOn(err)

	noImage, err := imageInspector.NoImage()
	errutil.FailOn(err)
	if noImage {
		if doPull {
			xc.Out.Info("target.image",
				ovars{
					"status":  "image.not.found",
					"image":   targetRef,
					"message": "trying to pull target image",
				})

			err := imageInspector.Pull(doShowPullLogs, dockerConfigPath, registryAccount, registrySecret)
			xc.FailOn(err)
		} else {
			xc.Out.Info("target.image.error",
				ovars{
					"status":  "image.not.found",
					"image":   targetRef,
					"message": "make sure the target image already exists locally (use --pull flag to auto-download it from registry)",
				})

			exitCode := command.ECTCommon | command.ECCImageNotFound
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			xc.Exit(exitCode)
		}
	}

	logger.Tracef("targetRef=%s ii.ImageRef=%s", targetRef, imageInspector.ImageRef)
	cmdReport.TargetReference = imageInspector.ImageRef

	xc.Out.State("image.inspection.start")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	xc.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(paramsStatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	xc.Out.Info("image",
		ovars{
			"id":         imageInspector.ImageInfo.ID,
			"size.bytes": imageInspector.ImageInfo.VirtualSize,
			"size.human": humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
		})

	if imageInspector.ImageInfo.Config != nil &&
		len(imageInspector.ImageInfo.Config.Labels) > 0 {
		for labelName := range imageInspector.ImageInfo.Config.Labels {
			if labelName == consts.DSLabelVersion {
				xc.Out.Info("target.image.error",
					ovars{
						"status":  "image.already.optimized",
						"image":   targetRef,
						"message": "the target image is already optimized",
					})

				exitCode := command.ECTBuild | ecbImageAlreadyOptimized
				xc.Out.State("exited",
					ovars{
						"exit.code": exitCode,
					})

				cmdReport.Error = "image.already.optimized"
				xc.Exit(exitCode)
			}
		}
	}

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	xc.FailOn(err)

	if imageInspector.DockerfileInfo != nil {
		if imageInspector.DockerfileInfo.ExeUser != "" {
			xc.Out.Info("image.users",
				ovars{
					"exe": imageInspector.DockerfileInfo.ExeUser,
					"all": strings.Join(imageInspector.DockerfileInfo.AllUsers, ","),
				})
		}

		if len(imageInspector.DockerfileInfo.ImageStack) > 0 {
			cmdReport.ImageStack = imageInspector.DockerfileInfo.ImageStack

			for idx, layerInfo := range imageInspector.DockerfileInfo.ImageStack {
				xc.Out.Info("image.stack",
					ovars{
						"index": idx,
						"name":  layerInfo.FullName,
						"id":    layerInfo.ID,
					})
			}
		}

		if len(imageInspector.DockerfileInfo.ExposedPorts) > 0 {
			xc.Out.Info("image.exposed_ports",
				ovars{
					"list": strings.Join(imageInspector.DockerfileInfo.ExposedPorts, ","),
				})
		}

		if !rtaOnbuildBaseImage && imageInspector.DockerfileInfo.HasOnbuild {
			xc.Out.Info("target.image.error",
				ovars{
					"status":  "onbuild.base.image",
					"image":   targetRef,
					"message": "Runtime analysis for onbuild base images is not supported",
				})

			exitCode := command.ECTBuild | ecbOnbuildBaseImage
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			cmdReport.Error = "onbuild.base.image"
			xc.Exit(exitCode)
		}
	}

	xc.Out.State("image.inspection.done")
	return imageInspector, localVolumePath, statePath, stateKey
}

func buildFatImage(
	xc *app.ExecutionContext,
	targetRef string,
	customImageTag string,
	cbOpts *config.ContainerBuildOptions,
	doShowBuildLogs bool,
	client *dockerapi.Client,
	cmdReport *report.BuildCommand,
) (fatImageRepoNameTag string) {
	xc.Out.State("building",
		ovars{
			"message": "building basic image",
		})

	//create a fat image name:
	//* use the explicit fat image tag if provided
	//* or create one based on the user provided (slim image) custom tag if it's available
	//* otherwise auto-generate a name
	if cbOpts.Tag != "" {
		fatImageRepoNameTag = cbOpts.Tag
	} else if customImageTag != "" {
		citParts := strings.Split(customImageTag, ":")
		switch len(citParts) {
		case 1:
			fatImageRepoNameTag = fmt.Sprintf("%s.fat", customImageTag)
		case 2:
			fatImageRepoNameTag = fmt.Sprintf("%s.fat:%s", citParts[0], citParts[1])
		default:
			xc.Out.Info("param.error",
				ovars{
					"status": "malformed.custom.image.tag",
					"value":  customImageTag,
				})

			exitCode := command.ECTBuild | ecbBadCustomImageTag
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
					"version":   v.Current(),
					"location":  fsutil.ExeDir(),
				})

			cmdReport.Error = "malformed.custom.image.tag"
			xc.Exit(exitCode)
		}
	} else {
		fatImageRepoNameTag = fmt.Sprintf("slim-tmp-fat-image.%v.%v",
			os.Getpid(), time.Now().UTC().Format("20060102150405"))
	}

	cbOpts.Tag = fatImageRepoNameTag

	xc.Out.Info("basic.image.info",
		ovars{
			"tag":        cbOpts.Tag,
			"dockerfile": cbOpts.Dockerfile,
			"context":    targetRef,
		})

	fatBuilder, err := builder.NewBasicImageBuilder(
		client,
		cbOpts,
		targetRef,
		doShowBuildLogs)
	xc.FailOn(err)

	err = fatBuilder.Build()

	if doShowBuildLogs || err != nil {
		xc.Out.LogDump("regular.image.build", fatBuilder.BuildLog.String(),
			ovars{
				"tag": cbOpts.Tag,
			})
	}

	if err != nil {
		xc.Out.Info("build.error",
			ovars{
				"status": "standard.image.build.error",
				"value":  err,
			})

		exitCode := command.ECTBuild | ecbImageBuildError
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		xc.Exit(exitCode)
	}

	xc.Out.State("basic.image.build.completed")

	return fatImageRepoNameTag
}

func buildOutputImage(
	xc *app.ExecutionContext,
	customImageTag string,
	additionalTags []string,
	cbOpts *config.ContainerBuildOptions,
	overrides *config.ContainerOverrides,
	imageOverrideSelectors map[string]bool,
	instructions *config.ImageNewInstructions,
	doDeleteFatImage bool,
	doShowBuildLogs bool,
	imageInspector *image.Inspector,
	client *dockerapi.Client,
	logger *log.Entry,
	cmdReport *report.BuildCommand,
	imageBuildEngine string,
	imageBuildArch string,
) string {
	onError := func(e error) {
		xc.Out.Info("build.error",
			ovars{
				"status": "optimized.image.build.error",
				"error":  e,
			})

		exitCode := command.ECTBuild | ecbImageBuildError
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		cmdReport.Error = "optimized.image.build.error"
		xc.Exit(exitCode)
	}

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	cmdReport.ImageBuildEngine = imageBuildEngine

	logger.Debugf("image build engine - %v", imageBuildEngine)
	xc.Out.State("building",
		ovars{
			"message": "building optimized image",
			"engine":  imageBuildEngine,
		})

	var outputImageName string
	var hasData bool
	var imageCreated bool
	switch imageBuildEngine {
	case IBENone:
	case IBEInternal:
		engine, err := internalbuilder.New(doShowBuildLogs,
			true, //pushToDaemon - TODO: have a param to control this &
			//output image tar (if not 'saving' to daemon)
			false)
		xc.FailOn(err)

		opts := imagebuilder.SimpleBuildOptions{
			ImageConfig: imagebuilder.ImageConfig{
				Architecture: imageBuildArch,
				Config: imagebuilder.RunConfig{
					ExposedPorts: map[string]struct{}{},
					Volumes:      map[string]struct{}{},
					Labels:       map[string]string{},
				},
			},
		}

		if customImageTag != "" {
			//must be first
			opts.Tags = append(opts.Tags, customImageTag)
		}

		if len(additionalTags) > 0 {
			opts.Tags = append(opts.Tags, additionalTags...)
		}

		UpdateBuildOptionsWithSrcImageInfo(&opts, imageInspector.ImageInfo)
		UpdateBuildOptionsWithOverrides(&opts, imageOverrideSelectors, overrides)

		if imageInspector.ImageRef != "" {
			opts.ImageConfig.Config.Labels[consts.DSLabelSourceImage] = imageInspector.ImageRef
		}

		var sourceImageID string
		if imageInspector.ImageInfo != nil &&
			imageInspector.ImageInfo.ID != "" {
			sourceImageID = imageInspector.ImageInfo.ID
		}

		if sourceImageID == "" &&
			imageInspector.ImageRecordInfo.ID != "" {
			sourceImageID = imageInspector.ImageRecordInfo.ID
		}

		if sourceImageID != "" {
			opts.ImageConfig.Config.Labels[consts.DSLabelSourceImageID] = sourceImageID
		}

		opts.ImageConfig.Config.Labels[consts.DSLabelVersion] = v.Current()

		//(new) instructions have higher value precedence over the runtime overrides
		UpdateBuildOptionsWithNewInstructions(&opts, instructions)

		dataTar := filepath.Join(imageInspector.ArtifactLocation, "files.tar")
		if fsutil.Exists(dataTar) &&
			fsutil.IsRegularFile(dataTar) &&
			fsutil.IsTarFile(dataTar) {
			layerInfo := imagebuilder.LayerDataInfo{
				Type:   imagebuilder.TarSource,
				Source: dataTar,
				Params: &imagebuilder.DataParams{
					TargetPath: "/",
				},
			}

			opts.Layers = append(opts.Layers, layerInfo)
			hasData = true
		} else {
			dataDir := filepath.Join(imageInspector.ArtifactLocation, "files")
			if fsutil.Exists(dataTar) && fsutil.IsDir(dataDir) {
				layerInfo := imagebuilder.LayerDataInfo{
					Type:   imagebuilder.DirSource,
					Source: dataDir,
					Params: &imagebuilder.DataParams{
						TargetPath: "/",
					},
				}

				opts.Layers = append(opts.Layers, layerInfo)
				hasData = true
			} else {
				logger.Info("WARNING - no data artifacts")
			}
		}

		imageResult, err := engine.Build(opts)
		if err != nil {
			onError(err)
		}

		outputImageName = imageResult.Name // customImageTag // engine.RepoName
		cmdReport.MinifiedImageID = imageResult.ID
		cmdReport.MinifiedImageDigest = imageResult.Digest
		imageCreated = true
	case IBEBuildKit:
	case IBEDocker:
		engine, err := builder.NewImageBuilder(
			client,
			customImageTag,
			additionalTags,
			imageInspector.ImageInfo,
			imageInspector.ArtifactLocation,
			doShowBuildLogs,
			imageOverrideSelectors,
			overrides,
			instructions,
			imageInspector.ImageRef)
		xc.FailOn(err)

		if !engine.HasData {
			logger.Info("WARNING - no data artifacts")
		}

		err = engine.Build()
		if doShowBuildLogs || err != nil {
			xc.Out.LogDump("optimized.image.build", engine.BuildLog.String(),
				ovars{
					"tag": customImageTag,
				})
		}

		if err != nil {
			onError(err)
		}

		if cbOpts.Dockerfile != "" {
			if doDeleteFatImage {
				xc.Out.Info("Dockerfile", ovars{
					"image.name":        cbOpts.Tag,
					"image.fat.deleted": "true",
				})
				var err = client.RemoveImage(cbOpts.Tag)
				errutil.WarnOn(err)
			} else {
				xc.Out.Info("Dockerfile", ovars{
					"image.name":        cbOpts.Tag,
					"image.fat.deleted": "false",
				})
			}
		}

		outputImageName = engine.RepoName
		hasData = engine.HasData
		imageCreated = true
	default:
		logger.Errorf("bad image build engine - %v", imageBuildEngine)
		onError(fmt.Errorf("bad image build engine - %v", imageBuildEngine))
	}

	cmdReport.State = cmd.StateCompleted
	cmdReport.ImageCreated = imageCreated
	cmdReport.MinifiedImage = outputImageName
	cmdReport.MinifiedImageHasData = hasData

	xc.Out.State("completed")

	return outputImageName
}

// NOTE: lots of C&P from image_builder (TODO: refactor)
const (
	dsCmdPortInfo = "65501/tcp"
	dsEvtPortInfo = "65502/tcp"
)

func UpdateBuildOptionsWithNewInstructions(
	options *imagebuilder.SimpleBuildOptions,
	instructions *config.ImageNewInstructions) {
	if instructions != nil {
		log.Debugf("NewImageBuilder: Using new image instructions => %+v", instructions)

		if instructions.Workdir != "" {
			options.ImageConfig.Config.WorkingDir = instructions.Workdir
		}

		if len(instructions.Env) > 0 {
			options.ImageConfig.Config.Env = append(options.ImageConfig.Config.Env, instructions.Env...)
		}

		for k, v := range instructions.ExposedPorts {
			options.ImageConfig.Config.ExposedPorts[string(k)] = v
		}

		for k, v := range instructions.Volumes {
			options.ImageConfig.Config.Volumes[k] = v
		}

		for k, v := range instructions.Labels {
			options.ImageConfig.Config.Labels[k] = v
		}

		if len(instructions.Entrypoint) > 0 {
			options.ImageConfig.Config.Entrypoint = instructions.Entrypoint
		}

		if len(instructions.Cmd) > 0 {
			options.ImageConfig.Config.Cmd = instructions.Cmd
		}

		if len(options.ImageConfig.Config.ExposedPorts) > 0 &&
			len(instructions.RemoveExposedPorts) > 0 {
			for k := range instructions.RemoveExposedPorts {
				if _, ok := options.ImageConfig.Config.ExposedPorts[string(k)]; ok {
					delete(options.ImageConfig.Config.ExposedPorts, string(k))
				}
			}
		}

		if len(options.ImageConfig.Config.Volumes) > 0 &&
			len(instructions.RemoveVolumes) > 0 {
			for k := range instructions.RemoveVolumes {
				if _, ok := options.ImageConfig.Config.Volumes[k]; ok {
					delete(options.ImageConfig.Config.Volumes, k)
				}
			}
		}

		if len(options.ImageConfig.Config.Labels) > 0 &&
			len(instructions.RemoveLabels) > 0 {
			for k := range instructions.RemoveLabels {
				if _, ok := options.ImageConfig.Config.Labels[k]; ok {
					delete(options.ImageConfig.Config.Labels, k)
				}
			}
		}

		if len(instructions.RemoveEnvs) > 0 &&
			len(options.ImageConfig.Config.Env) > 0 {
			var newEnv []string
			for _, envPair := range options.ImageConfig.Config.Env {
				envParts := strings.SplitN(envPair, "=", 2)
				if len(envParts) > 0 && envParts[0] != "" {
					if _, ok := instructions.RemoveEnvs[envParts[0]]; !ok {
						newEnv = append(newEnv, envPair)
					}
				}
			}

			options.ImageConfig.Config.Env = newEnv
		}
	}
}

func UpdateBuildOptionsWithOverrides(
	options *imagebuilder.SimpleBuildOptions,
	overrideSelectors map[string]bool,
	overrides *config.ContainerOverrides) {
	if overrides != nil && len(overrideSelectors) > 0 {
		log.Debugf("UpdateBuildOptionsWithOverrides: Using container runtime overrides => %+v", overrideSelectors)
		for k := range overrideSelectors {
			switch k {
			case "entrypoint":
				if len(overrides.Entrypoint) > 0 {
					options.ImageConfig.Config.Entrypoint = overrides.Entrypoint
				}
			case "cmd":
				if len(overrides.Cmd) > 0 {
					options.ImageConfig.Config.Cmd = overrides.Cmd
				}
			case "workdir":
				if overrides.Workdir != "" {
					options.ImageConfig.Config.WorkingDir = overrides.Workdir
				}
			case "env":
				if len(overrides.Env) > 0 {
					options.ImageConfig.Config.Env = append(options.ImageConfig.Config.Env, overrides.Env...)
				}
			case "label":
				for k, v := range overrides.Labels {
					options.ImageConfig.Config.Labels[k] = v
				}
			case "volume":
				for k, v := range overrides.Volumes {
					options.ImageConfig.Config.Volumes[k] = v
				}
			case "expose":
				dsCmdPort := dockerapi.Port(dsCmdPortInfo)
				dsEvtPort := dockerapi.Port(dsEvtPortInfo)

				for k, v := range overrides.ExposedPorts {
					if k == dsCmdPort || k == dsEvtPort {
						continue
					}
					options.ImageConfig.Config.ExposedPorts[string(k)] = v
				}
			}
		}
	}
}

func UpdateBuildOptionsWithSrcImageInfo(
	options *imagebuilder.SimpleBuildOptions,
	imageInfo *dockerapi.Image) {
	labels := SourceToOutputImageLabels(imageInfo.Config.Labels)
	for k, v := range labels {
		options.ImageConfig.Config.Labels[k] = v
	}

	//note: not passing imageInfo.OS explicitly
	//because it gets "hardcoded" to "linux" internally
	//(other OS types are not supported)
	if options.ImageConfig.Architecture == "" {
		options.ImageConfig.Architecture = imageInfo.Architecture
	}

	options.ImageConfig.Config.User = imageInfo.Config.User
	options.ImageConfig.Config.Entrypoint = imageInfo.Config.Entrypoint
	options.ImageConfig.Config.Cmd = imageInfo.Config.Cmd
	options.ImageConfig.Config.WorkingDir = imageInfo.Config.WorkingDir
	options.ImageConfig.Config.Env = imageInfo.Config.Env
	options.ImageConfig.Config.Volumes = imageInfo.Config.Volumes
	options.ImageConfig.Config.OnBuild = imageInfo.Config.OnBuild
	options.ImageConfig.Config.StopSignal = imageInfo.Config.StopSignal

	for k, v := range imageInfo.Config.ExposedPorts {
		options.ImageConfig.Config.ExposedPorts[string(k)] = v
	}

	if options.ImageConfig.Config.ExposedPorts == nil {
		options.ImageConfig.Config.ExposedPorts = map[string]struct{}{}
	}

	if options.ImageConfig.Config.Volumes == nil {
		options.ImageConfig.Config.Volumes = map[string]struct{}{}
	}

	if options.ImageConfig.Config.Labels == nil {
		options.ImageConfig.Config.Labels = map[string]string{}
	}
}

func SourceToOutputImageLabels(srcLabels map[string]string) map[string]string {
	labels := map[string]string{}
	if srcLabels != nil {
		//cleanup non-standard labels from buildpacks
		for k, v := range srcLabels {
			lineLen := len(k) + len(v) + 7
			if lineLen > 65535 {
				//TODO: improve JSON data splitting
				valueLen := len(v)
				parts := valueLen / 50000
				parts++
				offset := 0
				for i := 0; i < parts && offset < valueLen; i++ {
					chunkSize := 50000
					if (offset + chunkSize) > valueLen {
						chunkSize = valueLen - offset
					}
					value := v[offset:(offset + chunkSize)]
					offset += chunkSize
					key := fmt.Sprintf("%s.%d", k, i)
					labels[key] = value
				}
			} else {
				labels[k] = v
			}
		}
	}

	return labels
}
