package container

import (
	"bufio"
	"bytes"
	goerr "errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerhost"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container/ipc"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/internal/app/master/security/apparmor"
	"github.com/docker-slim/docker-slim/internal/app/master/security/seccomp"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/Sirupsen/logrus"
	dockerapi "github.com/cloudimmunity/go-dockerclientx"
)

// IpcErrRecvTimeoutStr - an IPC receive timeout error
const IpcErrRecvTimeoutStr = "receive time out"

// Container inspector constants
const (
	SensorBinPath     = "/opt/dockerslim/bin/sensor"
	ContainerNamePat  = "dockerslimk_%v_%v"
	ArtifactsDir      = "artifacts"
	SensorBinLocal    = "docker-slim-sensor"
	ArtifactsMountPat = "%s:/opt/dockerslim/artifacts"
	SensorMountPat    = "%s:/opt/dockerslim/bin/sensor:ro"
	CmdPortDefault    = "65501/tcp"
	EvtPortDefault    = "65502/tcp"
	LabelName         = "dockerslim"
)

var ErrStartMonitorTimeout = goerr.New("start monitor timeout")

// Inspector is a container execution inspector
type Inspector struct {
	ContainerInfo      *dockerapi.Container
	ContainerPortsInfo string
	ContainerPortList  string
	ContainerID        string
	ContainerName      string
	FatContainerCmd    []string
	LocalVolumePath    string
	StatePath          string
	CmdPort            dockerapi.Port
	EvtPort            dockerapi.Port
	DockerHostIP       string
	ImageInspector     *image.Inspector
	APIClient          *dockerapi.Client
	Overrides          *config.ContainerOverrides
	Links              []string
	EtcHostsMaps       []string
	DNSServers         []string
	DNSSearchDomains   []string
	ShowContainerLogs  bool
	VolumeMounts       map[string]config.VolumeMount
	ExcludePaths       map[string]bool
	IncludePaths       map[string]bool
	IncludeBins        map[string]bool
	IncludeExes        map[string]bool
	DoIncludeShell     bool
	DoDebug            bool
	PrintState         bool
	PrintPrefix        string
	dockerEventCh      chan *dockerapi.APIEvents
	dockerEventStopCh  chan struct{}
}

func pathMapKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

// NewInspector creates a new container execution inspector
func NewInspector(client *dockerapi.Client,
	statePath string,
	imageInspector *image.Inspector,
	localVolumePath string,
	overrides *config.ContainerOverrides,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	showContainerLogs bool,
	volumeMounts map[string]config.VolumeMount,
	excludePaths map[string]bool,
	includePaths map[string]bool,
	includeBins map[string]bool,
	includeExes map[string]bool,
	doIncludeShell bool,
	doDebug bool,
	printState bool,
	printPrefix string) (*Inspector, error) {

	inspector := &Inspector{
		StatePath:         statePath,
		LocalVolumePath:   localVolumePath,
		CmdPort:           CmdPortDefault,
		EvtPort:           EvtPortDefault,
		ImageInspector:    imageInspector,
		APIClient:         client,
		Overrides:         overrides,
		Links:             links,
		EtcHostsMaps:      etcHostsMaps,
		DNSServers:        dnsServers,
		DNSSearchDomains:  dnsSearchDomains,
		ShowContainerLogs: showContainerLogs,
		VolumeMounts:      volumeMounts,
		ExcludePaths:      excludePaths,
		IncludePaths:      includePaths,
		IncludeBins:       includeBins,
		IncludeExes:       includeExes,
		DoIncludeShell:    doIncludeShell,
		DoDebug:           doDebug,
		PrintState:        printState,
		PrintPrefix:       printPrefix,
	}

	if overrides != nil && ((len(overrides.Entrypoint) > 0) || overrides.ClearEntrypoint) {
		log.Debugf("overriding Entrypoint %+v => %+v (%v)",
			imageInspector.ImageInfo.Config.Entrypoint, overrides.Entrypoint, overrides.ClearEntrypoint)
		if len(overrides.Entrypoint) > 0 {
			inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Entrypoint...)
		}

	} else if len(imageInspector.ImageInfo.Config.Entrypoint) > 0 {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Entrypoint...)
	}

	if overrides != nil && ((len(overrides.Cmd) > 0) || overrides.ClearCmd) {
		log.Debugf("overriding Cmd %+v => %+v (%v)",
			imageInspector.ImageInfo.Config.Cmd, overrides.Cmd, overrides.ClearCmd)
		if len(overrides.Cmd) > 0 {
			inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Cmd...)
		}

	} else if len(imageInspector.ImageInfo.Config.Cmd) > 0 {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Cmd...)
	}

	inspector.dockerEventCh = make(chan *dockerapi.APIEvents)
	inspector.dockerEventStopCh = make(chan struct{})

	return inspector, nil
}

// RunContainer starts the container inspector instance execution
func (i *Inspector) RunContainer() error {
	artifactsPath := filepath.Join(i.LocalVolumePath, ArtifactsDir)
	sensorPath := filepath.Join(fsutil.ExeDir(), SensorBinLocal)

	if runtime.GOOS == "darwin" {
		stateSensorPath := filepath.Join(i.StatePath, SensorBinLocal)
		if fsutil.Exists(stateSensorPath) {
			sensorPath = stateSensorPath
		}
	}

	artifactsMountInfo := fmt.Sprintf(ArtifactsMountPat, artifactsPath)
	sensorMountInfo := fmt.Sprintf(SensorMountPat, sensorPath)

	var volumeBinds []string
	for _, volumeMount := range i.VolumeMounts {
		mountInfo := fmt.Sprintf("%s:%s:%s", volumeMount.Source, volumeMount.Destination, volumeMount.Options)
		volumeBinds = append(volumeBinds, mountInfo)
	}

	volumeBinds = append(volumeBinds, artifactsMountInfo)
	volumeBinds = append(volumeBinds, sensorMountInfo)

	var containerCmd []string
	if i.DoDebug {
		containerCmd = append(containerCmd, "-d")
	}

	i.ContainerName = fmt.Sprintf(ContainerNamePat, os.Getpid(), time.Now().UTC().Format("20060102150405"))

	containerOptions := dockerapi.CreateContainerOptions{
		Name: i.ContainerName,
		Config: &dockerapi.Config{
			Image: i.ImageInspector.ImageRef,
			//ExposedPorts: map[dockerapi.Port]struct{}{
			//	i.CmdPort: {},
			//	i.EvtPort: {},
			//},
			Entrypoint: []string{SensorBinPath},
			Cmd:        containerCmd,
			Env:        i.Overrides.Env,
			Labels:     map[string]string{"type": LabelName},
			Hostname:   i.Overrides.Hostname,
		},
		HostConfig: &dockerapi.HostConfig{
			Binds:           volumeBinds,
			PublishAllPorts: true,
			CapAdd:          []string{"SYS_ADMIN"},
			Privileged:      true,
		},
	}

	runAsUser := i.ImageInspector.ImageInfo.Config.User
	containerOptions.Config.User = "0:0"

	commsExposedPorts := map[dockerapi.Port]struct{}{
		i.CmdPort: {},
		i.EvtPort: {},
	}

	if len(i.Overrides.ExposedPorts) > 0 {
		containerOptions.Config.ExposedPorts = i.Overrides.ExposedPorts
		for k, v := range commsExposedPorts {
			if _, ok := containerOptions.Config.ExposedPorts[k]; ok {
				log.Warnf("RunContainer: comms port conflict => %v", k)
			}

			containerOptions.Config.ExposedPorts[k] = v
		}
		log.Debugf("RunContainer: Config.ExposedPorts => %#v", containerOptions.Config.ExposedPorts)
	} else {
		containerOptions.Config.ExposedPorts = commsExposedPorts
		log.Debugf("RunContainer: default exposed ports => %#v", containerOptions.Config.ExposedPorts)
	}

	if i.Overrides.Network != "" {
		containerOptions.HostConfig.NetworkMode = i.Overrides.Network
		log.Debugf("RunContainer: HostConfig.NetworkMode => %v", i.Overrides.Network)
	}

	// adding this separately for better visibility...
	if len(i.Links) > 0 {
		containerOptions.HostConfig.Links = i.Links
		log.Debugf("RunContainer: HostConfig.Links => %v", i.Links)
	}

	if len(i.EtcHostsMaps) > 0 {
		containerOptions.HostConfig.ExtraHosts = i.EtcHostsMaps
		log.Debugf("RunContainer: HostConfig.ExtraHosts => %v", i.EtcHostsMaps)
	}

	if len(i.DNSServers) > 0 {
		containerOptions.HostConfig.DNS = i.DNSServers //for newer versions of Docker
		containerOptions.Config.DNS = i.DNSServers     //for older versions of Docker
		log.Debugf("RunContainer: HostConfig.DNS/Config.DNS => %v", i.DNSServers)
	}

	if len(i.DNSSearchDomains) > 0 {
		containerOptions.HostConfig.DNSSearch = i.DNSSearchDomains
		log.Debugf("RunContainer: HostConfig.DNSSearch => %v", i.DNSSearchDomains)
	}

	containerInfo, err := i.APIClient.CreateContainer(containerOptions)
	if err != nil {
		return err
	}

	i.ContainerID = containerInfo.ID

	if i.PrintState {
		fmt.Printf("%s info=container status=created id=%v\n", i.PrintPrefix, i.ContainerID)
	}

	i.APIClient.AddEventListener(i.dockerEventCh)
	go func() {
		for {
			select {
			case devent := <-i.dockerEventCh:
				if devent == nil || devent.ID == "" || devent.Status == "" {
					break
				}

				if devent.ID == i.ContainerID {
					if devent.Status == "die" {
						//TODO: update the docker client library to get the exit status to know if it really crashed
						if i.PrintState {
							fmt.Printf("%s info=container status=crashed id=%v\n", i.PrintPrefix, i.ContainerID)
						}

						i.showContainerLogs()

						if i.PrintState {
							fmt.Printf("%s state=exited version=%s\n", i.PrintPrefix, v.Current())
						}
						os.Exit(-123)
					}
				}

			case <-i.dockerEventStopCh:
				log.Debug("RunContainer: Docker event monitor stopped")
				return
			}
		}
	}()

	if err := i.APIClient.StartContainer(i.ContainerID, nil); err != nil {
		return err
	}

	if i.ContainerInfo, err = i.APIClient.InspectContainer(i.ContainerID); err != nil {
		return err
	}

	errutil.FailWhen(i.ContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")
	errutil.FailWhen(len(i.ContainerInfo.NetworkSettings.Ports) < len(commsExposedPorts), "docker-slim: error => missing comms ports")
	log.Debugf("RunContainer: container NetworkSettings.Ports => %#v", i.ContainerInfo.NetworkSettings.Ports)

	if len(i.ContainerInfo.NetworkSettings.Ports) > 2 {
		portKeys := make([]string, 0, len(i.ContainerInfo.NetworkSettings.Ports)-2)
		portList := make([]string, 0, len(i.ContainerInfo.NetworkSettings.Ports)-2)
		for pk, pbinding := range i.ContainerInfo.NetworkSettings.Ports {
			if pk != i.CmdPort && pk != i.EvtPort {
				var portInfo string
				if len(pbinding) > 0 {
					portInfo = fmt.Sprintf("%v => %v:%v", pk, pbinding[0].HostIP, pbinding[0].HostPort)
					portList = append(portList, string(pbinding[0].HostPort))
				} else {
					portInfo = string(pk)
				}

				portKeys = append(portKeys, portInfo)
			}
		}

		i.ContainerPortList = strings.Join(portList, ",")
		i.ContainerPortsInfo = strings.Join(portKeys, ",")
	}

	if err = i.initContainerChannels(); err != nil {
		return err
	}

	cmd := &command.StartMonitor{
		AppName: i.FatContainerCmd[0],
	}

	if len(i.FatContainerCmd) > 1 {
		cmd.AppArgs = i.FatContainerCmd[1:]
	}

	if len(i.ExcludePaths) > 0 {
		cmd.Excludes = pathMapKeys(i.ExcludePaths)
	}

	if len(i.IncludePaths) > 0 {
		cmd.Includes = pathMapKeys(i.IncludePaths)
	}

	if len(i.IncludeBins) > 0 {
		cmd.IncludeBins = pathMapKeys(i.IncludeBins)
	}

	if len(i.IncludeExes) > 0 {
		cmd.IncludeExes = pathMapKeys(i.IncludeExes)
	}

	cmd.IncludeShell = i.DoIncludeShell

	if runAsUser != "" {
		cmd.AppUser = runAsUser
	}

	_, err = ipc.SendContainerCmd(cmd)
	if err != nil {
		return err
	}

	if i.PrintState {
		fmt.Printf("%s info=cmd.startmonitor status=sent\n", i.PrintPrefix)
	}

	for idx := 0; idx < 3; idx++ {
		evt, err := ipc.GetContainerEvt()

		//don't want to expose mangos here...
		if err != nil {
			if err.Error() == IpcErrRecvTimeoutStr {
				if i.PrintState {
					fmt.Printf("%s info=event.startmonitor.done status=receive.timeout\n", i.PrintPrefix)
				}

				log.Debug("timeout waiting for the docker-slim container to start...")
				continue
			}

			return err
		}

		if evt == nil || evt.Name == "" {
			log.Debug("empty event waiting for the docker-slim container to start (trying again)...")
			continue
		}

		if evt.Name == event.StartMonitorDone {
			if i.PrintState {
				fmt.Printf("%s info=event.startmonitor.done status=received\n", i.PrintPrefix)
			}
			return nil
		}

		if evt.Name == event.Error {
			if i.PrintState {
				fmt.Printf("%s info=event.error status=received data=%s\n", i.PrintPrefix, evt.Data)
				fmt.Printf("%s state=exited version=%s\n", i.PrintPrefix, v.Current())
			}

			os.Exit(-124)
		}

		if evt.Name != event.StartMonitorDone {
			if i.PrintState {
				fmt.Printf("%s info=event.startmonitor.done status=received.unexpected data=%+v\n", i.PrintPrefix, evt)
			}
			return event.ErrUnexpectedEvent
		}
	}

	return ErrStartMonitorTimeout
}

func (i *Inspector) showContainerLogs() {
	var outData bytes.Buffer
	outw := bufio.NewWriter(&outData)
	var errData bytes.Buffer
	errw := bufio.NewWriter(&errData)

	log.Debug("getting container logs => ", i.ContainerID)
	logsOptions := dockerapi.LogsOptions{
		Container:    i.ContainerID,
		OutputStream: outw,
		ErrorStream:  errw,
		Stdout:       true,
		Stderr:       true,
	}

	err := i.APIClient.Logs(logsOptions)
	if err != nil {
		log.Infof("error getting container logs => %v - %v", i.ContainerID, err)
	} else {
		outw.Flush()
		errw.Flush()
		fmt.Println("docker-slim: container stdout:")
		outData.WriteTo(os.Stdout)
		fmt.Println("docker-slim: container stderr:")
		errData.WriteTo(os.Stdout)
		fmt.Println("docker-slim: end of container logs =============")
	}
}

// ShutdownContainer terminates the container inspector instance execution
func (i *Inspector) ShutdownContainer() error {
	i.shutdownContainerChannels()

	if i.ShowContainerLogs {
		i.showContainerLogs()
	}

	err := i.APIClient.StopContainer(i.ContainerID, 9)

	if _, ok := err.(*dockerapi.ContainerNotRunning); ok {
		log.Info("can't stop the docker-slim container (container is not running)...")
	} else {
		errutil.WarnOn(err)
	}

	removeOption := dockerapi.RemoveContainerOptions{
		ID:            i.ContainerID,
		RemoveVolumes: true,
		Force:         true,
	}

	if err := i.APIClient.RemoveContainer(removeOption); err != nil {
		log.Info("error removing container =>", err)
	}

	return nil
}

// FinishMonitoring ends the target container monitoring activities
func (i *Inspector) FinishMonitoring() {
	close(i.dockerEventStopCh)
	i.dockerEventStopCh = nil

	cmdResponse, err := ipc.SendContainerCmd(&command.StopMonitor{})
	errutil.WarnOn(err)
	//_ = cmdResponse
	log.Debugf("'stop' monitor response => '%v'", cmdResponse)

	log.Info("waiting for the container to finish its work...")

	//for now there's only one event ("done")
	//getEvt() should timeout in two minutes (todo: pick a good timeout)
	evt, err := ipc.GetContainerEvt()
	log.Debugf("sensor event => '%v'", evt)

	//don't want to expose mangos here... mangos.ErrRecvTimeout = errors.New("receive time out")
	if err != nil && err.Error() == IpcErrRecvTimeoutStr {
		log.Info("timeout waiting for the docker-slim container to finish its work...")
		return
	}

	errutil.WarnOn(err)
	_ = evt
	log.Debugf("sensor event => '%v'", evt)

	cmdResponse, err = ipc.SendContainerCmd(&command.ShutdownSensor{})
	if err != nil {
		log.Debugf("error sending 'shutdown' => '%v'", err)
	}
	log.Debugf("'shutdown' sensor response => '%v'", cmdResponse)
}

func (i *Inspector) initContainerChannels() error {
	/*
		NOTE: not using IPC for now... (future option for regular Docker deployments)
		ipcLocation := filepath.Join(localVolumePath,"ipc")
		_, err = os.Stat(ipcLocation)
		if os.IsNotExist(err) {
			os.MkdirAll(ipcLocation, 0777)
			_, err = os.Stat(ipcLocation)
			errutil.FailOn(err)
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

// HasCollectedData returns true if any data was produced monitoring the target container
func (i *Inspector) HasCollectedData() bool {
	return fsutil.Exists(filepath.Join(i.ImageInspector.ArtifactLocation, report.DefaultContainerReportFileName))
}

// ProcessCollectedData performs post-processing on the collected container data
func (i *Inspector) ProcessCollectedData() error {
	log.Info("generating AppArmor profile...")
	err := apparmor.GenProfile(i.ImageInspector.ArtifactLocation, i.ImageInspector.AppArmorProfileName)
	if err != nil {
		return err
	}

	return seccomp.GenProfile(i.ImageInspector.ArtifactLocation, i.ImageInspector.SeccompProfileName)
}
