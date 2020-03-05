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
)

var (
	ErrBadParam = errors.New("bad parameter")
	ErrNotFound = errors.New("not found")
)

const (
	dockerHost           = "unix:///var/run/docker.sock"
	volumeMountPat       = "%s:/data"
	volumeBasePath       = "/data"
	emptyImageName       = "docker-slim-empty-image"
	emptyImageDockerfile = "FROM scratch\nCMD\n"
)

func HasEmptyImage(dclient *dockerapi.Client) error {
	return HasImage(dclient, emptyImageName)
}

func HasImage(dclient *dockerapi.Client, imageRef string) error {
	if imageRef == "" || imageRef == "." || imageRef == ".." {
		return ErrBadParam
	}

	var err error
	if dclient == nil {
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("HasImage(%s): dockerapi.NewClient() error = %v", imageRef, err)
			return err
		}
	}

	listOptions := dockerapi.ListImagesOptions{
		Filter: imageRef,
		All:    false,
	}

	imageList, err := dclient.ListImages(listOptions)
	if err != nil {
		log.Errorf("HasImage(%s): dockerapi.ListImages() error = %v", imageRef, err)
		return err
	}

	log.Debugf("HasImage(%s): matching images - %+v", imageRef, imageList)

	if len(imageList) == 0 {
		log.Debugf("HasImage(%s): empty image not found", imageRef)
		return ErrNotFound
	}

	return nil
}

func BuildEmptyImage(dclient *dockerapi.Client) error {
	var err error
	if dclient == nil {
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("BuildEmptyImage: dockerapi.NewClient() error = %v", err)
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
		log.Errorf("CreateEmptyImage: dockerapi.BuildImage() error = %v", err)
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
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("SaveImage: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	//todo: 'pull' the image if it's not available locally yet
	if err := HasImage(dclient, imageRef); err != nil {
		return err
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
		log.Errorf("SaveImage: dclient.ExportImage() error = %v", err)
		dfile.Close()
		return err
	}

	dfile.Close()

	if extract {
		dstDir := filepath.Dir(local)
		arc := archive.NewDefaultArchiver()

		afile, err := os.Open(local)
		if err != nil {
			log.Errorf("SaveImage: os.Open error - %v", err)
			return err
		}

		tarOptions := &archive.TarOptions{
			NoLchown: true,
			UIDMaps:  arc.IDMapping.UIDs(),
			GIDMaps:  arc.IDMapping.GIDs(),
		}
		err = arc.Untar(afile, dstDir, tarOptions)
		if err != nil {
			log.Errorf("SaveImage: error unpacking tar - %v", err)
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
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("HasVolume: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	listOptions := dockerapi.ListVolumesOptions{
		Filters: map[string][]string{"name": {name}},
	}

	volumes, err := dclient.ListVolumes(listOptions)
	if err != nil {
		log.Errorf("HasVolume: dclient.ListVolumes() error = %v", err)
		return err
	}

	if len(volumes) == 0 {
		log.Debugf("HasVolume: volume not found - %v", name)
		return ErrNotFound
	}

	return nil
}

func DeleteVolume(dclient *dockerapi.Client, name string) error {
	if name == "" {
		return ErrBadParam
	}

	var err error
	if dclient == nil {
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("DeleteVolume: dockerapi.NewClient() error = %v", err)
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
			fmt.Printf("DeleteVolume: dclient.RemoveVolumeWithOptions() error = %v\n", err)
			return err
		}
	}

	return nil
}

func CopyToVolume(dclient *dockerapi.Client, volumeName, source, dstRootDir, dstTargetDir string) error {
	var err error
	if dclient == nil {
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("CopyToVolume: dockerapi.NewClient() error = %v", err)
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
		log.Errorf("CopyToVolume: dclient.CreateContainer() error = %v", err)
		return err
	}

	containerID := containerInfo.ID
	log.Debugf("CopyToVolume: containerID - %v", containerID)

	rmContainer := func() {
		removeOptions := dockerapi.RemoveContainerOptions{
			ID:    containerID,
			Force: true,
		}

		err = dclient.RemoveContainer(removeOptions)
		if err != nil {
			fmt.Printf("CopyToVolume: dclient.RemoveContainer() error = %v\n", err)
		}
	}

	tarData, err := archive.Tar(source, archive.Uncompressed)
	if err != nil {
		log.Errorf("CopyToVolume: archive.Tar() error = %v", err)
		rmContainer()
		return err
	}

	targetPath := volumeBasePath
	if dstRootDir != "" {
		dirData, err := GenStateDirsTar(dstRootDir, dstTargetDir)
		if err != nil {
			log.Errorf("CopyToVolume: GenStateDirsTar() error = %v", err)
			rmContainer()
			return err
		}

		dirUploadOptions := dockerapi.UploadToContainerOptions{
			InputStream: dirData,
			Path:        targetPath,
		}

		err = dclient.UploadToContainer(containerID, dirUploadOptions)
		if err != nil {
			log.Errorf("CopyToVolume: copy dirs - dclient.UploadToContainer() error = %v", err)
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
		log.Errorf("CopyToVolume: dclient.UploadToContainer() error = %v", err)
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
		log.Errorf("error writing base dir header to archive - %v", err)
		return nil, err
	}

	stateDirHdr := tar.Header{
		Typeflag: tar.TypeDir,
		Name:     fmt.Sprintf("%s/%s/", rootDir, stateDir),
		Mode:     16877,
	}

	if err := tw.WriteHeader(&stateDirHdr); err != nil {
		log.Errorf("error writing state dir header to archive - %v", err)
		return nil, err
	}

	if err := tw.Close(); err != nil {
		log.Errorf("error closing archive - %v", err)
		return nil, err
	}

	return &b, nil
}

func CreateVolumeWithData(dclient *dockerapi.Client, source, name string, labels map[string]string) error {
	if name == "" {
		return ErrBadParam
	}

	if source != "" {
		if _, err := os.Stat(source); err != nil {
			log.Errorf("CreateVolumeWithData: bad source (%v) = %v", source, err)
			return err
		}
	}

	var err error
	if dclient == nil {
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("CreateVolumeWithData: dockerapi.NewClient() error = %v", err)
			return err
		}
	}

	volumeOptions := dockerapi.CreateVolumeOptions{
		Name:   name,
		Labels: labels,
	}

	volumeInfo, err := dclient.CreateVolume(volumeOptions)
	if err != nil {
		log.Errorf("CreateVolumeWithData: dclient.CreateVolume() error = %v", err)
		return err
	}

	log.Debugf("CreateVolumeWithData: volumeInfo = %+v", volumeInfo)

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
		dclient, err = dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("CopyFromContainer: dockerapi.NewClient() error = %v", err)
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
		log.Errorf("dclient.DownloadFromContainer() error = %v", err)
		dfile.Close()
		return err
	}

	dfile.Close()

	if extract {
		dstDir := filepath.Dir(local)
		arc := archive.NewDefaultArchiver()

		afile, err := os.Open(local)
		if err != nil {
			log.Errorf("os.Open error - %v", err)
			return err
		}

		tarOptions := &archive.TarOptions{
			NoLchown: true,
			UIDMaps:  arc.IDMapping.UIDs(),
			GIDMaps:  arc.IDMapping.GIDs(),
		}
		err = arc.Untar(afile, dstDir, tarOptions)
		if err != nil {
			log.Errorf("error unpacking tar - %v", err)
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
		log.Errorf("os.Open error - %v", err)
		return err
	}

	outFile, err := os.Create(dstPath)
	if err != nil {
		log.Errorf("os.Open error - %v", err)
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
			log.Errorf("error reading archive(%v) - %v", fullPath, err)
			inFile.Close()
			return err
		}

		if hdr == nil || hdr.Name == "" {
			log.Debugf("ignoring bad tar header")
			continue
		}

		if hdr.Name == removePrefix {
			log.Debugf("ignoring tar object: %v", hdr.Name)
			continue
		}

		if hdr.Name != "" && strings.HasPrefix(hdr.Name, removePrefix) {
			hdr.Name = strings.TrimPrefix(hdr.Name, removePrefix)
		}

		if err := tw.WriteHeader(hdr); err != nil {
			log.Errorf("error writing header to archive(%v) - %v", dstPath, err)
			inFile.Close()
			outFile.Close()
			return err
		}

		if _, err := io.Copy(tw, tr); err != nil {
			log.Errorf("error copying data to archive(%v) - %v", dstPath, err)
			inFile.Close()
			outFile.Close()
			return err
		}
	}

	if err := tw.Close(); err != nil {
		log.Errorf("error closing archive(%v) - %v", dstPath, err)
	}

	outFile.Close()
	inFile.Close()

	if removeOrig {
		os.Remove(fullPath)
	}

	return nil
}
