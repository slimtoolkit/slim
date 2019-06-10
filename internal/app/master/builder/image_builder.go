package builder

import (
	"bytes"
	"path/filepath"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerfile"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

// ImageBuilder creates new container images
type ImageBuilder struct {
	ShowBuildLogs bool
	RepoName      string
	ID            string
	Entrypoint    []string
	Cmd           []string
	WorkingDir    string
	Env           []string
	ExposedPorts  map[docker.Port]struct{}
	Volumes       map[string]struct{}
	OnBuild       []string
	User          string
	HasData       bool
	BuildOptions  docker.BuildImageOptions
	APIClient     *docker.Client
	BuildLog      bytes.Buffer
}

// NewImageBuilder creates a new ImageBuilder instances
func NewImageBuilder(client *docker.Client,
	imageRepoName string,
	imageInfo *docker.Image,
	artifactLocation string,
	showBuildLogs bool,
	overrideSelectors map[string]bool,
	overrides *config.ContainerOverrides,
	instructions *config.ImageNewInstructions) (*ImageBuilder, error) {
	builder := &ImageBuilder{
		ShowBuildLogs: showBuildLogs,
		RepoName:      imageRepoName,
		ID:            imageInfo.ID,
		Entrypoint:    imageInfo.Config.Entrypoint,
		Cmd:           imageInfo.Config.Cmd,
		WorkingDir:    imageInfo.Config.WorkingDir,
		Env:           imageInfo.Config.Env,
		ExposedPorts:  imageInfo.Config.ExposedPorts,
		Volumes:       imageInfo.Config.Volumes,
		OnBuild:       imageInfo.Config.OnBuild,
		User:          imageInfo.Config.User,
		BuildOptions: docker.BuildImageOptions{
			Name:           imageRepoName,
			RmTmpContainer: true,
			ContextDir:     artifactLocation,
			Dockerfile:     "Dockerfile",
			//SuppressOutput: true,
			//OutputStream:   os.Stdout,
		},
		APIClient: client,
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
	}

	builder.BuildOptions.OutputStream = &builder.BuildLog

	dataDir := filepath.Join(artifactLocation, "files")
	builder.HasData = fsutil.IsDir(dataDir)

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
	return dockerfile.GenerateFromInfo(b.BuildOptions.ContextDir,
		b.WorkingDir,
		b.Env,
		b.User,
		b.ExposedPorts,
		b.Entrypoint,
		b.Cmd,
		b.HasData)
}
