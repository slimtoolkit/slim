package container

import (
	"fmt"

	"internal/utils"
	dockerhost "slim/docker/host"
	"slim/inspectors/container/ipc"
	"slim/inspectors/image"
	"slim/security/apparmor"
	"slim/security/seccomp"

	log "github.com/Sirupsen/logrus"
	dockerapi "github.com/cloudimmunity/go-dockerclientx"
)

type Inspector struct {
	ContainerInfo   *dockerapi.Container
	ContainerID     string
	FatContainerCmd []string
	LocalVolumePath string
	CmdPort         dockerapi.Port
	EvtPort         dockerapi.Port
	DockerHostIP    string
	ImageInspector  *image.Inspector
	ApiClient       *dockerapi.Client
}

func NewInspector(client *dockerapi.Client, imageInspector *image.Inspector, localVolumePath string) (*Inspector, error) {
	inspector := &Inspector{
		LocalVolumePath: localVolumePath,
		CmdPort:         "65501/tcp",
		EvtPort:         "65502/tcp",
		ImageInspector:  imageInspector,
		ApiClient:       client,
	}

	if len(imageInspector.ImageInfo.Config.Entrypoint) > 0 {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Entrypoint...)
	}

	if len(imageInspector.ImageInfo.Config.Cmd) > 0 {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Cmd...)
	}

	return inspector, nil
}

func (i *Inspector) RunContainer() error {
	mountInfo := fmt.Sprintf("%s:/opt/dockerslim", i.LocalVolumePath)

	containerOptions := dockerapi.CreateContainerOptions{
		Name: "dockerslimk",
		Config: &dockerapi.Config{
			Image: i.ImageInspector.ImageRef,
			ExposedPorts: map[dockerapi.Port]struct{}{
				i.CmdPort: struct{}{},
				i.EvtPort: struct{}{},
			},
			Entrypoint: []string{"/opt/dockerslim/bin/alauncher"},
			Cmd:        i.FatContainerCmd,
			Labels:     map[string]string{"type": "dockerslim"},
		},
		HostConfig: &dockerapi.HostConfig{
			Binds:           []string{mountInfo},
			PublishAllPorts: true,
			CapAdd:          []string{"SYS_ADMIN"},
			Privileged:      true,
		},
	}

	containerInfo, err := i.ApiClient.CreateContainer(containerOptions)
	if err != nil {
		return err
	}

	i.ContainerID = containerInfo.ID
	log.Infoln("docker-slim: created container =>", i.ContainerID)

	if err := i.ApiClient.StartContainer(i.ContainerID, &dockerapi.HostConfig{
		PublishAllPorts: true,
		CapAdd:          []string{"SYS_ADMIN"},
		Privileged:      true,
	}); err != nil {
		return err
	}

	//inspContainerInfo
	i.ContainerInfo, err = i.ApiClient.InspectContainer(i.ContainerID)
	if err != nil {
		return err
	}

	utils.FailWhen(i.ContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")
	log.Debugf("container NetworkSettings.Ports => %#v\n", i.ContainerInfo.NetworkSettings.Ports)

	return i.initContainerChannels()
}

func (i *Inspector) ShutdownContainer() error {
	i.shutdownContainerChannels()

	err := i.ApiClient.StopContainer(i.ContainerID, 9)
	utils.WarnOn(err)

	removeOption := dockerapi.RemoveContainerOptions{
		ID:            i.ContainerID,
		RemoveVolumes: true,
		Force:         true,
	}
	err = i.ApiClient.RemoveContainer(removeOption)
	return nil
}

func (i *Inspector) FinishMonitoring() {
	cmdResponse, err := ipc.SendContainerCmd("monitor.finish")
	utils.WarnOn(err)
	_ = cmdResponse

	log.Debugf("'monitor.finish' response => '%v'\n", cmdResponse)
	log.Info("docker-slim: waiting for the container finish its work...")

	//for now there's only one event ("done")
	//getEvt() should timeout in two minutes (todo: pick a good timeout)
	evt, err := ipc.GetContainerEvt()
	utils.WarnOn(err)
	_ = evt
	log.Debugf("docker-slim: launcher event => '%v'\n", evt)
}

func (i *Inspector) initContainerChannels() error {
	/*
		NOTE: not using IPC for now... (future option for regular Docker deployments)
		ipcLocation := filepath.Join(localVolumePath,"ipc")
		_, err = os.Stat(ipcLocation)
		if os.IsNotExist(err) {
			os.MkdirAll(ipcLocation, 0777)
			_, err = os.Stat(ipcLocation)
			utils.FailOn(err)
		}
	*/

	cmdPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.CmdPort]
	evtPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.EvtPort]
	i.DockerHostIP = dockerhost.GetIP()

	if err := ipc.InitContainerChannels(i.DockerHostIP, cmdPortBindings[0].HostPort, evtPortBindings[0].HostPort); err != nil {
		return err
	}

	return nil
}

func (i *Inspector) shutdownContainerChannels() {
	ipc.ShutdownContainerChannels()
}

func (i *Inspector) ProcessCollectedData() error {
	log.Info("docker-slim: generating AppArmor profile...")
	err := apparmor.GenProfile(i.ImageInspector.ArtifactLocation, i.ImageInspector.AppArmorProfileName)
	if err != nil {
		return err
	}

	return seccomp.GenProfile(i.ImageInspector.ArtifactLocation, i.ImageInspector.SeccompProfileName)
}
