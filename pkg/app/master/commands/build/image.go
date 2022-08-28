package build

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/builder"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/consts"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"
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

	if imageInspector.NoImage() {
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

			exitCode := commands.ECTBuild | ecbImageBuildError
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
			if labelName == consts.ContainerLabelName {
				xc.Out.Info("target.image.error",
					ovars{
						"status":  "image.already.optimized",
						"image":   targetRef,
						"message": "the target image is already optimized",
					})

				exitCode := commands.ECTBuild | ecbImageAlreadyOptimized
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

			exitCode := commands.ECTBuild | ecbOnbuildBaseImage
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

			exitCode := commands.ECTBuild | ecbBadCustomImageTag
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
		fatImageRepoNameTag = fmt.Sprintf("docker-slim-tmp-fat-image.%v.%v",
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

		exitCode := commands.ECTBuild | ecbImageBuildError
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

func buildSlimImage(
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
) string {
	xc.Out.State("building",
		ovars{
			"message": "building optimized image",
		})

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	builder, err := builder.NewImageBuilder(client,
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

	if !builder.HasData {
		logger.Info("WARNING - no data artifacts")
	}

	err = builder.Build()

	if doShowBuildLogs || err != nil {
		xc.Out.LogDump("optimized.image.build", builder.BuildLog.String(),
			ovars{
				"tag": customImageTag,
			})
	}

	if err != nil {
		xc.Out.Info("build.error",
			ovars{
				"status": "optimized.image.build.error",
				"error":  err,
			})

		exitCode := commands.ECTBuild | ecbImageBuildError
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		cmdReport.Error = "optimized.image.build.error"
		xc.Exit(exitCode)
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted

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

	cmdReport.MinifiedImage = builder.RepoName
	cmdReport.MinifiedImageHasData = builder.HasData

	return builder.RepoName
}
