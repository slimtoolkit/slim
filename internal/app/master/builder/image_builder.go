package builder

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/reverse"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"

	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidContextDir = errors.New("invalid context directory")
)

// BasicImageBuilder creates regular container images
type BasicImageBuilder struct {
	ShowBuildLogs bool
	BuildOptions  docker.BuildImageOptions
	APIClient     *docker.Client
	BuildLog      bytes.Buffer
}

// ImageBuilder creates new optimized container images
type ImageBuilder struct {
	BasicImageBuilder
	RepoName     string
	ID           string
	Entrypoint   []string
	Cmd          []string
	WorkingDir   string
	Env          []string
	ExposedPorts map[docker.Port]struct{}
	Volumes      map[string]struct{}
	OnBuild      []string
	User         string
	HasData      bool
	TarData      bool
}

// NewImageBuilder creates a new BasicImageBuilder instances
func NewBasicImageBuilder(client *docker.Client,
	imageRepoNameTag string,
	dockerfileName string,
	buildContext string,
	showBuildLogs bool) (*BasicImageBuilder, error) {
	builder := BasicImageBuilder{
		ShowBuildLogs: showBuildLogs,
		BuildOptions: docker.BuildImageOptions{
			Name:           imageRepoNameTag,
			RmTmpContainer: true,
			Dockerfile:     dockerfileName,
		},
		APIClient: client,
	}

	if strings.HasPrefix(buildContext, "http://") || strings.HasPrefix(buildContext, "https://") {
		builder.BuildOptions.Remote = buildContext
	} else {
		if exists := fsutil.DirExists(buildContext); exists {
			builder.BuildOptions.ContextDir = buildContext
		} else {
			return nil, ErrInvalidContextDir
		}
	}

	builder.BuildOptions.OutputStream = &builder.BuildLog
	return &builder, nil
}

// Build creates a new container image
func (b *BasicImageBuilder) Build() error {
	return b.APIClient.BuildImage(b.BuildOptions)
}

// Remove deletes the configured container image
func (b *BasicImageBuilder) Remove() error {
	return nil
}

// NewImageBuilder creates a new ImageBuilder instances
func NewImageBuilder(client *docker.Client,
	imageRepoNameTag string,
	imageInfo *docker.Image,
	artifactLocation string,
	showBuildLogs bool,
	overrideSelectors map[string]bool,
	overrides *config.ContainerOverrides,
	instructions *config.ImageNewInstructions) (*ImageBuilder, error) {
	builder := &ImageBuilder{
		BasicImageBuilder: BasicImageBuilder{
			ShowBuildLogs: showBuildLogs,
			APIClient:     client,
			BuildOptions: docker.BuildImageOptions{
				Name:           imageRepoNameTag,
				RmTmpContainer: true,
				ContextDir:     artifactLocation,
				Dockerfile:     "Dockerfile",
				//SuppressOutput: true,
			},
		},
		RepoName:     imageRepoNameTag,
		ID:           imageInfo.ID,
		Entrypoint:   imageInfo.Config.Entrypoint,
		Cmd:          imageInfo.Config.Cmd,
		WorkingDir:   imageInfo.Config.WorkingDir,
		Env:          imageInfo.Config.Env,
		ExposedPorts: imageInfo.Config.ExposedPorts,
		Volumes:      imageInfo.Config.Volumes,
		OnBuild:      imageInfo.Config.OnBuild,
		User:         imageInfo.Config.User,
	}

	if overrides != nil && len(overrideSelectors) > 0 {
		log.Debugf("NewImageBuilder: Using container runtime overrides => %+v", overrideSelectors)
		for k := range overrideSelectors {
			switch k {
			case "entrypoint":
				if len(overrides.Entrypoint) > 0 {
					builder.Entrypoint = overrides.Entrypoint
				}
			case "cmd":
				if len(overrides.Cmd) > 0 {
					builder.Cmd = overrides.Cmd
				}
			case "workdir":
				if overrides.Workdir != "" {
					builder.WorkingDir = overrides.Workdir
				}
			case "env":
				if len(overrides.Env) > 0 {
					builder.Env = append(builder.Env, instructions.Env...)
				}
			case "expose":
				//TODO: refactor this port filter...
				dsCmdPort := docker.Port("65501/tcp")
				dsEnvPort := docker.Port("65502/tcp")

				if builder.ExposedPorts == nil {
					builder.ExposedPorts = map[docker.Port]struct{}{}
				}

				for k, v := range overrides.ExposedPorts {
					if k == dsCmdPort || k == dsEnvPort {
						continue
					}
					builder.ExposedPorts[k] = v
				}
			}
		}
	}

	//instructions have higher value precedence over the runtime overrides
	if instructions != nil {
		log.Debugf("NewImageBuilder: Using new image instructions => %+v", instructions)

		if instructions.Workdir != "" {
			builder.WorkingDir = instructions.Workdir
		}

		if len(instructions.Env) > 0 {
			builder.Env = append(builder.Env, instructions.Env...)
		}

		if builder.ExposedPorts == nil {
			builder.ExposedPorts = map[docker.Port]struct{}{}
		}

		for k, v := range instructions.ExposedPorts {
			builder.ExposedPorts[k] = v
		}

		if len(instructions.Entrypoint) > 0 {
			builder.Entrypoint = instructions.Entrypoint
		}

		if len(instructions.Cmd) > 0 {
			builder.Cmd = instructions.Cmd
		}

		if len(builder.ExposedPorts) > 0 &&
			len(instructions.RemoveExposedPorts) > 0 {
			for k := range instructions.RemoveExposedPorts {
				if _, ok := builder.ExposedPorts[k]; ok {
					delete(builder.ExposedPorts, k)
				}
			}
		}

		if len(builder.Volumes) > 0 &&
			len(instructions.RemoveVolumes) > 0 {
			for k := range instructions.RemoveVolumes {
				if _, ok := builder.Volumes[k]; ok {
					delete(builder.Volumes, k)
				}
			}
		}

		if len(instructions.RemoveEnvs) > 0 &&
			len(builder.Env) > 0 {
			var newEnv []string
			for _, envPair := range builder.Env {
				envParts := strings.SplitN(envPair, "=", 2)
				if len(envParts) > 0 && envParts[0] != "" {
					if _, ok := instructions.RemoveEnvs[envParts[0]]; !ok {
						newEnv = append(newEnv, envPair)
					}
				}
			}

			builder.Env = newEnv
		}
	}

	builder.BuildOptions.OutputStream = &builder.BuildLog

	dataTar := filepath.Join(artifactLocation, "files.tar")
	builder.TarData = fsutil.IsRegularFile(dataTar)
	if builder.TarData {
		builder.HasData = true
	} else {
		dataDir := filepath.Join(artifactLocation, "files")
		builder.HasData = fsutil.IsDir(dataDir)
	}

	return builder, nil
}

// Build creates a new container image
func (b *ImageBuilder) Build() error {
	if err := b.GenerateDockerfile(); err != nil {
		return err
	}

	return b.APIClient.BuildImage(b.BuildOptions)
}

// GenerateDockerfile creates a Dockerfile file
func (b *ImageBuilder) GenerateDockerfile() error {
	return reverse.GenerateFromInfo(b.BuildOptions.ContextDir,
		b.Volumes,
		b.WorkingDir,
		b.Env,
		b.User,
		b.ExposedPorts,
		b.Entrypoint,
		b.Cmd,
		b.HasData,
		b.TarData)
}
