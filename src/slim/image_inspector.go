package main

import (
	"fmt"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

type ImageInspector struct {
	ImageRef            string
	ArtifactLocation    string
	SlimImageRepo       string
	AppArmorProfileName string
	ImageInfo           *docker.Image
	ImageRecordInfo     docker.APIImages
	ApiClient           *docker.Client
}

func NewImageInspector(client *docker.Client, imageRef string, artifactLocation string) (*ImageInspector, error) {
	inspector := &ImageInspector{
		ImageRef:            imageRef,
		SlimImageRepo:       "slim",
		AppArmorProfileName: "apparmor-profile",
		ArtifactLocation:    artifactLocation,
		ApiClient:           client,
	}

	return inspector, nil
}

func (i *ImageInspector) Inspect() error {
	var err error
	i.ImageInfo, err = i.ApiClient.InspectImage(i.ImageRef)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			log.Info("docker-slim: could not find target image")
		}
		return err
	}

	imageList, err := i.ApiClient.ListImages(docker.ListImagesOptions{All: true})
	if err != nil {
		return err
	}

	for _, r := range imageList {
		if r.ID == i.ImageInfo.ID {
			i.ImageRecordInfo = r
			break
		}
	}

	if i.ImageRecordInfo.ID == "" {
		log.Info("docker-slim: could not find target image in the image list")
		return docker.ErrNoSuchImage
	}

	return nil
}

func (i *ImageInspector) processImageName() {
	if len(i.ImageRecordInfo.RepoTags) > 0 {
		if rtInfo := strings.Split(i.ImageRecordInfo.RepoTags[0], ":"); len(rtInfo) > 1 {
			i.SlimImageRepo = fmt.Sprintf("%s.slim", rtInfo[0])
			if nameParts := strings.Split(rtInfo[0], "/"); len(nameParts) > 1 {
				i.AppArmorProfileName = strings.Join(nameParts, "-")
			} else {
				i.AppArmorProfileName = rtInfo[0]
			}
			i.AppArmorProfileName = fmt.Sprintf("%s-apparmor-profile", i.AppArmorProfileName)
		}
	}
}

func (i *ImageInspector) ProcessCollectedData() error {
	i.processImageName()

	fatImageDockerInstructions, err := reverseDockerfileFromHistory(i.ApiClient, i.ImageRef)
	if err != nil {
		return err
	}
	fatImageDockerfileLocation := filepath.Join(i.ArtifactLocation, "Dockerfile.fat")
	err = saveDockerfileData(fatImageDockerfileLocation, fatImageDockerInstructions)
	failOnError(err)

	return nil
}
