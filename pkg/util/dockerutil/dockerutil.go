package dockerutil

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"os"

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
	emptyImageName       = "docker-slim-empty-image"
	emptyImageDockerfile = "FROM scratch\nCMD\n"
)

func HasEmptyImage() error {
	dclient, err := dockerapi.NewClient(dockerHost)
	if err != nil {
		log.Errorf("HasEmptyImage: dockerapi.NewClient() error = %v", err)
		return err
	}

	listOptions := dockerapi.ListImagesOptions{
		Filter: emptyImageName, // works with "docker-slim-empty-image:latest" too
		All:    false,
	}

	imageList, err := dclient.ListImages(listOptions)
	if err != nil {
		log.Errorf("HasEmptyImage: dockerapi.ListImages() error = %v", err)
		return err
	}

	if len(imageList) == 0 {
		log.Debug("HasEmptyImage: empty image not found")
		return ErrNotFound
	}

	return nil
}

func BuildEmptyImage() error {
	dclient, err := dockerapi.NewClient(dockerHost)
	if err != nil {
		log.Errorf("BuildEmptyImage: dockerapi.NewClient() error = %v", err)
		return err
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

func HasVolume(name string) error {
	if name == "" {
		return ErrBadParam
	}

	dclient, err := dockerapi.NewClient(dockerHost)
	if err != nil {
		log.Errorf("HasVolume: dockerapi.NewClient() error = %v", err)
		return err
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

func DeleteVolume(name string) error {
	if name == "" {
		return ErrBadParam
	}

	if err := HasVolume(name); err == nil {
		dclient, err := dockerapi.NewClient(dockerHost)
		if err != nil {
			log.Errorf("DeleteVolume: dockerapi.NewClient() error = %v", err)
			return err
		}

		removeOptions := dockerapi.RemoveVolumeOptions{
			Name:  name,
			Force: true,
		}

		//ok to call remove even if the volume isn't there
		err = dclient.RemoveVolumeWithOptions(removeOptions)
		if err != nil {
			fmt.Printf("CreateVolumeWithData: dclient.RemoveVolumeWithOptions() error = %v\n", err)
			return err
		}
	}

	return nil
}

func CreateVolumeWithData(source, name string, labels map[string]string) error {
	if source == "" || name == "" {
		return ErrBadParam
	}

	if _, err := os.Stat(source); err != nil {
		log.Errorf("CreateVolumeWithData: bad source = %v", err)
		return err
	}

	dclient, err := dockerapi.NewClient(dockerHost)
	if err != nil {
		log.Errorf("CreateVolumeWithData: dockerapi.NewClient() error = %v", err)
		return err
	}

	volumeOptions := dockerapi.CreateVolumeOptions{
		Name:   name,
		Labels: labels,
	}

	log.Info("CreateVolumeWithData: creating volume...")
	volumeInfo, err := dclient.CreateVolume(volumeOptions)
	if err != nil {
		log.Errorf("CreateVolumeWithData: dclient.CreateVolume() error = %v", err)
		return err
	}

	log.Infof("CreateVolumeWithData: volumeInfo = %#v", volumeInfo)

	volumeBinds := []string{fmt.Sprintf(volumeMountPat, name)}

	containerOptions := dockerapi.CreateContainerOptions{
		Name: name,
		Config: &dockerapi.Config{
			Image:  emptyImageName,
			Labels: map[string]string{"owner": "docker-slim"},
		},
		HostConfig: &dockerapi.HostConfig{
			Binds: volumeBinds,
		},
	}

	log.Info("CreateVolumeWithData: creating container...")
	containerInfo, err := dclient.CreateContainer(containerOptions)
	if err != nil {
		log.Errorf("dclient.CreateContainer() error = %v", err)
		return err
	}

	containerID := containerInfo.ID
	log.Debugf("CreateVolumeWithData: containerID - %v", containerID)

	defer func() {
		removeOptions := dockerapi.RemoveContainerOptions{
			ID:    containerID,
			Force: true,
		}

		log.Info("CreateVolumeWithData: removing container (from defer)...")
		err = dclient.RemoveContainer(removeOptions)
		if err != nil {
			fmt.Printf("CreateVolumeWithData: dclient.RemoveContainer() error = %v\n", err)
		}
	}()

	log.Info("CreateVolumeWithData: creating tar data for volume...")
	tarData, err := archive.Tar(source, archive.Uncompressed)
	if err != nil {
		log.Errorf("archive.Tar() error = %v", err)
		return err
	}

	defer tarData.Close()

	uploadOptions := dockerapi.UploadToContainerOptions{
		InputStream: tarData,
		Path:        "/data",
	}

	log.Info("CreateVolumeWithData: uploading data...")
	err = dclient.UploadToContainer(containerID, uploadOptions)
	if err != nil {
		log.Errorf("dclient.UploadToContainer() error = %v", err)
		return err
	}

	return nil
}

func CopyFromContainer(remove, local string) error {
	return nil
}
