package builder

import (
	//"os"
	"bytes"
	"path/filepath"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerfile"
	"github.com/docker-slim/docker-slim/pkg/utils/fsutils"

	//log "github.com/Sirupsen/logrus"
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
	imageOverrides map[string]bool,
	overrides *config.ContainerOverrides) (*ImageBuilder, error) {
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

	builder.BuildOptions.OutputStream = &builder.BuildLog

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
		b.User,
		b.ExposedPorts,
		b.Entrypoint,
		b.Cmd,
		b.HasData)
}
