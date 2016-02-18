package builder

import (
	"os"

	"github.com/cloudimmunity/docker-slim/master/config"
	"github.com/cloudimmunity/docker-slim/master/docker/dockerfile"

	//log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

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
	BuildOptions docker.BuildImageOptions
	ApiClient    *docker.Client
}

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
		ApiClient: client,
	}

	return builder, nil
}

func (b *ImageBuilder) Build() error {
	if err := b.GenerateDockerfile(); err != nil {
		return err
	}

	return b.ApiClient.BuildImage(b.BuildOptions)
}

func (b *ImageBuilder) GenerateDockerfile() error {
	return dockerfile.GenerateFromInfo(b.BuildOptions.ContextDir,
		b.WorkingDir,
		b.Env,
		b.ExposedPorts,
		b.Entrypoint,
		b.Cmd)
}
