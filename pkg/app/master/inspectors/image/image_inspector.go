package image

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/reverse"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
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
	DockerfileInfo *reverse.Dockerfile
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
	_, err := dockerutil.HasImage(i.APIClient, i.ImageRef)
	if err == nil {
		return false
	}

	if err != dockerutil.ErrNotFound {
		log.Debugf("image.inspector.NoImage: err=%v", err)
	}

	if err == dockerutil.ErrNotFound &&
		!strings.Contains(i.ImageRef, ":") {
		//check if there are any tags for the target image
		matches, err := dockerutil.ListImages(i.APIClient, i.ImageRef)
		if err != nil {
			log.Debugf("image.inspector.NoImage: err=%v", err)
			return true
		}

		for ref := range matches {
			i.ImageRef = ref
			return false
		}
	}

	return true
}

// Pull tries to download the target image
func (i *Inspector) Pull(showPullLog bool, username, password string) error {
	var pullLog bytes.Buffer
	var repo string
	var tag string
	if strings.Contains(i.ImageRef, ":") {
		parts := strings.SplitN(i.ImageRef, ":", 2)
		repo = parts[0]
		tag = parts[1]
	} else {
		repo = i.ImageRef
		tag = "latest"
	}

	input := docker.PullImageOptions{
		Repository: repo,
		Tag:        tag,
	}

	if showPullLog {
		input.OutputStream = &pullLog
	}

	var err error
	var authConfig *docker.AuthConfiguration
	registry := extractRegistry(repo)
	if username != "" && password != "" {
		authConfig = &docker.AuthConfiguration{Username: username, Password: password}
	} else {
		authConfig, err = docker.NewAuthConfigurationsFromCredsHelpers(registry)
		if err != nil {
			log.Debugf(
				"image.inspector.Pull: failed to acquire local docker credentials for %s err=%s",
				registry,
				err.Error(),
			)
		}
	}

	// could not find a credentials' helper, check auth configs
	if authConfig == nil {
		dConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
		if err != nil {
			log.Debugf("err extracting docker auth configs - %s", err.Error())
			return err
		}
		r, found := dConfigs.Configs[registry]
		if !found {
			return errors.New("could not find an auth config for registry")
		}
		authConfig = &r
	}
	log.Debugf("loaded registry auth config %+v", authConfig)

	err = i.APIClient.PullImage(input, *authConfig)
	if err != nil {
		log.Debugf("image.inspector.Pull: client.PullImage err=%v", err)
		return err
	}

	if showPullLog {
		fmt.Printf("pull logs ====================\n")
		fmt.Println(pullLog.String())
		fmt.Printf("end of pull logs =============\n")
	}

	return nil
}

func extractRegistry(repo string) string {
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")

	registry := strings.Split(repo, "/")[0]
	re := `^(www\.)?[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,5}(:[0-9]{1,5})?(\/.*)?$`
	var validDomain = regexp.MustCompile(re)

	// when target does not contain registry domain, assume dockerhub
	if !validDomain.MatchString(repo) {
		registry = "index.docker.io"
	}
	return registry
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
	i.DockerfileInfo, err = reverse.DockerfileFromHistory(i.APIClient, i.ImageRef)
	if err != nil {
		return err
	}
	fatImageDockerfileLocation := filepath.Join(i.ArtifactLocation, fatDockerfileName)
	err = reverse.SaveDockerfileData(fatImageDockerfileLocation, i.DockerfileInfo.Lines)
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
