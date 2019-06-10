package image

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerfile"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

const (
	slimImageRepo          = "slim"
	appArmorProfileName    = "apparmor-profile"
	seccompProfileName     = "seccomp-profile"
	fatDockerfileName      = "Dockerfile.fat"
	appArmorProfileNamePat = "%s-apparmor-profile"
	seccompProfileNamePat  = "%s-seccomp.json"
)

// Inspector is a container image inspector
type Inspector struct {
	ImageRef            string
	ArtifactLocation    string
	SlimImageRepo       string
	AppArmorProfileName string
	SeccompProfileName  string
	ImageInfo           *docker.Image
	ImageRecordInfo     docker.APIImages
	APIClient           *docker.Client
	//fatImageDockerInstructions []string
	DockerfileInfo *dockerfile.Info
}

// NewInspector creates a new container image inspector
func NewInspector(client *docker.Client, imageRef string /*, artifactLocation string*/) (*Inspector, error) {
	inspector := &Inspector{
		ImageRef:            imageRef,
		SlimImageRepo:       slimImageRepo,
		AppArmorProfileName: appArmorProfileName,
		SeccompProfileName:  seccompProfileName,
		//ArtifactLocation:    artifactLocation,
		APIClient: client,
	}

	return inspector, nil
}

// NoImage returns true if the target image doesn't exist
func (i *Inspector) NoImage() bool {
	_, err := i.APIClient.InspectImage(i.ImageRef)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			return true
		}
	}

	return false
}

// Inspect starts the target image inspection
func (i *Inspector) Inspect() error {
	var err error
	i.ImageInfo, err = i.APIClient.InspectImage(i.ImageRef)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			log.Info("could not find target image")
		}
		return err
	}

	imageList, err := i.APIClient.ListImages(docker.ListImagesOptions{All: true})
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
		log.Info("could not find target image in the image list")
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
			i.AppArmorProfileName = fmt.Sprintf(appArmorProfileNamePat, i.AppArmorProfileName)
			i.SeccompProfileName = fmt.Sprintf(seccompProfileNamePat, i.SeccompProfileName)
		}
	}
}

// ProcessCollectedData performs post-processing on the collected image data
func (i *Inspector) ProcessCollectedData() error {
	i.processImageName()

	var err error
	i.DockerfileInfo, err = dockerfile.ReverseDockerfileFromHistory(i.APIClient, i.ImageRef)
	if err != nil {
		return err
	}
	fatImageDockerfileLocation := filepath.Join(i.ArtifactLocation, fatDockerfileName)
	err = dockerfile.SaveDockerfileData(fatImageDockerfileLocation, i.DockerfileInfo.Lines)
	errutil.FailOn(err)

	return nil
}

// ShowFatImageDockerInstructions prints the original target image Dockerfile instructions
func (i *Inspector) ShowFatImageDockerInstructions() {
	if i.DockerfileInfo != nil && i.DockerfileInfo.Lines != nil {
		fmt.Println("docker-slim: Fat image - Dockerfile instructures: start ====")
		fmt.Println(strings.Join(i.DockerfileInfo.Lines, "\n"))
		fmt.Println("docker-slim: Fat image - Dockerfile instructures: end ======")
	}
}
