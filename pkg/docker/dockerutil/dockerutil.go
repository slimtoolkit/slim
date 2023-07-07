package dockerutil

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/archive"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
)

var (
	ErrBadParam = errors.New("bad parameter")
	ErrNotFound = errors.New("not found")
)

const (
	volumeMountPat       = "%s:/data"
	volumeBasePath       = "/data"
	emptyImageName       = "docker-slim-empty-image:latest"
	emptyImageDockerfile = "FROM scratch\nCMD\n"
)

type BasicImageProps struct {
	ID      string
	Size    int64
	Created int64
}

type ImageIdentity struct {
	ID           string
	ShortTags    []string
	RepoTags     []string
	ShortDigests []string
	RepoDigests  []string
}

func APIImagesToIdentity(info *dockerapi.APIImages) *ImageIdentity {
	imageInfo := &dockerapi.Image{
		ID:          info.ID,
		RepoTags:    info.RepoTags,
		RepoDigests: info.RepoDigests,
	}

	return ImageToIdentity(imageInfo)
}

func ImageToIdentity(info *dockerapi.Image) *ImageIdentity {
	result := &ImageIdentity{
		ID:          info.ID,
		RepoTags:    info.RepoTags,
		RepoDigests: info.RepoDigests,
	}

	for _, tag := range result.RepoTags {
		parts := strings.Split(tag, ":")
		if len(parts) == 2 {
			result.ShortTags = append(result.ShortTags, parts[1])
		}
	}

	for _, digest := range result.RepoDigests {
		parts := strings.Split(digest, "@")
		if len(parts) == 2 {
			result.ShortDigests = append(result.ShortDigests, parts[1])
		}
	}

	return result
}

func CleanImageID(id string) string {
	if strings.HasPrefix(id, "sha256:") {
		id = strings.TrimPrefix(id, "sha256:")
	}

	return id
}

func HasEmptyImage(dclient *dockerapi.Client) error {
	_, err := HasImage(dclient, emptyImageName)
	return err
}

func HasImage(dclient *dockerapi.Client, imageRef string) (*ImageIdentity, error) {
	//NOTES:
	//ListImages doesn't filter by image ID (must use ImageInspect instead)
	//Check images by name:tag, full or partial image ID or name@digest
	if imageRef == "" || imageRef == "." || imageRef == ".." {
		return nil, ErrBadParam
	}

	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.HasImage(%s): dockerapi.NewClient() error = %v", imageRef, err)
			return nil, err
		}
	}

	imageInfo, err := dclient.InspectImage(imageRef)
	if err != nil {
		if err == dockerapi.ErrNoSuchImage {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return ImageToIdentity(imageInfo), nil
}

func ListImages(dclient *dockerapi.Client, imageNameFilter string) (map[string]BasicImageProps, error) {
	// python <- exact match only
	// py* <- all image names starting with 'py' (no/default namespace)
	// dslimexamples/* <- all image names in the 'dslimexamples' namespace
	// dslimexamples/ser* <- all image names starting with 'ser' in the 'dslimexamples' namespace
	// dslimexamples/*python* <- all image names with 'python' in them in the 'dslimexamples' namespace
	// */*python* <- all image names with 'python' in them in all namesapces (except the default namespace)
	// */*alpine <- all image names ending with 'alpine' in all namesapces (except the default namespace)
	// * <- all image names with no/default namespace. note that no images with namespaces will be returned
	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.ListImages(%s): dockerapi.NewClient() error = %v", imageNameFilter, err)
			return nil, err
		}
	}

	listOptions := dockerapi.ListImagesOptions{
		All: false,
	}

	if imageNameFilter != "" {
		listOptions.Filters = map[string][]string{
			"reference": {imageNameFilter},
		}
	}

	imageList, err := dclient.ListImages(listOptions)
	if err != nil {
		log.Errorf("dockerutil.ListImages(%s): dockerapi.ListImages() error = %v", imageNameFilter, err)
		return nil, err
	}

	log.Debugf("dockerutil.ListImages(%s): matching images - %+v", imageNameFilter, imageList)

	images := map[string]BasicImageProps{}
	for _, imageInfo := range imageList {
		for _, repo := range imageInfo.RepoTags {
			info := BasicImageProps{
				ID:      strings.TrimPrefix(imageInfo.ID, "sha256:"),
				Size:    imageInfo.Size,
				Created: imageInfo.Created,
			}

			if repo == "<none>:<none>" {
				repo = strings.TrimPrefix(imageInfo.ID, "sha256:")
				images[repo] = info
				break
			}

			images[repo] = info
		}
	}

	return images, nil
}

func BuildEmptyImage(dclient *dockerapi.Client) error {
	//TODO: use the 'internal' build engine that doesn't need Docker
	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.BuildEmptyImage: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	var input bytes.Buffer
	tw := tar.NewWriter(&input)
	header := tar.Header{
		Name: "Dockerfile",
		Size: int64(len(emptyImageDockerfile)),
	}

	if err := tw.WriteHeader(&header); err != nil {
		return err
	}

	if _, err := tw.Write([]byte(emptyImageDockerfile)); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	var output bytes.Buffer
	buildOptions := dockerapi.BuildImageOptions{
		Name:                emptyImageName,
		InputStream:         &input,
		OutputStream:        &output,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
	}
	if err := dclient.BuildImage(buildOptions); err != nil {
		log.Errorf("dockerutil.BuildEmptyImage: dockerapi.BuildImage() error = %v / output: %s", err, output.String())
		return err
	}

	return nil
}

func SaveImage(dclient *dockerapi.Client, imageRef, local string, extract, removeOrig bool) error {
	if local == "" {
		return ErrBadParam
	}

	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.SaveImage: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	imageRef = CleanImageID(imageRef)

	//todo: 'pull' the image if it's not available locally yet
	//note: HasImage() doesn't work with image IDs

	dir := fsutil.FileDir(local)
	if !fsutil.DirExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	dfile, err := os.Create(local)
	if err != nil {
		return err
	}

	options := dockerapi.ExportImageOptions{
		Name:              imageRef,
		OutputStream:      dfile,
		InactivityTimeout: 20 * time.Second,
	}

	err = dclient.ExportImage(options)
	if err != nil {
		log.Errorf("dockerutil.SaveImage: dclient.ExportImage() error = %v", err)
		dfile.Close()
		return err
	}

	dfile.Close()

	if extract {
		dstDir := filepath.Dir(local)
		arc := archive.NewDefaultArchiver()

		afile, err := os.Open(local)
		if err != nil {
			log.Errorf("dockerutil.SaveImage: os.Open error - %v", err)
			return err
		}

		tarOptions := &archive.TarOptions{
			NoLchown: true,
			//UIDMaps:  arc.IDMapping.UIDs(),
			//GIDMaps:  arc.IDMapping.GIDs(),

		}

		tarOptions.IDMap.UIDMaps = arc.IDMapping.UIDs()
		tarOptions.IDMap.GIDMaps = arc.IDMapping.GIDs()

		err = arc.Untar(afile, dstDir, tarOptions)
		if err != nil {
			log.Errorf("dockerutil.SaveImage: error unpacking tar - %v", err)
			afile.Close()
			return err
		}

		afile.Close()

		if removeOrig {
			os.Remove(local)
		}
	}

	return nil
}

func HasVolume(dclient *dockerapi.Client, name string) error {
	if name == "" {
		return ErrBadParam
	}

	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.HasVolume: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	listOptions := dockerapi.ListVolumesOptions{
		Filters: map[string][]string{"name": {name}},
	}

	volumes, err := dclient.ListVolumes(listOptions)
	if err != nil {
		log.Errorf("dockerutil.HasVolume: dclient.ListVolumes() error = %v", err)
		return err
	}

	if len(volumes) == 0 {
		log.Debugf("dockerutil.HasVolume: volume not found - %v", name)
		return ErrNotFound
	}

	for _, info := range volumes {
		if info.Name == name {
			return nil
		}
	}

	return ErrNotFound
}

func DeleteVolume(dclient *dockerapi.Client, name string) error {
	if name == "" {
		return ErrBadParam
	}

	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.DeleteVolume: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	if err := HasVolume(dclient, name); err == nil {
		removeOptions := dockerapi.RemoveVolumeOptions{
			Name:  name,
			Force: true,
		}

		//ok to call remove even if the volume isn't there
		err = dclient.RemoveVolumeWithOptions(removeOptions)
		if err != nil {
			fmt.Printf("dockerutil.DeleteVolume: dclient.RemoveVolumeWithOptions() error = %v\n", err)
			return err
		}
	}

	return nil
}

func CopyToVolume(
	dclient *dockerapi.Client,
	volumeName string,
	source string,
	dstRootDir string,
	dstTargetDir string) error {
	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.CopyToVolume: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	volumeBinds := []string{fmt.Sprintf(volumeMountPat, volumeName)}

	containerOptions := dockerapi.CreateContainerOptions{
		Name: volumeName, //todo: might be good to make it unique (to support concurrent copy op)
		Config: &dockerapi.Config{
			Image:  emptyImageName,
			Labels: map[string]string{"owner": "docker-slim"},
		},
		HostConfig: &dockerapi.HostConfig{
			Binds: volumeBinds,
		},
	}

	containerInfo, err := dclient.CreateContainer(containerOptions)
	if err != nil {
		log.Errorf("dockerutil.CopyToVolume: dclient.CreateContainer() error = %v", err)
		return err
	}

	containerID := containerInfo.ID
	log.Debugf("dockerutil.CopyToVolume: containerID - %v", containerID)

	rmContainer := func() {
		removeOptions := dockerapi.RemoveContainerOptions{
			ID:    containerID,
			Force: true,
		}

		err = dclient.RemoveContainer(removeOptions)
		if err != nil {
			fmt.Printf("dockerutil.CopyToVolume: dclient.RemoveContainer() error = %v\n", err)
		}
	}

	cleanSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		log.Errorf("dockerutil.CopyToVolume: filepath.EvalSymlinks(%s) error = %v", source, err)
		rmContainer()
		return err
	}

	if fsutil.IsSymlink(cleanSource) {
		log.Errorf("dockerutil.CopyToVolume: source is a symlink = %s", cleanSource)
		rmContainer()
		return fmt.Errorf("source is symlink")
	}

	tarData, err := archive.Tar(cleanSource, archive.Uncompressed)
	if err != nil {
		log.Errorf("dockerutil.CopyToVolume: archive.Tar() error = %v", err)
		rmContainer()
		return err
	}

	targetPath := volumeBasePath
	if dstRootDir != "" {
		dirData, err := GenStateDirsTar(dstRootDir, dstTargetDir)
		if err != nil {
			log.Errorf("dockerutil.CopyToVolume: GenStateDirsTar() error = %v", err)
			rmContainer()
			return err
		}

		dirUploadOptions := dockerapi.UploadToContainerOptions{
			InputStream: dirData,
			Path:        targetPath,
		}

		err = dclient.UploadToContainer(containerID, dirUploadOptions)
		if err != nil {
			log.Errorf("dockerutil.CopyToVolume: copy dirs - dclient.UploadToContainer() error = %v", err)
			rmContainer()
			return err
		}

		targetPath = filepath.Join(volumeBasePath, dstRootDir, dstTargetDir)
	}

	uploadOptions := dockerapi.UploadToContainerOptions{
		InputStream: tarData,
		Path:        targetPath,
	}

	err = dclient.UploadToContainer(containerID, uploadOptions)
	if err != nil {
		log.Errorf("dockerutil.CopyToVolume: dclient.UploadToContainer() error = %v", err)
		tarData.Close()
		rmContainer()
		return err
	}

	tarData.Close()
	rmContainer()

	return nil
}

func GenStateDirsTar(rootDir, stateDir string) (io.Reader, error) {
	if rootDir == "" || stateDir == "" {
		return nil, ErrBadParam
	}

	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	baseDirHdr := tar.Header{
		Typeflag: tar.TypeDir,
		Name:     fmt.Sprintf("%s/", rootDir),
		Mode:     16877,
	}

	if err := tw.WriteHeader(&baseDirHdr); err != nil {
		log.Errorf("dockerutil.GenStateDirsTar: error writing base dir header to archive - %v", err)
		return nil, err
	}

	stateDirHdr := tar.Header{
		Typeflag: tar.TypeDir,
		Name:     fmt.Sprintf("%s/%s/", rootDir, stateDir),
		Mode:     16877,
	}

	if err := tw.WriteHeader(&stateDirHdr); err != nil {
		log.Errorf("dockerutil.GenStateDirsTar: error writing state dir header to archive - %v", err)
		return nil, err
	}

	if err := tw.Close(); err != nil {
		log.Errorf("dockerutil.GenStateDirsTar: error closing archive - %v", err)
		return nil, err
	}

	return &b, nil
}

func CreateVolumeWithData(
	dclient *dockerapi.Client,
	source string,
	name string,
	labels map[string]string) error {
	if name == "" {
		return ErrBadParam
	}

	if source != "" {
		if _, err := os.Stat(source); err != nil {
			log.Errorf("dockerutil.CreateVolumeWithData: bad source (%v) = %v", source, err)
			return err
		}
	}

	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.CreateVolumeWithData: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	volumeOptions := dockerapi.CreateVolumeOptions{
		Name:   name,
		Labels: labels,
	}

	volumeInfo, err := dclient.CreateVolume(volumeOptions)
	if err != nil {
		log.Errorf("dockerutil.CreateVolumeWithData: dclient.CreateVolume() error = %v", err)
		return err
	}

	log.Debugf("dockerutil.CreateVolumeWithData: volumeInfo = %+v", volumeInfo)

	if source != "" {
		return CopyToVolume(dclient, name, source, "", "")
	}

	return nil
}

func CopyFromContainer(dclient *dockerapi.Client, containerID, remote, local string, extract, removeOrig bool) error {
	if containerID == "" || remote == "" || local == "" {
		return ErrBadParam
	}

	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.CopyFromContainer: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	dfile, err := os.Create(local)
	if err != nil {
		return err
	}

	downloadOptions := dockerapi.DownloadFromContainerOptions{
		Path:              remote,
		OutputStream:      dfile,
		InactivityTimeout: 20 * time.Second,
	}

	err = dclient.DownloadFromContainer(containerID, downloadOptions)
	if err != nil {
		log.Errorf("dockerutil.CopyFromContainer: dclient.DownloadFromContainer() error = %v", err)
		dfile.Close()
		return err
	}

	dfile.Close()

	if extract {
		dstDir := filepath.Dir(local)
		arc := archive.NewDefaultArchiver()

		afile, err := os.Open(local)
		if err != nil {
			log.Errorf("dockerutil.CopyFromContainer: os.Open error - %v", err)
			return err
		}

		tarOptions := &archive.TarOptions{
			NoLchown: true,
			//UIDMaps:  arc.IDMapping.UIDs(),
			//GIDMaps:  arc.IDMapping.GIDs(),
		}

		tarOptions.IDMap.UIDMaps = arc.IDMapping.UIDs()
		tarOptions.IDMap.GIDMaps = arc.IDMapping.GIDs()

		err = arc.Untar(afile, dstDir, tarOptions)
		if err != nil {
			log.Errorf("dockerutil.CopyFromContainer: error unpacking tar - %v", err)
			afile.Close()
			return err
		}

		afile.Close()

		if removeOrig {
			os.Remove(local)
		}
	}

	return nil
}

func PrepareContainerDataArchive(fullPath, newName, removePrefix string, removeOrig bool) error {
	if fullPath == "" || newName == "" || removePrefix == "" {
		return ErrBadParam
	}

	dirName := filepath.Dir(fullPath)
	dstPath := filepath.Join(dirName, newName)

	inFile, err := os.Open(fullPath)
	if err != nil {
		log.Errorf("dockerutil.PrepareContainerDataArchive: os.Open(%s) error - %v", fullPath, err)
		return err
	}

	outFile, err := os.Create(dstPath)
	if err != nil {
		log.Errorf("dockerutil.PrepareContainerDataArchive: os.Open(%s) error - %v", dstPath, err)
		inFile.Close()
		return err
	}

	tw := tar.NewWriter(outFile)
	tr := tar.NewReader(inFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Errorf("dockerutil.PrepareContainerDataArchive: error reading archive(%v) - %v", fullPath, err)
			inFile.Close()
			return err
		}

		if hdr == nil || hdr.Name == "" {
			log.Debugf("dockerutil.PrepareContainerDataArchive: ignoring bad tar header")
			continue
		}

		if hdr.Name == removePrefix {
			log.Debugf("dockerutil.PrepareContainerDataArchive: ignoring tar object: %v", hdr.Name)
			continue
		}

		if hdr.Name != "" && strings.HasPrefix(hdr.Name, removePrefix) {
			hdr.Name = strings.TrimPrefix(hdr.Name, removePrefix)
		}

		if err := tw.WriteHeader(hdr); err != nil {
			log.Errorf("dockerutil.PrepareContainerDataArchive: error writing header to archive(%v) - %v", dstPath, err)
			inFile.Close()
			outFile.Close()
			return err
		}

		if _, err := io.Copy(tw, tr); err != nil {
			log.Errorf("dockerutil.PrepareContainerDataArchive: error copying data to archive(%v) - %v", dstPath, err)
			inFile.Close()
			outFile.Close()
			return err
		}
	}

	if err := tw.Close(); err != nil {
		log.Errorf("dockerutil.PrepareContainerDataArchive: error closing archive(%v) - %v", dstPath, err)
	}

	outFile.Close()
	inFile.Close()

	if removeOrig {
		os.Remove(fullPath)
	}

	return nil
}

func ListNetworks(dclient *dockerapi.Client, nameFilter string) ([]string, error) {
	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.ListNetworks(%s): dockerapi.NewClient() error = %v", nameFilter, err)
			return nil, err
		}
	}

	filter := dockerapi.NetworkFilterOpts{
		"name": map[string]bool{
			nameFilter: true,
		},
	}

	networkList, err := dclient.FilteredListNetworks(filter)
	if err != nil {
		log.Errorf("dockerutil.ListNetworks(%s): dockerapi.FilteredListNetworks() error = %v", nameFilter, err)
		return nil, err
	}

	var names []string
	for _, networkInfo := range networkList {
		names = append(names, networkInfo.Name)
	}

	return names, nil
}

func ListVolumes(dclient *dockerapi.Client, nameFilter string) ([]string, error) {
	var err error
	if dclient == nil {
		unixSocketAddr := dockerclient.GetUnixSocketAddr()
		if unixSocketAddr == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		dclient, err = dockerapi.NewClient(unixSocketAddr)
		if err != nil {
			log.Errorf("dockerutil.ListVolumes(%s): dockerapi.NewClient() error = %v", nameFilter, err)
			return nil, err
		}
	}

	listOptions := dockerapi.ListVolumesOptions{
		Filters: map[string][]string{
			"name": {nameFilter},
		},
	}

	volumeList, err := dclient.ListVolumes(listOptions)
	if err != nil {
		log.Errorf("dockerutil.ListVolumes(%s): dockerapi.ListVolumes() error = %v", nameFilter, err)
		return nil, err
	}

	var names []string
	for _, volumeInfo := range volumeList {
		names = append(names, volumeInfo.Name)
	}

	return names, nil
}
