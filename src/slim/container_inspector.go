package main

import (
	//"bytes"
	"fmt"
	//"io/ioutil"
	//"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

type ContainerInspector struct {
	ContainerInfo   *docker.Container
	ContainerID     string
	FatContainerCmd []string
	LocalVolumePath string
	CmdPort         docker.Port
	EvtPort         docker.Port
	DockerHostIP    string
	ImgInspector    *ImageInspector
	ApiClient       *docker.Client
}

func NewContainerInspector(client *docker.Client, imageInspector *ImageInspector, localVolumePath string) (*ContainerInspector, error) {
	inspector := &ContainerInspector{
		LocalVolumePath: localVolumePath,
		CmdPort:         "65501/tcp",
		EvtPort:         "65502/tcp",
		ImgInspector:    imageInspector,
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

func (i *ContainerInspector) RunContainer() error {
	mountInfo := fmt.Sprintf("%s:/opt/dockerslim", i.LocalVolumePath)

	containerOptions := docker.CreateContainerOptions{
		Name: "dockerslimk",
		Config: &docker.Config{
			Image: i.ImgInspector.ImageRef,
			ExposedPorts: map[docker.Port]struct{}{
				i.CmdPort: struct{}{},
				i.EvtPort: struct{}{},
			},
			Entrypoint: []string{"/opt/dockerslim/bin/alauncher"},
			Cmd:        i.FatContainerCmd,
			Labels:     map[string]string{"type": "dockerslim"},
		},
		HostConfig: &docker.HostConfig{
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

	if err := i.ApiClient.StartContainer(i.ContainerID, &docker.HostConfig{
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

	failWhen(i.ContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")
	log.Debugf("container NetworkSettings.Ports => %#v\n", i.ContainerInfo.NetworkSettings.Ports)

	return i.initContainerChannels()
}

func (i *ContainerInspector) ShutdownContainer() error {
	i.shutdownContainerChannels()

	err := i.ApiClient.StopContainer(i.ContainerID, 9)
	warnOnError(err)

	removeOption := docker.RemoveContainerOptions{
		ID:            i.ContainerID,
		RemoveVolumes: true,
		Force:         true,
	}
	err = i.ApiClient.RemoveContainer(removeOption)
	return nil
}

func (i *ContainerInspector) FinishMonitoring() {
	cmdResponse, err := sendCmd(cmdChannel, "monitor.finish")
	warnOnError(err)
	_ = cmdResponse

	log.Debugf("'monitor.finish' response => '%v'\n", cmdResponse)
	log.Info("docker-slim: waiting for the container finish its work...")

	//for now there's only one event ("done")
	evt, err := getEvt(evtChannel)
	warnOnError(err)
	_ = evt
	log.Debugf("docker-slim: alauncher event => '%v'\n", evt)
}

func (i *ContainerInspector) initContainerChannels() error {
	/*
		NOTE: not using IPC for now... (future option for regular Docker deployments)
		ipcLocation := filepath.Join(localVolumePath,"ipc")
		_, err = os.Stat(ipcLocation)
		if os.IsNotExist(err) {
			os.MkdirAll(ipcLocation, 0777)
			_, err = os.Stat(ipcLocation)
			failOnError(err)
		}
	*/

	cmdPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.CmdPort]
	evtPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.EvtPort]
	i.DockerHostIP = getDockerHostIP()
	cmdChannelAddr = fmt.Sprintf("tcp://%v:%v", i.DockerHostIP, cmdPortBindings[0].HostPort)
	evtChannelAddr = fmt.Sprintf("tcp://%v:%v", i.DockerHostIP, evtPortBindings[0].HostPort)
	log.Debugf("cmdChannelAddr=%v evtChannelAddr=%v\n", cmdChannelAddr, evtChannelAddr)

	//evtChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-launcher.events.ipc", localVolumePath)
	//cmdChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-launcher.cmds.ipc", localVolumePath)

	//NOTE: cmdChannel and evtChannel are globals (need to refactor this hack :))
	var err error
	evtChannel, err = newEvtChannel(evtChannelAddr)
	if err != nil {
		return err
	}
	cmdChannel, err = newCmdClient(cmdChannelAddr)
	if err != nil {
		return err
	}

	return nil
}

func (i *ContainerInspector) shutdownContainerChannels() {
	shutdownEvtChannel()
	shutdownCmdChannel()
}
