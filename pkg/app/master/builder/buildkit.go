package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/util/buildflags"
	"github.com/docker/buildx/util/progress"
	docker "github.com/fsouza/go-dockerclient"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/reverse"
)

/*
TODO
1. Document the need for qemu on docker host: docker run --privileged --rm tonistiigi/binfmt --install all

*/

// BasicImageBuilderBuildKit creates regular container images
type BasicImageBuilderBuildKit struct {
	showBuildLogs bool
	buildOptions  build.Options
	buildLog      bytes.Buffer
	bkOpts        config.BuildKitOptions
}

// ImageBuilderBuildKit creates new optimized container images
type ImageBuilderBuildKit struct {
	BasicImageBuilderBuildKit

	id           string
	entrypoint   []string
	cmd          []string
	workingDir   string
	env          []string
	labels       map[string]string
	exposedPorts map[docker.Port]struct{}
	volumes      map[string]struct{}
	onBuild      []string
	user         string
	// data to be copied into the image. May be empty.
	data string
}

// NewBasicImageBuilderBuildKit creates a new BasicImageBuilderBuildKit instances
func NewBasicImageBuilderBuildKit(
	cbOpts *config.ContainerBuildOptions,
	buildContext string,
	showBuildLogs bool) (*BasicImageBuilderBuildKit, error) {

	buildArgs := make(map[string]string, len(cbOpts.BuildArgs))
	for _, ba := range cbOpts.BuildArgs {
		buildArgs[ba.Name] = ba.Value
	}

	dockerfilePath := cbOpts.Dockerfile
	if p, err := filepath.Abs(dockerfilePath); err == nil {
		dockerfilePath = p
	}

	opts := build.Options{
		Inputs: build.Inputs{
			ContextPath:    buildContext,
			DockerfilePath: dockerfilePath,
			InStream:       os.Stdin,
		},
		BuildArgs:   buildArgs,
		ExtraHosts:  strings.Split(cbOpts.ExtraHosts, ","),
		Labels:      parseLabels(cbOpts.Labels),
		NetworkMode: cbOpts.NetworkMode,
		NoCache:     cbOpts.BuildKit.NoCache,
		Pull:        cbOpts.BuildKit.Pull,
		Tags:        []string{cbOpts.Tag},
		Target:      cbOpts.Target,
		Platforms:   []v1.Platform{cbOpts.BuildKit.Platforms[0]},
		Exports:     cbOpts.BuildKit.Exports,
	}

	var err error
	if opts.CacheFrom, err = buildflags.ParseCacheEntry(cbOpts.CacheFrom); err != nil {
		return nil, err
	}

	builder := BasicImageBuilderBuildKit{
		showBuildLogs: showBuildLogs,
		buildOptions:  opts,
		bkOpts:        cbOpts.BuildKit,
	}

	return &builder, nil
}

const (
	defaultTargetName = "default"
)

// Build creates a new container image
func (b *BasicImageBuilderBuildKit) Build(ctx context.Context) error {
	opts := map[string]build.Options{
		defaultTargetName: b.buildOptions,
	}

	pctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	printer := progress.NewPrinter(pctx, &b.buildLog, os.Stderr, b.bkOpts.ProgressMode)

	_, err := build.Build(ctx, b.bkOpts.DriverInfos, opts, b.bkOpts.ClientGetter, b.bkOpts.ConfigDir, printer)
	if werr := printer.Wait(); err == nil {
		err = werr
	}
	if err != nil {
		return err
	}

	// printWarnings(os.Stderr, printer.Warnings(), progressMode)

	return nil
}

func (b *BasicImageBuilderBuildKit) GetLogs() string {
	return b.buildLog.String()
}

func (b *BasicImageBuilderBuildKit) HasData() bool {
	return false
}

// NewImageBuilderBuildKit creates a new ImageBuilderBuildKit instances
func NewImageBuilderBuildKit(
	imageRepoNameTag string,
	additionalTags []string,
	imageInfo *docker.Image,
	bkOpts config.BuildKitOptions,
	artifactDir string,
	showBuildLogs bool,
	overrideSelectors map[string]bool,
	overrides *config.ContainerOverrides,
	instructions *config.ImageNewInstructions) (*ImageBuilderBuildKit, error) {

	labels := parseLabels(imageInfo.Config.Labels)

	opts := build.Options{
		Inputs: build.Inputs{
			ContextPath:    artifactDir,
			DockerfilePath: filepath.Join(artifactDir, "Dockerfile"),
			InStream:       os.Stdin,
		},
		Labels:    labels,
		NoCache:   bkOpts.NoCache,
		Pull:      bkOpts.Pull,
		Tags:      append([]string{imageRepoNameTag}, additionalTags...),
		Platforms: bkOpts.Platforms,
		Exports:   bkOpts.Exports,
	}

	// if opts.CacheFrom, err = buildflags.ParseCacheEntry(cbOpts.CacheFrom); err != nil {
	// 	return nil, err
	// }

	builder := &ImageBuilderBuildKit{
		BasicImageBuilderBuildKit: BasicImageBuilderBuildKit{
			showBuildLogs: showBuildLogs,
			buildOptions:  opts,
			bkOpts:        bkOpts,
		},
		id:           imageInfo.ID,
		entrypoint:   imageInfo.Config.Entrypoint,
		cmd:          imageInfo.Config.Cmd,
		workingDir:   imageInfo.Config.WorkingDir,
		env:          imageInfo.Config.Env,
		labels:       labels,
		exposedPorts: imageInfo.Config.ExposedPorts,
		volumes:      imageInfo.Config.Volumes,
		onBuild:      imageInfo.Config.OnBuild,
		user:         imageInfo.Config.User,
	}

	if builder.exposedPorts == nil {
		builder.exposedPorts = map[docker.Port]struct{}{}
	}

	if builder.volumes == nil {
		builder.volumes = map[string]struct{}{}
	}

	if builder.labels == nil {
		builder.labels = map[string]string{}
	}

	if overrides != nil && len(overrideSelectors) > 0 {
		log.Debugf("NewImageBuilderBuildKit: Using container runtime overrides => %+v", overrideSelectors)
		for k := range overrideSelectors {
			switch k {
			case "entrypoint":
				if len(overrides.Entrypoint) > 0 {
					builder.entrypoint = overrides.Entrypoint
				}
			case "cmd":
				if len(overrides.Cmd) > 0 {
					builder.cmd = overrides.Cmd
				}
			case "workdir":
				if overrides.Workdir != "" {
					builder.workingDir = overrides.Workdir
				}
			case "env":
				if len(overrides.Env) > 0 {
					builder.env = append(builder.env, instructions.Env...)
				}
			case "label":
				for k, v := range overrides.Labels {
					builder.labels[k] = v
				}
			case "volume":
				for k, v := range overrides.Volumes {
					builder.volumes[k] = v
				}
			case "expose":
				dsCmdPort := docker.Port(dsCmdPortInfo)
				dsEvtPort := docker.Port(dsEvtPortInfo)

				for k, v := range overrides.ExposedPorts {
					if k == dsCmdPort || k == dsEvtPort {
						continue
					}
					builder.exposedPorts[k] = v
				}
			}
		}
	}

	//instructions have higher value precedence over the runtime overrides
	if instructions != nil {
		log.Debugf("NewImageBuilderBuildKit: Using new image instructions => %+v", instructions)

		if instructions.Workdir != "" {
			builder.workingDir = instructions.Workdir
		}

		if len(instructions.Env) > 0 {
			builder.env = append(builder.env, instructions.Env...)
		}

		for k, v := range instructions.ExposedPorts {
			builder.exposedPorts[k] = v
		}

		for k, v := range instructions.Volumes {
			builder.volumes[k] = v
		}

		for k, v := range instructions.Labels {
			builder.labels[k] = v
		}

		if len(instructions.Entrypoint) > 0 {
			builder.entrypoint = instructions.Entrypoint
		}

		if len(instructions.Cmd) > 0 {
			builder.cmd = instructions.Cmd
		}

		if len(builder.exposedPorts) > 0 {
			for k := range instructions.RemoveExposedPorts {
				delete(builder.exposedPorts, k)
			}
		}

		if len(builder.volumes) > 0 {
			for k := range instructions.RemoveVolumes {
				delete(builder.volumes, k)
			}
		}

		if len(builder.labels) > 0 {
			for k := range instructions.RemoveLabels {
				delete(builder.labels, k)
			}
		}

		if len(instructions.RemoveEnvs) > 0 &&
			len(builder.env) > 0 {
			var newEnv []string
			for _, envPair := range builder.env {
				envParts := strings.SplitN(envPair, "=", 2)
				if len(envParts) > 0 && envParts[0] != "" {
					if _, ok := instructions.RemoveEnvs[envParts[0]]; !ok {
						newEnv = append(newEnv, envPair)
					}
				}
			}

			builder.env = newEnv
		}
	}

	builder.data = getDataName(builder.buildOptions.Inputs.ContextPath)

	return builder, nil
}

// Build creates a new container image
func (b *ImageBuilderBuildKit) Build(ctx context.Context) error {
	if err := b.generateDockerfile(); err != nil {
		return err
	}

	return b.BasicImageBuilderBuildKit.Build(ctx)
}

// generateDockerfile creates a Dockerfile file
func (b *ImageBuilderBuildKit) generateDockerfile() error {
	return reverse.GenerateFromInfo(
		b.buildOptions.Inputs.ContextPath,
		b.volumes,
		b.workingDir,
		b.env,
		b.labels,
		b.user,
		b.exposedPorts,
		b.entrypoint,
		b.cmd,
		b.data,
	)
}

// parseLabels cleans up non-standard labels from buildpacks
func parseLabels(in map[string]string) map[string]string {
	labels := map[string]string{}
	for k, v := range in {
		if lineLen := len(k) + len(v) + 7; lineLen <= 65535 {
			labels[k] = v
			continue
		}

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
	}

	return labels
}
