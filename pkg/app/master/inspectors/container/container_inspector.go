package container

import (
	"bufio"
	"bytes"
	goerr "errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerhost"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/container/ipc"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/security/apparmor"
	"github.com/docker-slim/docker-slim/pkg/app/master/security/seccomp"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/ipc/channel"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// Container inspector constants
const (
	SensorBinPath           = "/opt/dockerslim/bin/docker-slim-sensor"
	ContainerNamePat        = "dockerslimk_%v_%v"
	ArtifactsDir            = "artifacts"
	ReportArtifactTar       = "creport.tar"
	ReportFileName          = "creport.json"
	FileArtifactsTar        = "files.tar"
	FileArtifactsOutTar     = "files_out.tar"
	FileArtifactsArchiveTar = "files_archive.tar"
	FileArtifactsDirName    = "files"
	FileArtifactsPrefix     = "files/"
	SensorBinLocal          = "docker-slim-sensor"
	ArtifactsMountPat       = "%s:/opt/dockerslim/artifacts"
	ArtifactsVolumePath     = "/opt/dockerslim/artifacts"
	SensorMountPat          = "%s:/opt/dockerslim/bin/docker-slim-sensor:ro"
	VolumeSensorMountPat    = "%s:/opt/dockerslim/bin:ro"
	LabelName               = "dockerslim"
)

type ovars = commands.OutVars

var (
	cmdPortStrDefault  = fmt.Sprintf("%d", channel.CmdPort)
	cmdPortSpecDefault = dockerapi.Port(fmt.Sprintf("%d/tcp", channel.CmdPort))
	evtPortStrDefault  = fmt.Sprintf("%d", channel.EvtPort)
	evtPortSpecDefault = dockerapi.Port(fmt.Sprintf("%d/tcp", channel.EvtPort))
)

var ErrStartMonitorTimeout = goerr.New("start monitor timeout")

const (
	defaultConnectWait   = 60
	sensorVolumeBaseName = "docker-slim-sensor"
)

// Inspector is a container execution inspector
type Inspector struct {
	ContainerInfo         *dockerapi.Container
	ContainerPortsInfo    string
	ContainerPortList     string
	ContainerID           string
	ContainerName         string
	FatContainerCmd       []string
	LocalVolumePath       string
	DoUseLocalMounts      bool
	SensorVolumeName      string
	DoKeepTmpArtifacts    bool
	StatePath             string
	CmdPort               dockerapi.Port
	EvtPort               dockerapi.Port
	DockerHostIP          string
	ImageInspector        *image.Inspector
	APIClient             *dockerapi.Client
	Overrides             *config.ContainerOverrides
	PortBindings          map[dockerapi.Port][]dockerapi.PortBinding
	DoPublishExposedPorts bool
	Links                 []string
	EtcHostsMaps          []string
	DNSServers            []string
	DNSSearchDomains      []string
	DoShowContainerLogs   bool
	RunTargetAsUser       bool
	VolumeMounts          map[string]config.VolumeMount
	KeepPerms             bool
	PathPerms             map[string]*fsutil.AccessInfo
	ExcludePatterns       map[string]*fsutil.AccessInfo
	PreservePaths         map[string]*fsutil.AccessInfo
	IncludePaths          map[string]*fsutil.AccessInfo
	IncludeBins           map[string]*fsutil.AccessInfo
	IncludeExes           map[string]*fsutil.AccessInfo
	DoIncludeShell        bool
	DoDebug               bool
	PrintState            bool
	PrintPrefix           string
	InContainer           bool
	dockerEventCh         chan *dockerapi.APIEvents
	dockerEventStopCh     chan struct{}
	ipcClient             *ipc.Client
	logger                *log.Entry
	xc                    *commands.ExecutionContext
	crOpts                *config.ContainerRunOptions
}

func pathMapKeys(m map[string]*fsutil.AccessInfo) []string {
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
func NewInspector(
	xc *commands.ExecutionContext,
	crOpts *config.ContainerRunOptions,
	logger *log.Entry,
	client *dockerapi.Client,
	statePath string,
	imageInspector *image.Inspector,
	localVolumePath string,
	doUseLocalMounts bool,
	sensorVolumeName string,
	doKeepTmpArtifacts bool,
	overrides *config.ContainerOverrides,
	portBindings map[dockerapi.Port][]dockerapi.PortBinding,
	doPublishExposedPorts bool,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	runTargetAsUser bool,
	showContainerLogs bool,
	volumeMounts map[string]config.VolumeMount,
	keepPerms bool,
	pathPerms map[string]*fsutil.AccessInfo,
	excludePatterns map[string]*fsutil.AccessInfo,
	preservePaths map[string]*fsutil.AccessInfo,
	includePaths map[string]*fsutil.AccessInfo,
	includeBins map[string]*fsutil.AccessInfo,
	includeExes map[string]*fsutil.AccessInfo,
	doIncludeShell bool,
	doDebug bool,
	inContainer bool,
	printState bool,
	printPrefix string) (*Inspector, error) {

	logger = logger.WithFields(log.Fields{"component": "container.inspector"})
	inspector := &Inspector{
		logger:                logger,
		StatePath:             statePath,
		LocalVolumePath:       localVolumePath,
		DoUseLocalMounts:      doUseLocalMounts,
		SensorVolumeName:      sensorVolumeName,
		DoKeepTmpArtifacts:    doKeepTmpArtifacts,
		CmdPort:               cmdPortSpecDefault,
		EvtPort:               evtPortSpecDefault,
		ImageInspector:        imageInspector,
		APIClient:             client,
		Overrides:             overrides,
		PortBindings:          portBindings,
		DoPublishExposedPorts: doPublishExposedPorts,
		Links:                 links,
		EtcHostsMaps:          etcHostsMaps,
		DNSServers:            dnsServers,
		DNSSearchDomains:      dnsSearchDomains,
		DoShowContainerLogs:   showContainerLogs,
		RunTargetAsUser:       runTargetAsUser,
		VolumeMounts:          volumeMounts,
		KeepPerms:             keepPerms,
		PathPerms:             pathPerms,
		ExcludePatterns:       excludePatterns,
		PreservePaths:         preservePaths,
		IncludePaths:          includePaths,
		IncludeBins:           includeBins,
		IncludeExes:           includeExes,
		DoIncludeShell:        doIncludeShell,
		DoDebug:               doDebug,
		PrintState:            printState,
		PrintPrefix:           printPrefix,
		InContainer:           inContainer,
		xc:                    xc,
		crOpts:                crOpts,
	}

	if overrides != nil && ((len(overrides.Entrypoint) > 0) || overrides.ClearEntrypoint) {
		logger.Debugf("overriding Entrypoint %+v => %+v (%v)",
			imageInspector.ImageInfo.Config.Entrypoint, overrides.Entrypoint, overrides.ClearEntrypoint)
		if len(overrides.Entrypoint) > 0 {
			inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Entrypoint...)
		}

	} else if len(imageInspector.ImageInfo.Config.Entrypoint) > 0 {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Entrypoint...)
	}

	if overrides != nil && ((len(overrides.Cmd) > 0) || overrides.ClearCmd) {
		logger.Debugf("overriding Cmd %+v => %+v (%v)",
			imageInspector.ImageInfo.Config.Cmd, overrides.Cmd, overrides.ClearCmd)
		if len(overrides.Cmd) > 0 {
			inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Cmd...)
		}

	} else if len(imageInspector.ImageInfo.Config.Cmd) > 0 {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Cmd...)
	}

	emptyIdx := -1
	for idx, val := range inspector.FatContainerCmd {
		val = strings.TrimSpace(val)
		if val != "" {
			break
		}

		emptyIdx = idx
	}

	if emptyIdx > -1 {
		inspector.FatContainerCmd = inspector.FatContainerCmd[emptyIdx+1:]
	}

	logger.Debugf("FatContainerCmd - %+v", inspector.FatContainerCmd)

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

	if !fsutil.Exists(sensorPath) {
		if i.PrintState {
			i.xc.Out.Info("sensor.error",
				ovars{
					"message":  "sensor binary not found",
					"location": sensorPath,
				})

			i.xc.Out.State("exited",
				ovars{
					"exit.code": -125,
					"component": "container.inspector",
					"version":   v.Current(),
				})
		}

		os.Exit(-125)
	}

	if finfo, err := os.Lstat(sensorPath); err == nil {
		i.logger.Debugf("RunContainer: sensor (%s) perms => %#o", sensorPath, finfo.Mode().Perm())
		if finfo.Mode().Perm()&fsutil.FilePermUserExe == 0 {
			i.logger.Debugf("RunContainer: sensor (%s) missing execute permission", sensorPath)
			updatedMode := finfo.Mode() | fsutil.FilePermUserExe | fsutil.FilePermGroupExe | fsutil.FilePermOtherExe
			if err = os.Chmod(sensorPath, updatedMode); err != nil {
				i.logger.Errorf("RunContainer: error updating sensor (%s) perms (%#o -> %#o) => %v",
					sensorPath, finfo.Mode().Perm(), updatedMode.Perm(), err)
			}
		}
	} else {
		i.logger.Errorf("RunContainer: error getting sensor (%s) info => %#v", sensorPath, err)
	}

	var volumeBinds []string
	if i.crOpts != nil && i.crOpts.HostConfig != nil {
		volumeBinds = i.crOpts.HostConfig.Binds
	}

	configVolumes := i.Overrides.Volumes
	if configVolumes == nil {
		configVolumes = map[string]struct{}{}
	}

	var err error
	var volumeName string
	if !i.DoUseLocalMounts {
		volumeName, err = ensureSensorVolume(i.logger, i.APIClient, sensorPath, i.SensorVolumeName)
		errutil.FailOn(err)
	}

	var artifactsMountInfo string
	if i.DoUseLocalMounts {
		artifactsMountInfo = fmt.Sprintf(ArtifactsMountPat, artifactsPath)
		volumeBinds = append(volumeBinds, artifactsMountInfo)
	} else {
		artifactsMountInfo = ArtifactsVolumePath
		configVolumes[artifactsMountInfo] = struct{}{}
	}

	var sensorMountInfo string
	if i.DoUseLocalMounts {
		sensorMountInfo = fmt.Sprintf(SensorMountPat, sensorPath)
	} else {
		sensorMountInfo = fmt.Sprintf(VolumeSensorMountPat, volumeName)
	}

	volumeBinds = append(volumeBinds, sensorMountInfo)

	for _, volumeMount := range i.VolumeMounts {
		mountInfo := fmt.Sprintf("%s:%s:%s", volumeMount.Source, volumeMount.Destination, volumeMount.Options)
		volumeBinds = append(volumeBinds, mountInfo)
	}

	var containerCmd []string
	if i.DoDebug {
		containerCmd = append(containerCmd, "-d")
	}

	i.ContainerName = fmt.Sprintf(ContainerNamePat, os.Getpid(), time.Now().UTC().Format("20060102150405"))

	labels := i.Overrides.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	labels["runtime.container.type"] = LabelName

	var hostConfig *dockerapi.HostConfig
	if i.crOpts != nil && i.crOpts.HostConfig != nil {
		hostConfig = i.crOpts.HostConfig
	}

	if hostConfig == nil {
		hostConfig = &dockerapi.HostConfig{}
	}

	hostConfig.Binds = volumeBinds
	hostConfig.Privileged = true
	hostConfig.UsernsMode = "host"

	hasSysAdminCap := false
	for _, cap := range hostConfig.CapAdd {
		if cap == "SYS_ADMIN" {
			hasSysAdminCap = true
		}
	}

	if !hasSysAdminCap {
		hostConfig.CapAdd = append(hostConfig.CapAdd, "SYS_ADMIN")
	}

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
			Labels:     labels,
			Hostname:   i.Overrides.Hostname,
		},
		HostConfig: hostConfig,
	}

	if i.crOpts != nil {
		if i.crOpts.Runtime != "" {
			containerOptions.HostConfig.Runtime = i.crOpts.Runtime
			i.logger.Debugf("RunContainer: using custom runtime => %s", containerOptions.HostConfig.Runtime)
		}

		if len(i.crOpts.SysctlParams) > 0 {
			containerOptions.HostConfig.Sysctls = i.crOpts.SysctlParams
			i.logger.Debugf("RunContainer: using sysctl params => %#v", containerOptions.HostConfig.Sysctls)
		}

		if i.crOpts.ShmSize > -1 {
			containerOptions.HostConfig.ShmSize = i.crOpts.ShmSize
			i.logger.Debugf("RunContainer: using shm-size params => %#v", containerOptions.HostConfig.ShmSize)
		}
	}

	if len(configVolumes) > 0 {
		containerOptions.Config.Volumes = configVolumes
	}

	runAsUser := i.ImageInspector.ImageInfo.Config.User
	containerOptions.Config.User = "0:0"

	if runAsUser != "" && strings.ToLower(runAsUser) != "root" {
		//containerOptions.Config.Tty = true
		//containerOptions.Config.OpenStdin = true
		//NOTE:
		//when enabling TTY need to add extra params getting logs
		//or the client.Logs() call will fail with an
		//"Unrecognized input header" error
	}

	commsExposedPorts := map[dockerapi.Port]struct{}{
		i.CmdPort: {},
		i.EvtPort: {},
	}

	//add comms ports to the exposed ports in the container
	if len(i.Overrides.ExposedPorts) > 0 {
		containerOptions.Config.ExposedPorts = i.Overrides.ExposedPorts
		for k, v := range commsExposedPorts {
			if _, ok := containerOptions.Config.ExposedPorts[k]; ok {
				i.logger.Errorf("RunContainer: comms port conflict => %v", k)
			}

			containerOptions.Config.ExposedPorts[k] = v
		}
		i.logger.Debugf("RunContainer: Config.ExposedPorts => %#v", containerOptions.Config.ExposedPorts)
	} else {
		containerOptions.Config.ExposedPorts = commsExposedPorts
		i.logger.Debugf("RunContainer: default exposed ports => %#v", containerOptions.Config.ExposedPorts)
	}

	if len(i.PortBindings) > 0 {
		//need to add the IPC ports too
		if pbInfo, ok := i.PortBindings[dockerapi.Port(cmdPortSpecDefault)]; ok {
			i.logger.Errorf("RunContainer: port bindings comms port conflict (cmd) = %#v", pbInfo)
			if i.PrintState {
				i.xc.Out.Info("sensor.error",
					ovars{
						"message": "port binding ipc port conflict",
						"type":    "cmd",
					})

				i.xc.Out.State("exited",
					ovars{
						"exit.code": -126,
						"component": "container.inspector",
						"version":   v.Current(),
					})
			}

			os.Exit(-126)
		}

		i.PortBindings[dockerapi.Port(cmdPortSpecDefault)] = []dockerapi.PortBinding{{
			HostPort: cmdPortStrDefault,
		}}

		if pbInfo, ok := i.PortBindings[dockerapi.Port(evtPortSpecDefault)]; ok {
			i.logger.Errorf("RunContainer: port bindings comms port conflict (evt) = %#v", pbInfo)
			if i.PrintState {
				i.xc.Out.Info("sensor.error",
					ovars{
						"message": "port binding ipc port conflict",
						"type":    "evt",
					})

				i.xc.Out.State("exited",
					ovars{
						"exit.code": -127,
						"component": "container.inspector",
						"version":   v.Current(),
					})
			}

			os.Exit(-127)
		}

		i.PortBindings[dockerapi.Port(evtPortSpecDefault)] = []dockerapi.PortBinding{{
			HostPort: evtPortStrDefault,
		}}

		containerOptions.HostConfig.PortBindings = i.PortBindings
	} else {
		if i.DoPublishExposedPorts {
			portBindings := map[dockerapi.Port][]dockerapi.PortBinding{}

			if i.ImageInspector.ImageInfo.Config != nil {
				for k := range i.ImageInspector.ImageInfo.Config.ExposedPorts {
					parts := strings.Split(string(k), "/")
					portBindings[k] = []dockerapi.PortBinding{{
						HostPort: parts[0],
					}}
				}
			}

			for k := range containerOptions.Config.ExposedPorts {
				parts := strings.Split(string(k), "/")
				portBindings[k] = []dockerapi.PortBinding{{
					HostPort: parts[0],
				}}
			}

			containerOptions.HostConfig.PortBindings = portBindings
			i.logger.Debugf("RunContainer: publishExposedPorts/portBindings => %+v", portBindings)
		} else {
			containerOptions.HostConfig.PublishAllPorts = true
		}
	}

	if i.Overrides.Network != "" {
		containerOptions.HostConfig.NetworkMode = i.Overrides.Network
		i.logger.Debugf("RunContainer: HostConfig.NetworkMode => %v", i.Overrides.Network)
	}

	// adding this separately for better visibility...
	if len(i.Links) > 0 {
		containerOptions.HostConfig.Links = i.Links
		i.logger.Debugf("RunContainer: HostConfig.Links => %v", i.Links)
	}

	if len(i.EtcHostsMaps) > 0 {
		containerOptions.HostConfig.ExtraHosts = i.EtcHostsMaps
		i.logger.Debugf("RunContainer: HostConfig.ExtraHosts => %v", i.EtcHostsMaps)
	}

	if len(i.DNSServers) > 0 {
		containerOptions.HostConfig.DNS = i.DNSServers //for newer versions of Docker
		containerOptions.Config.DNS = i.DNSServers     //for older versions of Docker
		i.logger.Debugf("RunContainer: HostConfig.DNS/Config.DNS => %v", i.DNSServers)
	}

	if len(i.DNSSearchDomains) > 0 {
		containerOptions.HostConfig.DNSSearch = i.DNSSearchDomains
		i.logger.Debugf("RunContainer: HostConfig.DNSSearch => %v", i.DNSSearchDomains)
	}

	containerInfo, err := i.APIClient.CreateContainer(containerOptions)
	if err != nil {
		return err
	}

	if i.ContainerName != containerInfo.Name {
		i.logger.Debugf("RunContainer: Container name mismatch expected=%v got=%v", i.ContainerName, containerInfo.Name)
	}

	i.ContainerID = containerInfo.ID

	if i.PrintState {
		i.xc.Out.Info("container",
			ovars{
				"status": "created",
				"name":   containerInfo.Name,
				"id":     i.ContainerID,
			})
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
						nonZeroExitCode := false
						if exitCodeStr, ok := devent.Actor.Attributes["exitCode"]; ok && exitCodeStr != "" && exitCodeStr != "0" {
							nonZeroExitCode = true
						}

						if nonZeroExitCode {
							if i.PrintState {
								i.xc.Out.Info("container",
									ovars{
										"status": "crashed",
										"id":     i.ContainerID,
									})
							}

							i.ShowContainerLogs()

							if i.PrintState {
								i.xc.Out.State("exited",
									ovars{
										"exit.code": -123,
										"version":   v.Current(),
									})
							}
							os.Exit(-123)
						}
					}
				}

			case <-i.dockerEventStopCh:
				i.logger.Debug("RunContainer: Docker event monitor stopped")
				return
			}
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	go func() {
		<-signals
		_ = i.APIClient.KillContainer(dockerapi.KillContainerOptions{ID: i.ContainerID})
		i.logger.Fatalf("KillContainer: docker-slim received SIGINT, killing container %s", i.ContainerID)
	}()

	if err := i.APIClient.StartContainer(i.ContainerID, nil); err != nil {
		return err
	}

	if i.ContainerInfo, err = i.APIClient.InspectContainer(i.ContainerID); err != nil {
		return err
	}

	errutil.FailWhen(i.ContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")

	if i.ContainerInfo.HostConfig != nil &&
		i.ContainerInfo.HostConfig.NetworkMode != "host" {
		i.logger.Debugf("RunContainer: container HostConfig.NetworkMode => %s len(ports)=%d",
			i.ContainerInfo.HostConfig.NetworkMode, len(i.ContainerInfo.NetworkSettings.Ports))
		errutil.FailWhen(len(i.ContainerInfo.NetworkSettings.Ports) < len(commsExposedPorts), "docker-slim: error => missing comms ports")
	}

	i.logger.Debugf("RunContainer: container NetworkSettings.Ports => %#v", i.ContainerInfo.NetworkSettings.Ports)

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

	if len(i.ExcludePatterns) > 0 {
		cmd.Excludes = pathMapKeys(i.ExcludePatterns)
	}

	if len(i.PreservePaths) > 0 {
		cmd.Preserves = i.PreservePaths
	}

	if len(i.IncludePaths) > 0 {
		cmd.Includes = i.IncludePaths
	}

	cmd.KeepPerms = i.KeepPerms

	if len(i.PathPerms) > 0 {
		cmd.Perms = i.PathPerms
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

		if strings.ToLower(runAsUser) != "root" {
			cmd.RunTargetAsUser = i.RunTargetAsUser
		}
	}

	_, err = i.ipcClient.SendCommand(cmd)
	if err != nil {
		return err
	}

	if i.PrintState {
		i.xc.Out.Info("cmd.startmonitor",
			ovars{
				"status": "sent",
			})
	}

	for idx := 0; idx < 3; idx++ {
		evt, err := i.ipcClient.GetEvent()

		//don't want to expose mangos here...
		if err != nil {
			if os.IsTimeout(err) || err == channel.ErrWaitTimeout {
				if i.PrintState {
					i.xc.Out.Info("event.startmonitor.done",
						ovars{
							"status": "receive.timeout",
						})
				}

				i.logger.Debug("timeout waiting for the docker-slim container to start...")
				continue
			}

			return err
		}

		if evt == nil || evt.Name == "" {
			i.logger.Debug("empty event waiting for the docker-slim container to start (trying again)...")
			continue
		}

		if evt.Name == event.StartMonitorDone {
			if i.PrintState {
				i.xc.Out.Info("event.startmonitor.done",
					ovars{
						"status": "received",
					})
			}
			return nil
		}

		if evt.Name == event.Error {
			if i.PrintState {
				i.xc.Out.Info("event.error",
					ovars{
						"status": "received",
						"data":   evt.Data,
					})

				i.xc.Out.State("exited",
					ovars{
						"exit.code": -124,
						"component": "container.inspector",
						"version":   v.Current(),
					})
			}

			os.Exit(-124)
		}

		if evt.Name != event.StartMonitorDone {
			if i.PrintState {
				i.xc.Out.Info("event.startmonitor.done",
					ovars{
						"status": "received.unexpected",
						"data":   fmt.Sprintf("%+v", evt),
					})
			}
			return event.ErrUnexpectedEvent
		}
	}

	return ErrStartMonitorTimeout
}

func (i *Inspector) ShowContainerLogs() {
	var outData bytes.Buffer
	outw := bufio.NewWriter(&outData)
	var errData bytes.Buffer
	errw := bufio.NewWriter(&errData)

	i.logger.Debug("getting container logs => ", i.ContainerID)
	logsOptions := dockerapi.LogsOptions{
		Container:    i.ContainerID,
		OutputStream: outw,
		ErrorStream:  errw,
		Stdout:       true,
		Stderr:       true,
	}

	err := i.APIClient.Logs(logsOptions)
	if err != nil {
		i.logger.Infof("error getting container logs => %v - %v", i.ContainerID, err)
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
	if !i.DoUseLocalMounts {
		deleteOrig := true
		if i.DoKeepTmpArtifacts {
			deleteOrig = false
		}

		reportLocalPath := filepath.Join(i.LocalVolumePath, ArtifactsDir, ReportArtifactTar)
		reportRemotePath := filepath.Join(ArtifactsVolumePath, ReportFileName)
		err := dockerutil.CopyFromContainer(i.APIClient, i.ContainerID, reportRemotePath, reportLocalPath, true, deleteOrig)
		if err != nil {
			errutil.FailOn(err)
		}

		/*
			//ALTERNATIVE WAY TO XFER THE FILE ARTIFACTS
			filesOutLocalPath := filepath.Join(i.LocalVolumePath, ArtifactsDir, FileArtifactsArchiveTar)
			filesTarRemotePath := filepath.Join(ArtifactsVolumePath, FileArtifactsTar)
			err = dockerutil.CopyFromContainer(i.APIClient,
				i.ContainerID,
				filesTarRemotePath,
				filesOutLocalPath,
				true,
				false) //make it 'true' once tested/debugged
			if err != nil {
				errutil.FailOn(err)
			}
		*/

		filesOutLocalPath := filepath.Join(i.LocalVolumePath, ArtifactsDir, FileArtifactsOutTar)
		filesRemotePath := filepath.Join(ArtifactsVolumePath, FileArtifactsDirName)
		err = dockerutil.CopyFromContainer(i.APIClient, i.ContainerID, filesRemotePath, filesOutLocalPath, false, false)
		if err != nil {
			errutil.FailOn(err)
		}

		//NOTE: possible enhancement (if the original filemode bits still get lost)
		//(alternative to archiving files in the container to preserve filemodes)
		//Rewrite the filemode bits using the data from creport.json,
		//but creport.json also needs to be enhanced to use
		//octal filemodes for the file records
		err = dockerutil.PrepareContainerDataArchive(filesOutLocalPath, FileArtifactsTar, FileArtifactsPrefix, deleteOrig)
		if err != nil {
			errutil.FailOn(err)
		}
	}

	i.shutdownContainerChannels()

	if i.DoShowContainerLogs {
		i.ShowContainerLogs()
	}

	err := i.APIClient.StopContainer(i.ContainerID, 9)

	if _, ok := err.(*dockerapi.ContainerNotRunning); ok {
		i.logger.Info("can't stop the docker-slim container (container is not running)...")
	} else {
		errutil.WarnOn(err)
	}

	removeOption := dockerapi.RemoveContainerOptions{
		ID:            i.ContainerID,
		RemoveVolumes: true,
		Force:         true,
	}

	if err := i.APIClient.RemoveContainer(removeOption); err != nil {
		i.logger.Info("error removing container =>", err)
	}

	return nil
}

// FinishMonitoring ends the target container monitoring activities
func (i *Inspector) FinishMonitoring() {
	close(i.dockerEventStopCh)
	i.dockerEventStopCh = nil

	cmdResponse, err := i.ipcClient.SendCommand(&command.StopMonitor{})
	errutil.WarnOn(err)
	//_ = cmdResponse
	i.logger.Debugf("'stop' monitor response => '%v'", cmdResponse)

	i.logger.Info("waiting for the container to finish its work...")

	evt, err := i.ipcClient.GetEvent()
	i.logger.Debugf("sensor event => '%v'", evt)

	errutil.WarnOn(err)
	_ = evt
	i.logger.Debugf("sensor event => '%v'", evt)

	cmdResponse, err = i.ipcClient.SendCommand(&command.ShutdownSensor{})
	if err != nil {
		i.logger.Debugf("error sending 'shutdown' => '%v'", err)
	}
	i.logger.Debugf("'shutdown' sensor response => '%v'", cmdResponse)
}

func (i *Inspector) initContainerChannels() error {
	var targetHost string
	var cmdPort string
	var evtPort string

	if i.InContainer || i.Overrides.Network == "host" {
		targetHost = i.ContainerInfo.NetworkSettings.IPAddress
		cmdPort = cmdPortStrDefault
		evtPort = evtPortStrDefault
	} else {
		cmdPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.CmdPort]
		evtPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.EvtPort]
		i.DockerHostIP = dockerhost.GetIP()

		targetHost = i.DockerHostIP
		cmdPort = cmdPortBindings[0].HostPort
		evtPort = evtPortBindings[0].HostPort
	}

	ipcClient, err := ipc.NewClient(targetHost, cmdPort, evtPort, defaultConnectWait)
	if err != nil {
		return err
	}

	i.ipcClient = ipcClient
	return nil
}

func (i *Inspector) shutdownContainerChannels() {
	if i.ipcClient != nil {
		i.ipcClient.Stop()
		i.ipcClient = nil
	}
}

// HasCollectedData returns true if any data was produced monitoring the target container
func (i *Inspector) HasCollectedData() bool {
	return fsutil.Exists(filepath.Join(i.ImageInspector.ArtifactLocation, report.DefaultContainerReportFileName))
}

// ProcessCollectedData performs post-processing on the collected container data
func (i *Inspector) ProcessCollectedData() error {
	i.logger.Info("generating AppArmor profile...")
	err := apparmor.GenProfile(i.ImageInspector.ArtifactLocation, i.ImageInspector.AppArmorProfileName)
	if err != nil {
		return err
	}

	return seccomp.GenProfile(i.ImageInspector.ArtifactLocation, i.ImageInspector.SeccompProfileName)
}

/////////////////////////////////////////////////////////////////////////////////

func sensorVolumeName() string {
	return fmt.Sprintf("%s.%s", sensorVolumeBaseName, v.Tag())
}

func ensureSensorVolume(logger *log.Entry, client *dockerapi.Client, localSensorPath, volumeName string) (string, error) {
	if volumeName == "" {
		volumeName = sensorVolumeName()
	}

	err := dockerutil.HasVolume(client, volumeName)
	switch {
	case err == nil:
		logger.Debugf("ensureSensorVolume: already have volume = %v", volumeName)
	case err == dockerutil.ErrNotFound:
		logger.Debugf("ensureSensorVolume: no volume yet = %v", volumeName)
		if dockerutil.HasEmptyImage(client) == dockerutil.ErrNotFound {
			err := dockerutil.BuildEmptyImage(client)
			if err != nil {
				logger.Debugf("ensureSensorVolume: dockerutil.BuildEmptyImage() - error = %v", err)
				return "", err
			}
		}

		err = dockerutil.CreateVolumeWithData(client, localSensorPath, volumeName, nil)
		if err != nil {
			logger.Debugf("ensureSensorVolume: dockerutil.CreateVolumeWithData() - error = %v", err)
			return "", err
		}
	default:
		logger.Debugf("ensureSensorVolume: dockerutil.HasVolume() - error = %v", err)
		return "", err
	}

	return volumeName, nil
}
