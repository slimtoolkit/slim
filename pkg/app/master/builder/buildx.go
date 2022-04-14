package builder

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pluginmanager "github.com/docker/cli/cli-plugins/manager"
	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile"
)

/*
TODO
1. Document the need for qemu on docker host: docker run --privileged --rm tonistiigi/binfmt --install all

*/

// BasicImageBuilderBuildx creates regular container images
type BasicImageBuilderBuildx struct {
	bxPlugin      *pluginmanager.Plugin
	buildxArgs    []string
	buildCtx      string
	showBuildLogs bool
	buildLog      bytes.Buffer
}

type BuildxOptions struct {
	config.ContainerBuildOptions

	Tags []string
}

func (o BuildxOptions) toArgs() (args []string, err error) {

	if o.Buildx.Builder != "" {
		args = append(args, "--builder", o.Buildx.Builder)
	}

	args = append(args, "build")

	if o.ExtraHosts != "" {
		args = append(args, "--add-host", o.ExtraHosts)
	}

	for _, v := range o.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", v.Name, v.Value))
	}

	for _, v := range o.CacheFrom {
		args = append(args, "--cache-from", v)
	}

	args = append(args, "--file", o.Dockerfile)

	for k, v := range parseLabels(o.Labels) {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	if o.NetworkMode != "" {
		args = append(args, "--network", o.NetworkMode)
	}

	if o.Buildx.NoCache {
		args = append(args, "--no-cache")
	}

	for _, v := range o.Buildx.Exports {
		args = append(args, "--output", v)
	}

	for _, v := range o.Buildx.Platforms {
		args = append(args, "--platform", v)
	}

	args = append(args, "--progress", o.Buildx.ProgressMode)

	if o.Buildx.Pull {
		args = append(args, "--pull")
	}

	if o.Target != "" {
		args = append(args, "--target", o.Target)
	}

	for _, v := range o.Tags {
		args = append(args, "--tag", v)
	}

	args = append(args, o.DockerfileContext)

	return args, nil
}

// ImageBuilderBuildx creates new optimized container images
type ImageBuilderBuildx struct {
	BasicImageBuilderBuildx

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

// NewBasicImageBuilderBuildx creates a new BasicImageBuilderBuildx instances
func NewBasicImageBuilderBuildx(
	cbOpts *config.ContainerBuildOptions,
	buildContext string,
	showBuildLogs bool) (*BasicImageBuilderBuildx, error) {

	dockerfilePath := cbOpts.Dockerfile
	if p, err := filepath.Abs(dockerfilePath); err == nil {
		dockerfilePath = p
	}

	builder := BasicImageBuilderBuildx{
		showBuildLogs: showBuildLogs,
		bxPlugin:      cbOpts.Buildx.Plugin,
		buildCtx:      buildContext,
	}

	cbo := *cbOpts
	cbo.Dockerfile = dockerfilePath
	cbo.DockerfileContext = buildContext
	cbo.Labels = parseLabels(cbOpts.Labels)

	opts := BuildxOptions{
		ContainerBuildOptions: cbo,
		Tags:                  []string{cbOpts.Tag},
	}

	var err error
	if builder.buildxArgs, err = opts.toArgs(); err != nil {
		return nil, err
	}

	return &builder, nil
}

// Build creates a new container image
func (b *BasicImageBuilderBuildx) Build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, b.bxPlugin.Path, b.buildxArgs...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = &b.buildLog
	cmd.Stderr = &b.buildLog
	return cmd.Run()
}

func (b *BasicImageBuilderBuildx) GetLogs() string {
	bl := b.buildLog.String()

	// Ignore CLI usage on errors.
	sb := strings.Builder{}
	const usageLine = "Usage:"
	logScanner := bufio.NewScanner(strings.NewReader(bl))
	for logScanner.Scan() {
		line := logScanner.Text()
		if strings.TrimSpace(line) == usageLine {
			break
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (b *BasicImageBuilderBuildx) HasData() bool {
	return false
}

// NewImageBuilderBuildx creates a new ImageBuilderBuildx instances
func NewImageBuilderBuildx(
	imageRepoNameTag string,
	additionalTags []string,
	imageInfo *docker.Image,
	bxOpts config.BuildxOptions,
	artifactDir string,
	showBuildLogs bool,
	overrideSelectors map[string]bool,
	overrides *config.ContainerOverrides,
	instructions *config.ImageNewInstructions) (*ImageBuilderBuildx, error) {

	labels := parseLabels(imageInfo.Config.Labels)

	builder := &ImageBuilderBuildx{
		BasicImageBuilderBuildx: BasicImageBuilderBuildx{
			showBuildLogs: showBuildLogs,
			bxPlugin:      bxOpts.Plugin,
			buildCtx:      artifactDir,
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

	opts := BuildxOptions{
		ContainerBuildOptions: config.ContainerBuildOptions{
			Dockerfile:        filepath.Join(artifactDir, "Dockerfile"),
			DockerfileContext: artifactDir,
			Labels:            labels,
			Buildx:            bxOpts,
		},
		Tags: append([]string{imageRepoNameTag}, additionalTags...),
	}

	var err error
	if builder.buildxArgs, err = opts.toArgs(); err != nil {
		return nil, err
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
		log.Debugf("NewImageBuilderBuildx: Using container runtime overrides => %+v", overrideSelectors)
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
		log.Debugf("NewImageBuilderBuildx: Using new image instructions => %+v", instructions)

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

	builder.data = getDataName(builder.buildCtx)

	return builder, nil
}

// Build creates a new container image
func (b *ImageBuilderBuildx) Build(ctx context.Context) error {
	if err := b.generateDockerfile(); err != nil {
		return err
	}

	return b.BasicImageBuilderBuildx.Build(ctx)
}

// generateDockerfile creates a Dockerfile file
func (b *ImageBuilderBuildx) generateDockerfile() error {
	return dockerfile.GenerateFromInfo(b.buildCtx,
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
