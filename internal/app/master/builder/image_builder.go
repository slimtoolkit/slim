package builder

import (
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerfile"
	"github.com/docker-slim/docker-slim/pkg/utils/fsutils"

	//log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

// ImageBuilder creates new container images
type ImageBuilder struct {
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
	BuildOptions docker.BuildImageOptions
	APIClient    *docker.Client
}

// NewImageBuilder creates a new ImageBuilder instances
func NewImageBuilder(client *docker.Client,
	imageRepoName string,
	imageInfo *docker.Image,
	artifactLocation string,
	imageOverrides map[string]bool,
	overrides *config.ContainerOverrides) (*ImageBuilder, error) {
	builder := &ImageBuilder{
		RepoName:     imageRepoName,
		ID:           imageInfo.ID,
		Entrypoint:   imageInfo.Config.Entrypoint,
		Cmd:          imageInfo.Config.Cmd,
		WorkingDir:   imageInfo.Config.WorkingDir,
		Env:          imageInfo.Config.Env,
		ExposedPorts: imageInfo.Config.ExposedPorts,
		Volumes:      imageInfo.Config.Volumes,
		OnBuild:      imageInfo.Config.OnBuild,
		User:         imageInfo.Config.User,
		BuildOptions: docker.BuildImageOptions{
			Name:           imageRepoName,
			RmTmpContainer: true,
			ContextDir:     artifactLocation,
			Dockerfile:     "Dockerfile",
			OutputStream:   os.Stdout,
		},
		APIClient: client,
	}

	dataDir := filepath.Join(artifactLocation, "files")
	builder.HasData = fsutils.IsDir(dataDir)

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
		b.ExposedPorts,
		b.Entrypoint,
		b.Cmd,
		b.HasData)
}
