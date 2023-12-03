package image

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/consts"
	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/reverse"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
)

const (
	slimImageRepo          = "slim"
	appArmorProfileName    = "apparmor-profile"
	seccompProfileName     = "seccomp-profile"
	appArmorProfileNamePat = "%s-apparmor-profile"
	seccompProfileNamePat  = "%s-seccomp.json"
	https                  = "https://"
	http                   = "http://"
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
func (i *Inspector) NoImage() (bool, error) {
	//first, do a simple exact match lookup
	ii, err := dockerutil.HasImage(i.APIClient, i.ImageRef)
	if err == nil {
		log.Tracef("image.inspector.NoImage: ImageRef=%v ImageIdentity=%#v", i.ImageRef, ii)
		return false, nil
	}

	if err != dockerutil.ErrNotFound {
		log.Errorf("image.inspector.NoImage: err=%v", err)
		return true, err
	}

	//second, try to find something close enough
	//handle the case where there's no tag in the target image reference
	//and there are no default 'latest' tag
	//this will return/save the first available tag
	if err == dockerutil.ErrNotFound &&
		!strings.Contains(i.ImageRef, ":") {
		//check if there are any tags for the target image
		matches, err := dockerutil.ListImages(i.APIClient, i.ImageRef)
		if err != nil {
			log.Errorf("image.inspector.NoImage: err=%v", err)
			return true, err
		}

		for ref, props := range matches {
			log.Debugf("image.inspector.NoImage: match.ref=%s match.props=%#v", ref, props)
			i.ImageRef = ref
			return false, nil
		}
	}

	return true, nil
}

// Pull tries to download the target image
func (i *Inspector) Pull(showPullLog bool, dockerConfigPath, registryAccount, registrySecret string) error {
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
	authConfig, err = getRegistryCredential(registryAccount, registrySecret, dockerConfigPath, registry)
	if err != nil {
		log.Warnf("image.inspector.Pull: failed to get registry credential for registry=%s with err=%v", registry, err)
		//warn, attempt pull anyway, needs to work for public registries
	}

	if authConfig == nil {
		authConfig = &docker.AuthConfiguration{}
	}

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

func getRegistryCredential(registryAccount, registrySecret, dockerConfigPath, registry string) (cred *docker.AuthConfiguration, err error) {
	if registryAccount != "" && registrySecret != "" {
		cred = &docker.AuthConfiguration{
			Username: registryAccount,
			Password: registrySecret,
		}
		return
	}

	missingAuthConfigErr := fmt.Errorf("could not find an auth config for registry - %s", registry)
	if dockerConfigPath != "" {
		dAuthConfigs, err := docker.NewAuthConfigurationsFromFile(dockerConfigPath)
		if err != nil {
			log.Warnf(
				"image.inspector.Pull: getDockerCredential - failed to acquire local docker config path=%s err=%s",
				dockerConfigPath,
				err.Error(),
			)
			return nil, err
		}
		r, found := dAuthConfigs.Configs[registry]
		if !found {
			return nil, missingAuthConfigErr
		}
		cred = &r
		return cred, nil
	}

	cred, err = docker.NewAuthConfigurationsFromCredsHelpers(registry)
	if err != nil {
		log.Warnf(
			"image.inspector.Pull: failed to acquire local docker credential helpers for %s err=%s",
			registry,
			err.Error(),
		)
		return nil, err
	}

	// could not find a credentials' helper, check auth configs
	if cred == nil {
		dConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
		if err != nil {
			log.Debugf("image.inspector.Pull: getDockerCredential err extracting docker auth configs - %s", err.Error())
			return nil, err
		}
		r, found := dConfigs.Configs[registry]
		if !found {
			return nil, missingAuthConfigErr
		}
		cred = &r
	}

	log.Debugf("loaded registry auth config %+v", cred)
	return cred, nil
}

func extractRegistry(repo string) string {
	var scheme string
	if strings.Contains(repo, https) {
		scheme = https
		repo = strings.TrimPrefix(repo, https)
	}
	if strings.Contains(repo, http) {
		scheme = http
		repo = strings.TrimPrefix(repo, http)
	}
	registry := strings.Split(repo, "/")[0]

	domain := `((?:[a-z\d](?:[a-z\d-]{0,63}[a-z\d])?|\*)\.)+[a-z\d][a-z\d-]{0,63}[a-z\d]`
	ipv6 := `^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$`
	ipv4 := `^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`
	ipv4Port := `([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})\:?([0-9]{1,5})?`
	ipv6Port := `(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`

	if registry == "localhost" || strings.Contains(registry, "localhost:") {
		return scheme + registry
	}

	validDomain := regexp.MustCompile(domain)
	validIpv4 := regexp.MustCompile(ipv4)
	validIpv6 := regexp.MustCompile(ipv6)
	validIpv4WithPort := regexp.MustCompile(ipv4Port)
	validIpv6WithPort := regexp.MustCompile(ipv6Port)

	if validIpv6WithPort.MatchString(registry) {
		return scheme + registry
	}
	if validIpv4WithPort.MatchString(registry) {
		return scheme + registry
	}
	if validIpv6.MatchString(registry) {
		return scheme + registry
	}
	if validIpv4.MatchString(registry) {
		return scheme + registry
	}

	if !validDomain.MatchString(registry) {
		return https + "index.docker.io"
	}
	return scheme + registry
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

	log.Tracef("image.Inspector.Inspect: ImageInfo=%#v", i.ImageInfo)

	imageList, err := i.APIClient.ListImages(docker.ListImagesOptions{All: true})
	if err != nil {
		return err
	}

	log.Tracef("image.Inspector.Inspect: imageList.size=%v", len(imageList))
	for _, r := range imageList {
		log.Tracef("image.Inspector.Inspect: target=%v record=%#v", i.ImageInfo.ID, r)
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
		//try to find the repo/tag that matches the image ref (if it's not an image ID)
		//then pick the first available repo/tag if we can't
		imageName := i.ImageRecordInfo.RepoTags[0]
		for _, current := range i.ImageRecordInfo.RepoTags {
			if strings.HasPrefix(current, i.ImageRef) {
				imageName = current
				break
			}
		}

		if rtInfo := strings.Split(imageName, ":"); len(rtInfo) > 1 {
			if rtInfo[0] == "<none>" {
				rtInfo[0] = strings.TrimLeft(i.ImageRecordInfo.ID, "sha256:")[0:8]
			}
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
	fatImageDockerfileLocation := filepath.Join(i.ArtifactLocation, consts.ReversedDockerfile)
	err = reverse.SaveDockerfileData(fatImageDockerfileLocation, i.DockerfileInfo.Lines)
	errutil.FailOn(err)
	//save the reversed Dockerfile with the old name too (tmp compat)
	fatImageDockerfileLocationOld := filepath.Join(i.ArtifactLocation, consts.ReversedDockerfileOldName)
	err = reverse.SaveDockerfileData(fatImageDockerfileLocationOld, i.DockerfileInfo.Lines)
	errutil.WarnOn(err)

	return nil
}

// ShowFatImageDockerInstructions prints the original target image Dockerfile instructions
func (i *Inspector) ShowFatImageDockerInstructions() {
	if i.DockerfileInfo != nil && i.DockerfileInfo.Lines != nil {
		fmt.Println("slim: Fat image - Dockerfile instructures: start ====")
		fmt.Println(strings.Join(i.DockerfileInfo.Lines, "\n"))
		fmt.Println("slim: Fat image - Dockerfile instructures: end ======")
	}
}
