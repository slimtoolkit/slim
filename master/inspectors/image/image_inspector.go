package image

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudimmunity/docker-slim/master/docker/dockerfile"
	"github.com/cloudimmunity/docker-slim/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

type Inspector struct {
	ImageRef            string
	ArtifactLocation    string
	SlimImageRepo       string
	AppArmorProfileName string
	SeccompProfileName  string
	ImageInfo           *docker.Image
	ImageRecordInfo     docker.APIImages
	ApiClient           *docker.Client
}

func NewInspector(client *docker.Client, imageRef string /*, artifactLocation string*/) (*Inspector, error) {
	inspector := &Inspector{
		ImageRef:            imageRef,
		SlimImageRepo:       "slim",
		AppArmorProfileName: "apparmor-profile",
		SeccompProfileName:  "seccomp-profile",
		//ArtifactLocation:    artifactLocation,
		ApiClient: client,
	}

	return inspector, nil
}

func (i *Inspector) Inspect() error {
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

func (i *Inspector) processImageName() {
	if len(i.ImageRecordInfo.RepoTags) > 0 {
		if rtInfo := strings.Split(i.ImageRecordInfo.RepoTags[0], ":"); len(rtInfo) > 1 {
			i.SlimImageRepo = fmt.Sprintf("%s.slim", rtInfo[0])
			if nameParts := strings.Split(rtInfo[0], "/"); len(nameParts) > 1 {
				i.AppArmorProfileName = strings.Join(nameParts, "-")
				i.SeccompProfileName = strings.Join(nameParts, "-")
			} else {
				i.AppArmorProfileName = rtInfo[0]
				i.SeccompProfileName = rtInfo[0]
			}
			i.AppArmorProfileName = fmt.Sprintf("%s-apparmor-profile", i.AppArmorProfileName)
			i.SeccompProfileName = fmt.Sprintf("%s-seccomp.json", i.SeccompProfileName)
		}
	}
}

func (i *Inspector) ProcessCollectedData() error {
	i.processImageName()

	fatImageDockerInstructions, err := dockerfile.ReverseDockerfileFromHistory(i.ApiClient, i.ImageRef)
	if err != nil {
		return err
	}
	fatImageDockerfileLocation := filepath.Join(i.ArtifactLocation, "Dockerfile.fat")
	err = dockerfile.SaveDockerfileData(fatImageDockerfileLocation, fatImageDockerInstructions)
	utils.FailOn(err)

	return nil
}
