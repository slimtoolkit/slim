package container

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/slimtoolkit/slim/pkg/aflag"
	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/docker/dockerhost"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/image"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/ipc"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/sensor"
	"github.com/slimtoolkit/slim/pkg/app/master/security/apparmor"
	"github.com/slimtoolkit/slim/pkg/app/master/security/seccomp"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/ipc/channel"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	"github.com/slimtoolkit/slim/pkg/util/jsonutil"
	v "github.com/slimtoolkit/slim/pkg/version"

	containertypes "github.com/docker/docker/api/types/container"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// Container inspector constants
const (
	SensorIPCModeDirect = "direct"
	SensorIPCModeProxy  = "proxy"
	SensorBinPath       = "/opt/_slim/bin/slim-sensor"
	ContainerNamePat    = "slimk_%v_%v"
	ArtifactsDir        = "artifacts"
	ReportArtifactTar   = "creport.tar"
	fileArtifactsTar    = "files.tar"
	FileArtifactsOutTar = "files_out.tar"
	// FileArtifactsArchiveTar = "files_archive.tar"
	SensorMountPat       = "%s:/opt/_slim/bin/slim-sensor:ro"
	VolumeSensorMountPat = "%s:/opt/_slim/bin:ro"
	LabelName            = "_slim"
	MondelArtifactTar    = "mondel.tar"
)

type ovars = app.OutVars

var (
	cmdPortStrDefault  = fmt.Sprintf("%d", channel.CmdPort)
	cmdPortSpecDefault = dockerapi.Port(fmt.Sprintf("%d/tcp", channel.CmdPort))
	evtPortStrDefault  = fmt.Sprintf("%d", channel.EvtPort)
	evtPortSpecDefault = dockerapi.Port(fmt.Sprintf("%d/tcp", channel.EvtPort))
)

var ErrStartMonitorTimeout = errors.New("start monitor timeout")

const (
	sensorVolumeBaseName = "slim-sensor"
)

type NetNameInfo struct {
	Name     string
	FullName string
	Aliases  []string
}

// TODO(estroz): move all fields configured only after RunContainer is called
// to a InspectorRunResponse struct returned by RunContainer.

// Inspector is a container execution inspector
type Inspector struct {
	ContainerInfo         *dockerapi.Container
	ContainerPortsInfo    string
	ContainerPortList     string
	AvailablePorts        map[dockerapi.Port]dockerapi.PortBinding // Ports found to be available for probing.
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
	ExplicitVolumeMounts  map[string]config.VolumeMount
	BaseMounts            []dockerapi.HostMount
	BaseVolumesFrom       []string
	DoPublishExposedPorts bool
	HasClassicLinks       bool
	Links                 []string
	EtcHostsMaps          []string
	DNSServers            []string
	DNSSearchDomains      []string
	DoShowContainerLogs   bool
	DoEnableMondel        bool
	RunTargetAsUser       bool
	KeepPerms             bool
	PathPerms             map[string]*fsutil.AccessInfo
	ExcludePatterns       map[string]*fsutil.AccessInfo
	DoExcludeVarLockFiles bool
	PreservePaths         map[string]*fsutil.AccessInfo
	IncludePaths          map[string]*fsutil.AccessInfo
	IncludeBins           map[string]*fsutil.AccessInfo
	IncludeDirBinsList    map[string]*fsutil.AccessInfo
	IncludeExes           map[string]*fsutil.AccessInfo
	DoIncludeShell        bool
	DoIncludeWorkdir      bool
	DoIncludeCertAll      bool
	DoIncludeCertBundles  bool
	DoIncludeCertDirs     bool
	DoIncludeCertPKAll    bool
	DoIncludeCertPKDirs   bool
	DoIncludeNew          bool
	DoIncludeSSHClient    bool
	DoIncludeOSLibsNet    bool
	DoIncludeZoneInfo     bool
	SelectedNetworks      map[string]NetNameInfo
	DoDebug               bool
	LogLevel              string
	LogFormat             string
	PrintState            bool
	InContainer           bool
	RTASourcePT           bool
	DoObfuscateMetadata   bool
	SensorIPCEndpoint     string
	SensorIPCMode         string
	TargetHost            string
	dockerEventCh         chan *dockerapi.APIEvents
	dockerEventStopCh     chan struct{}
	isDone                aflag.Type
	ipcClient             *ipc.Client
	logger                *log.Entry
	xc                    *app.ExecutionContext
	crOpts                *config.ContainerRunOptions
	portBindings          map[dockerapi.Port][]dockerapi.PortBinding
	appNodejsInspectOpts  config.AppNodejsInspectOptions
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
	xc *app.ExecutionContext,
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
	explicitVolumeMounts map[string]config.VolumeMount,
	baseMounts []dockerapi.HostMount,
	baseVolumesFrom []string,
	portBindings map[dockerapi.Port][]dockerapi.PortBinding,
	doPublishExposedPorts bool,
	hasClassicLinks bool,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	showContainerLogs bool,
	doEnableMondel bool,
	runTargetAsUser bool,
	keepPerms bool,
	pathPerms map[string]*fsutil.AccessInfo,
	excludePatterns map[string]*fsutil.AccessInfo,
	doExcludeVarLockFiles bool,
	preservePaths map[string]*fsutil.AccessInfo,
	includePaths map[string]*fsutil.AccessInfo,
	includeBins map[string]*fsutil.AccessInfo,
	includeDirBinsList map[string]*fsutil.AccessInfo,
	includeExes map[string]*fsutil.AccessInfo,
	doIncludeShell bool,
	doIncludeWorkdir bool,
	doIncludeCertAll bool,
	doIncludeCertBundles bool,
	doIncludeCertDirs bool,
	doIncludeCertPKAll bool,
	doIncludeCertPKDirs bool,
	doIncludeNew bool,
	doIncludeSSHClient bool,
	doIncludeOSLibsNet bool,
	doIncludeZoneInfo bool,
	selectedNetworks map[string]NetNameInfo,
	//serviceAliases []string,
	doDebug bool,
	logLevel string,
	logFormat string,
	inContainer bool,
	rtaSourcePT bool,
	doObfuscateMetadata bool,
	sensorIPCEndpoint string,
	sensorIPCMode string,
	printState bool,
	appNodejsInspectOpts config.AppNodejsInspectOptions) (*Inspector, error) {

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
		ExplicitVolumeMounts:  explicitVolumeMounts,
		BaseMounts:            baseMounts,
		BaseVolumesFrom:       baseVolumesFrom,
		DoPublishExposedPorts: doPublishExposedPorts,
		HasClassicLinks:       hasClassicLinks,
		Links:                 links,
		EtcHostsMaps:          etcHostsMaps,
		DNSServers:            dnsServers,
		DNSSearchDomains:      dnsSearchDomains,
		DoShowContainerLogs:   showContainerLogs,
		DoEnableMondel:        doEnableMondel,
		RunTargetAsUser:       runTargetAsUser,
		KeepPerms:             keepPerms,
		PathPerms:             pathPerms,
		ExcludePatterns:       excludePatterns,
		DoExcludeVarLockFiles: doExcludeVarLockFiles,
		PreservePaths:         preservePaths,
		IncludePaths:          includePaths,
		IncludeBins:           includeBins,
		IncludeDirBinsList:    includeDirBinsList,
		IncludeExes:           includeExes,
		DoIncludeShell:        doIncludeShell,
		DoIncludeWorkdir:      doIncludeWorkdir,
		DoIncludeCertAll:      doIncludeCertAll,
		DoIncludeCertBundles:  doIncludeCertBundles,
		DoIncludeCertDirs:     doIncludeCertDirs,
		DoIncludeCertPKAll:    doIncludeCertPKAll,
		DoIncludeCertPKDirs:   doIncludeCertPKDirs,
		DoIncludeNew:          doIncludeNew,
		DoIncludeSSHClient:    doIncludeSSHClient,
		DoIncludeOSLibsNet:    doIncludeOSLibsNet,
		DoIncludeZoneInfo:     doIncludeZoneInfo,
		SelectedNetworks:      selectedNetworks,
		DoDebug:               doDebug,
		LogLevel:              logLevel,
		LogFormat:             logFormat,
		PrintState:            printState,
		InContainer:           inContainer,
		RTASourcePT:           rtaSourcePT,
		DoObfuscateMetadata:   doObfuscateMetadata,
		SensorIPCEndpoint:     sensorIPCEndpoint,
		SensorIPCMode:         sensorIPCMode,
		xc:                    xc,
		crOpts:                crOpts,
		portBindings:          portBindings,
		appNodejsInspectOpts:  appNodejsInspectOpts,
	}

	if overrides == nil {
		inspector.FatContainerCmd = BuildStartupCommand(
			imageInspector.ImageInfo.Config.Entrypoint,
			imageInspector.ImageInfo.Config.Cmd,
			imageInspector.ImageInfo.Config.Shell,
			false, nil, false, nil,
		)
	} else {
		inspector.FatContainerCmd = BuildStartupCommand(
			imageInspector.ImageInfo.Config.Entrypoint,
			imageInspector.ImageInfo.Config.Cmd,
			imageInspector.ImageInfo.Config.Shell,
			overrides.ClearEntrypoint,
			overrides.Entrypoint,
			overrides.ClearCmd,
			overrides.Cmd,
		)
	}

	logger.Debugf("FatContainerCmd - %+v", inspector.FatContainerCmd)

	inspector.dockerEventCh = make(chan *dockerapi.APIEvents)
	inspector.dockerEventStopCh = make(chan struct{})

	return inspector, nil
}

// RunContainer starts the container inspector instance execution
func (i *Inspector) RunContainer() error {
	logger := i.logger.WithField("op", "container.Inspector.RunContainer")

	artifactsPath := filepath.Join(i.LocalVolumePath, ArtifactsDir)
	sensorPath := sensor.EnsureLocalBinary(i.xc, i.logger, i.StatePath, i.PrintState)

	allMountsMap := map[string]dockerapi.HostMount{}

	//start with the base mounts (usually come from compose)
	if len(i.BaseMounts) > 0 {
		for _, m := range i.BaseMounts {
			mkey := fmt.Sprintf("%s:%s:%s", m.Type, m.Source, m.Target)
			allMountsMap[mkey] = m
		}
	}

	//var volumeBinds []string
	//then add binds and mounts from the host config param
	if i.crOpts != nil && i.crOpts.HostConfig != nil {
		//volumeBinds = i.crOpts.HostConfig.Binds
		for _, vb := range i.crOpts.HostConfig.Binds {
			parts := strings.Split(vb, ":")
			if len(parts) < 2 {
				logger.Errorf("invalid bind format in crOpts.HostConfig.Binds => %s", vb)
				continue
			}

			vm := dockerapi.HostMount{
				Type:   "bind",
				Source: parts[0],
				Target: parts[1],
			}

			if strings.HasPrefix(vm.Source, "~/") {
				hd, _ := os.UserHomeDir()
				vm.Source = filepath.Join(hd, vm.Source[2:])
			} else if strings.HasPrefix(vm.Source, "./") ||
				strings.HasPrefix(vm.Source, "../") ||
				(vm.Source == "..") ||
				(vm.Source == ".") {
				vm.Source, _ = filepath.Abs(vm.Source)
			}

			if len(parts) == 3 && parts[2] == "ro" {
				vm.ReadOnly = true
			}

			mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
			allMountsMap[mkey] = vm
		}

		for _, vm := range i.crOpts.HostConfig.Mounts {
			mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
			allMountsMap[mkey] = vm
		}
	}

	//configVolumes := i.Overrides.Volumes
	//if configVolumes == nil {
	//	configVolumes = map[string]struct{}{}
	//}

	//then add volumes from overrides
	if i.Overrides != nil && len(i.Overrides.Volumes) > 0 {
		for vol := range i.Overrides.Volumes {
			vm := dockerapi.HostMount{
				Type:   "volume",
				Target: vol,
			}

			mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
			allMountsMap[mkey] = vm
		}
	}

	//now handle the explicit volume mounts
	for _, vol := range i.ExplicitVolumeMounts {
		//mountInfo := fmt.Sprintf("%s:%s:%s", volumeMount.Source, volumeMount.Destination, volumeMount.Options)
		//volumeBinds = append(volumeBinds, mountInfo)

		vm := dockerapi.HostMount{
			Target: vol.Destination,
		}

		if strings.HasPrefix(vol.Source, "/") {
			vm.Source = vol.Source
			vm.Type = "bind"
		} else if strings.HasPrefix(vol.Source, "~/") {
			hd, _ := os.UserHomeDir()
			vm.Source = filepath.Join(hd, vol.Source[2:])
			vm.Type = "bind"
		} else if strings.HasPrefix(vol.Source, "./") ||
			strings.HasPrefix(vol.Source, "../") ||
			(vol.Source == "..") ||
			(vol.Source == ".") {
			vm.Source, _ = filepath.Abs(vol.Source)
			vm.Type = "bind"
		} else {
			//todo: list volumes and check vol.Source instead of defaulting to named volume
			vm.Source = vol.Source
			vm.Type = "volume"
		}

		if vol.Options == "ro" {
			vm.ReadOnly = true
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	}

	var err error
	var volumeName string
	if !i.DoUseLocalMounts {
		volumeName, err = ensureSensorVolume(i.logger, i.APIClient, sensorPath, i.SensorVolumeName)
		errutil.FailOn(err)
	}

	//var artifactsMountInfo string
	if i.DoUseLocalMounts {
		//"%s:/opt/_slim/artifacts"
		//artifactsMountInfo = fmt.Sprintf(ArtifactsMountPat, artifactsPath)
		//volumeBinds = append(volumeBinds, artifactsMountInfo)
		vm := dockerapi.HostMount{
			Type:   "bind",
			Source: artifactsPath,
			Target: app.DefaultArtifactsDirPath,
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	} else {
		//artifactsMountInfo = app.DefaultArtifactsDirPath
		//configVolumes[artifactsMountInfo] = struct{}{}
		vm := dockerapi.HostMount{
			Type:   "volume",
			Target: app.DefaultArtifactsDirPath,
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	}

	//var sensorMountInfo string
	if i.DoUseLocalMounts {
		//sensorMountInfo = fmt.Sprintf(SensorMountPat, sensorPath)
		vm := dockerapi.HostMount{
			Type:     "bind",
			Source:   sensorPath,
			Target:   SensorBinPath,
			ReadOnly: true,
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	} else {
		//sensorMountInfo = fmt.Sprintf(VolumeSensorMountPat, volumeName)
		vm := dockerapi.HostMount{
			Type:     "volume",
			Source:   volumeName,
			Target:   "/opt/_slim/bin",
			ReadOnly: true,
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	}

	//volumeBinds = append(volumeBinds, sensorMountInfo)

	var containerCmd []string
	if i.DoDebug {
		containerCmd = append(containerCmd, "-d")
	}

	if i.LogLevel != "" {
		containerCmd = append(containerCmd, "-log-level", i.LogLevel)
	}

	if i.LogFormat != "" {
		containerCmd = append(containerCmd, "-log-format", i.LogFormat)
	}

	if i.DoEnableMondel {
		containerCmd = append(containerCmd, "-n")
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

	//hostConfig.Binds = volumeBinds
	var mountsList []dockerapi.HostMount
	for _, m := range allMountsMap {
		mountsList = append(mountsList, m)
	}
	hostConfig.Mounts = mountsList

	hostConfig.Privileged = true
	hostConfig.UsernsMode = "host"

	hasSysAdminCap := false
	for _, c := range hostConfig.CapAdd {
		if c == "SYS_ADMIN" {
			hasSysAdminCap = true
		}
	}

	if !hasSysAdminCap {
		hostConfig.CapAdd = append(hostConfig.CapAdd, "SYS_ADMIN")
	}

	containerOptions := dockerapi.CreateContainerOptions{
		Name: i.ContainerName,
		Config: &dockerapi.Config{
			Image:      i.ImageInspector.ImageRef,
			Entrypoint: []string{SensorBinPath},
			Cmd:        containerCmd,
			Env:        i.Overrides.Env,
			Labels:     labels,
			Hostname:   i.Overrides.Hostname,
			WorkingDir: i.Overrides.Workdir,
		},
		HostConfig:       hostConfig,
		NetworkingConfig: &dockerapi.NetworkingConfig{},
	}

	if i.crOpts != nil {
		if i.crOpts.Runtime != "" {
			containerOptions.HostConfig.Runtime = i.crOpts.Runtime
			logger.Debugf("using custom runtime => %s", containerOptions.HostConfig.Runtime)
		}

		if len(i.crOpts.SysctlParams) > 0 {
			containerOptions.HostConfig.Sysctls = i.crOpts.SysctlParams
			logger.Debugf("using sysctl params => %#v", containerOptions.HostConfig.Sysctls)
		}

		if i.crOpts.ShmSize > -1 {
			containerOptions.HostConfig.ShmSize = i.crOpts.ShmSize
			logger.Debugf("using shm-size params => %#v", containerOptions.HostConfig.ShmSize)
		}
	}

	//if len(configVolumes) > 0 {
	//	containerOptions.Config.Volumes = configVolumes
	//}

	runAsUser := i.ImageInspector.ImageInfo.Config.User
	if i.Overrides.User != "" {
		runAsUser = i.Overrides.User
	}

	containerOptions.Config.User = "0:0"

	if runAsUser != "" && strings.ToLower(runAsUser) != "root" {
		//containerOptions.Config.Tty = true
		//containerOptions.Config.OpenStdin = true
		//NOTE:
		//when enabling TTY need to add extra params getting logs
		//or the client.Logs() call will fail with an
		//"Unrecognized input header" error
	}

	hostProbePorts := i.setPorts(&containerOptions)
	commsExposedPorts := containerOptions.Config.ExposedPorts

	if i.Overrides.Network != "" {
		// Non-user defined networks are *probably* a mode, ex. "host".
		if !containertypes.NetworkMode(i.Overrides.Network).IsUserDefined() {
			containerOptions.HostConfig.NetworkMode = i.Overrides.Network
			logger.Debugf("HostConfig.NetworkMode => %v", i.Overrides.Network)
		}

		if containerOptions.NetworkingConfig.EndpointsConfig == nil {
			containerOptions.NetworkingConfig.EndpointsConfig = map[string]*dockerapi.EndpointConfig{}
		}
		containerOptions.NetworkingConfig.EndpointsConfig[i.Overrides.Network] = &dockerapi.EndpointConfig{}
		logger.Debugf("NetworkingConfig.EndpointsConfig => %v", i.Overrides.Network)
	}

	// adding this separately for better visibility...
	if i.HasClassicLinks && len(i.Links) > 0 {
		containerOptions.HostConfig.Links = i.Links
		logger.Debugf("HostConfig.Links => %v", i.Links)
	}

	if len(i.EtcHostsMaps) > 0 {
		containerOptions.HostConfig.ExtraHosts = i.EtcHostsMaps
		logger.Debugf("HostConfig.ExtraHosts => %v", i.EtcHostsMaps)
	}

	if len(i.DNSServers) > 0 {
		containerOptions.HostConfig.DNS = i.DNSServers //for newer versions of Docker
		containerOptions.Config.DNS = i.DNSServers     //for older versions of Docker
		logger.Debugf("HostConfig.DNS/Config.DNS => %v", i.DNSServers)
	}

	if len(i.DNSSearchDomains) > 0 {
		containerOptions.HostConfig.DNSSearch = i.DNSSearchDomains
		logger.Debugf("HostConfig.DNSSearch => %v", i.DNSSearchDomains)
	}

	containerInfo, err := i.APIClient.CreateContainer(containerOptions)
	if err != nil {
		return err
	}
	// note: now need to cleanup the created container if there's an error

	if i.ContainerName != containerInfo.Name {
		logger.Debugf("Container name mismatch expected=%v got=%v", i.ContainerName, containerInfo.Name)
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

	if len(i.SelectedNetworks) > 0 {
		var networkLinks []string
		if !i.HasClassicLinks && len(i.Links) > 0 {
			networkLinks = i.Links
		}

		logger.Debugf("SelectedNetworks => %#v", i.SelectedNetworks)
		for key, netNameInfo := range i.SelectedNetworks {
			err = attachContainerToNetwork(i.logger, i.APIClient, i.ContainerID, netNameInfo, networkLinks)
			if err != nil {
				logger.Debugf("AttachContainerToNetwork(%s,%+v) key=%s error => %#v", i.ContainerID, netNameInfo, key, err)
				return err
			}
		}
	}

	if err := i.APIClient.AddEventListener(i.dockerEventCh); err != nil {
		logger.Debugf("i.APIClient.AddEventListener error => %v", err)
		return err
	}
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
						exitCodeStr, ok := devent.Actor.Attributes["exitCode"]
						if ok && exitCodeStr != "" && exitCodeStr != "0" {
							nonZeroExitCode = true
						}

						if nonZeroExitCode {
							if i.PrintState {
								i.xc.Out.Info("container",
									ovars{
										"status":    "crashed",
										"id":        i.ContainerID,
										"exit.code": exitCodeStr,
									})
							}

							i.ShowContainerLogs()

							if i.PrintState {
								i.xc.Out.State("exited",
									ovars{
										"exit.code":       -999,
										"version":         v.Current(),
										"location.exe":    fsutil.ExeDir(),
										"location.sensor": sensorPath,
										"sensor.filemode": fsutil.FileMode(sensorPath),
										"sensor.volume":   volumeName,
									})
							}
							i.xc.Exit(-999)
						}
					}
				}

			case <-i.dockerEventStopCh:
				logger.Debug("Docker event monitor stopped")
				return
			}
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	go func() {
		select {
		case <-signals:
			_ = i.APIClient.KillContainer(dockerapi.KillContainerOptions{ID: i.ContainerID})
			logger.Fatalf("[SIGMON] received SIGINT, killing container %s", i.ContainerID)
		case <-i.dockerEventStopCh:
			logger.Debug("[SIGMON] Docker event monitor stopped")
			//not killing target because we are going through a graceful shutdown
			//where we sent the StopMonitor and ShutdownSensor ipc commands
		}
	}()

	if err := i.APIClient.StartContainer(i.ContainerID, nil); err != nil {
		return err
	}

	inspectContainerOpts := dockerapi.InspectContainerOptions{ID: i.ContainerID, Size: true}
	if i.ContainerInfo, err = i.APIClient.InspectContainerWithOptions(inspectContainerOpts); err != nil {
		return err
	}

	if i.ContainerInfo.NetworkSettings == nil {
		return fmt.Errorf("slim: error => no network info")
	}

	if hCfg := i.ContainerInfo.HostConfig; hCfg != nil && !i.isHostNetworked() {
		logger.Debugf("container HostConfig.NetworkMode => %s len(ports)=%d",
			hCfg.NetworkMode, len(i.ContainerInfo.NetworkSettings.Ports))

		if len(i.ContainerInfo.NetworkSettings.Ports) < len(commsExposedPorts) {
			return fmt.Errorf("slim: error => missing comms ports")
		}
	}

	logger.Debugf("container NetworkSettings.Ports => %#v", i.ContainerInfo.NetworkSettings.Ports)

	i.setAvailablePorts(hostProbePorts)

	if i.PrintState {
		i.xc.Out.Info("container",
			ovars{
				"status": "running",
				"name":   containerInfo.Name,
				"id":     i.ContainerID,
			})
	}

	if err = i.initContainerChannels(); err != nil {
		return err
	}

	cmd := &command.StartMonitor{
		RTASourcePT: i.RTASourcePT,
		AppName:     i.FatContainerCmd[0],
	}

	if len(i.FatContainerCmd) > 1 {
		cmd.AppArgs = i.FatContainerCmd[1:]
	}

	if len(i.ExcludePatterns) > 0 {
		cmd.Excludes = pathMapKeys(i.ExcludePatterns)
	}

	cmd.ExcludeVarLockFiles = i.DoExcludeVarLockFiles

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

	if len(i.IncludeDirBinsList) > 0 {
		cmd.IncludeDirBinsList = i.IncludeDirBinsList
	}

	if len(i.IncludeExes) > 0 {
		cmd.IncludeExes = pathMapKeys(i.IncludeExes)
	}

	cmd.IncludeShell = i.DoIncludeShell

	if i.DoIncludeWorkdir {
		cmd.IncludeWorkdir = i.ImageInspector.ImageInfo.Config.WorkingDir
	}

	cmd.IncludeCertAll = i.DoIncludeCertAll
	cmd.IncludeCertBundles = i.DoIncludeCertBundles
	cmd.IncludeCertDirs = i.DoIncludeCertDirs
	cmd.IncludeCertPKAll = i.DoIncludeCertPKAll
	cmd.IncludeCertPKDirs = i.DoIncludeCertPKDirs
	cmd.IncludeNew = i.DoIncludeNew
	cmd.IncludeSSHClient = i.DoIncludeSSHClient
	cmd.IncludeOSLibsNet = i.DoIncludeOSLibsNet
	cmd.IncludeZoneInfo = i.DoIncludeZoneInfo

	if runAsUser != "" {
		cmd.AppUser = runAsUser

		if strings.ToLower(runAsUser) != "root" {
			cmd.RunTargetAsUser = i.RunTargetAsUser
		}
	}

	cmd.IncludeAppNextDir = i.appNodejsInspectOpts.NextOpts.IncludeAppDir
	cmd.IncludeAppNextBuildDir = i.appNodejsInspectOpts.NextOpts.IncludeBuildDir
	cmd.IncludeAppNextDistDir = i.appNodejsInspectOpts.NextOpts.IncludeDistDir
	cmd.IncludeAppNextStaticDir = i.appNodejsInspectOpts.NextOpts.IncludeStaticDir
	cmd.IncludeAppNextNodeModulesDir = i.appNodejsInspectOpts.NextOpts.IncludeNodeModulesDir

	cmd.IncludeAppNuxtDir = i.appNodejsInspectOpts.NuxtOpts.IncludeAppDir
	cmd.IncludeAppNuxtBuildDir = i.appNodejsInspectOpts.NuxtOpts.IncludeBuildDir
	cmd.IncludeAppNuxtDistDir = i.appNodejsInspectOpts.NuxtOpts.IncludeDistDir
	cmd.IncludeAppNuxtStaticDir = i.appNodejsInspectOpts.NuxtOpts.IncludeStaticDir
	cmd.IncludeAppNuxtNodeModulesDir = i.appNodejsInspectOpts.NuxtOpts.IncludeNodeModulesDir

	cmd.IncludeNodePackages = i.appNodejsInspectOpts.IncludePackages

	cmd.ObfuscateMetadata = i.DoObfuscateMetadata

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

	// We really need this code block to produce conclusive
	// outcomes. Hence, many retries to prevent (most of) the
	// premature terminations of the master process with the
	// sensor process (in a container) remaining (semi-)started.
	for idx := 0; idx < 16; idx++ {
		evt, err := i.ipcClient.GetEvent()
		if err != nil {
			if os.IsTimeout(err) || err == channel.ErrWaitTimeout {
				if i.PrintState {
					i.xc.Out.Info("event.startmonitor.done",
						ovars{
							"status": "receive.timeout",
						})
				}

				logger.Debug("timeout waiting for the slim container to start...")
				continue
			}

			return err
		}

		if evt == nil || evt.Name == "" {
			logger.Warn("empty event waiting for the slim container to start (trying again)...")
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

			//not returning an error, exiting, need to clean up the container
			i.ShutdownContainer(true)
			i.xc.Exit(-124)
		}

		if evt.Name != event.StartMonitorDone {
			if i.PrintState {
				i.xc.Out.Info("event.startmonitor.done",
					ovars{
						"status": "received.unexpected",
						"data":   jsonutil.ToString(evt),
					})
			}

			return event.ErrUnexpectedEvent
		}
	}

	return ErrStartMonitorTimeout
}

// isHostNetworked returns true if either the created container's network mode is "host"
// or if the Inspector is configured with a network "host".
func (i *Inspector) isHostNetworked() bool {
	if i.ContainerInfo != nil {
		return containertypes.NetworkMode(i.ContainerInfo.HostConfig.NetworkMode).IsHost()
	}
	return containertypes.NetworkMode(i.Overrides.Network).IsHost()
}

const localHostIP = "127.0.0.1"

// setPorts sets all port fields in CreateContainerOptions from user input and defaults.
// Exposed tcp ports are returned as hostProbePorts for containers configured with host networks,
// as those ports are exposed directly by the contained application on the loopback interface,
// and will not be surfaced in network settings.
func (i *Inspector) setPorts(ctrOpts *dockerapi.CreateContainerOptions) (hostProbePorts map[dockerapi.Port][]dockerapi.PortBinding) {
	// This is the minimal set of ports to either expose or directly use.
	commsExposedPorts := map[dockerapi.Port]struct{}{
		i.CmdPort: {},
		i.EvtPort: {},
	}

	//add comms ports to the exposed ports in the container
	if len(i.Overrides.ExposedPorts) > 0 {
		ctrOpts.Config.ExposedPorts = i.Overrides.ExposedPorts
		for k, v := range commsExposedPorts {
			if _, ok := ctrOpts.Config.ExposedPorts[k]; ok {
				i.logger.Errorf("RunContainer: comms port conflict => %v", k)
			}

			ctrOpts.Config.ExposedPorts[k] = v
		}
		i.logger.Debugf("RunContainer: Config.ExposedPorts => %#v", ctrOpts.Config.ExposedPorts)
	} else {
		ctrOpts.Config.ExposedPorts = commsExposedPorts
		i.logger.Debugf("RunContainer: default exposed ports => %#v", ctrOpts.Config.ExposedPorts)
	}

	if len(i.portBindings) > 0 {
		//need to add the IPC ports too
		cmdPort := dockerapi.Port(i.CmdPort)
		evtPort := dockerapi.Port(i.EvtPort)
		if pbInfo, ok := i.portBindings[cmdPort]; ok {
			i.exitIPCPortConflict(pbInfo, "cmd", -126)
		}
		if pbInfo, ok := i.portBindings[evtPort]; ok {
			i.exitIPCPortConflict(pbInfo, "evt", -127)
		}

		i.portBindings[cmdPort] = []dockerapi.PortBinding{{HostPort: cmdPortStrDefault}}
		i.portBindings[evtPort] = []dockerapi.PortBinding{{HostPort: evtPortStrDefault}}

		ctrOpts.HostConfig.PortBindings = i.portBindings
	} else if i.DoPublishExposedPorts {
		portBindings := map[dockerapi.Port][]dockerapi.PortBinding{}

		if i.ImageInspector.ImageInfo.Config != nil {
			for p := range i.ImageInspector.ImageInfo.Config.ExposedPorts {
				portBindings[p] = []dockerapi.PortBinding{{
					HostPort: p.Port(),
				}}
			}
		}

		for p := range ctrOpts.Config.ExposedPorts {
			portBindings[p] = []dockerapi.PortBinding{{
				HostPort: p.Port(),
			}}
		}

		ctrOpts.HostConfig.PortBindings = portBindings
		i.logger.Debugf("RunContainer: publishExposedPorts/portBindings => %+v", portBindings)
	} else {
		ctrOpts.HostConfig.PublishAllPorts = true
		i.logger.Debugf("RunContainer: HostConfig.PublishAllPorts => %v", ctrOpts.HostConfig.PublishAllPorts)
	}

	if i.isHostNetworked() {
		portMap := map[dockerapi.Port][]dockerapi.PortBinding{}
		if ctrOpts.HostConfig.PublishAllPorts {
			for p := range ctrOpts.Config.ExposedPorts {
				portMap[p] = []dockerapi.PortBinding{{HostPort: p.Port()}}
			}
		} else {
			portMap = ctrOpts.HostConfig.PortBindings
		}

		hostProbePorts = map[dockerapi.Port][]dockerapi.PortBinding{}
		for p, pbindings := range portMap {
			if p == i.CmdPort || p == i.EvtPort || p.Proto() != "tcp" {
				continue
			}
			if len(pbindings) == 0 {
				pbindings = []dockerapi.PortBinding{{HostPort: p.Port()}}
			}
			// Ensure all bindings at least have the loopback IP since this interface is
			// where host TCP ports are exposed by default.
			for i, pbinding := range pbindings {
				if pbinding.HostIP == "" {
					pbindings[i].HostIP = localHostIP
				}
			}
			hostProbePorts[p] = pbindings
		}
		i.logger.Debugf("RunContainer: host network loopback ports => %v", hostProbePorts)
	}

	return hostProbePorts
}

func (i *Inspector) setAvailablePorts(hostProbePorts map[dockerapi.Port][]dockerapi.PortBinding) {
	i.AvailablePorts = map[dockerapi.Port]dockerapi.PortBinding{}
	addPorts := func(keys, list []string, pk dockerapi.Port, pbinding []dockerapi.PortBinding) ([]string, []string) {
		if len(pbinding) > 0 {
			keys = append(keys, fmt.Sprintf("%v => %v:%v", pk, pbinding[0].HostIP, pbinding[0].HostPort))
			list = append(list, string(pbinding[0].HostPort))
		} else {
			keys = append(keys, string(pk))
		}
		return keys, list
	}

	// These may be empty if host networking is used.
	var portKeys, portList []string
	for pk, pbinding := range i.ContainerInfo.NetworkSettings.Ports {
		if pk == i.CmdPort || pk == i.EvtPort {
			continue
		}

		if len(pbinding) == 0 {
			i.logger.Debugf("setAvailablePorts: skipping empty port bindings => pk=%v", pk)
			continue
		}

		i.AvailablePorts[pk] = pbinding[0]

		portKeys, portList = addPorts(portKeys, portList, pk, pbinding)
	}

	if i.isHostNetworked() {
		for pk, pbinding := range hostProbePorts {
			if pk == i.CmdPort || pk == i.EvtPort {
				continue
			}

			// The above loop handled this key/binding.
			if b, added := i.AvailablePorts[pk]; added && b == (dockerapi.PortBinding{}) {
				continue
			}

			i.AvailablePorts[pk] = pbinding[0]

			portKeys, portList = addPorts(portKeys, portList, pk, pbinding)
		}
	}

	i.ContainerPortList = strings.Join(portList, ",")
	i.ContainerPortsInfo = strings.Join(portKeys, ",")
	if i.isHostNetworked() {
		const hostMsg = "(ports on host loopback)"
		if i.ContainerPortList != "" {
			i.ContainerPortList = fmt.Sprintf("%s %s", i.ContainerPortList, hostMsg)
		}
		if i.ContainerPortsInfo != "" {
			i.ContainerPortsInfo = fmt.Sprintf("%s %s", i.ContainerPortsInfo, hostMsg)
		}
	}
}

func (i *Inspector) exitIPCPortConflict(port []dockerapi.PortBinding, typ string, code int) {
	i.logger.Errorf("RunContainer: port bindings comms port conflict (%s) = %#v", typ, port)
	if i.PrintState {
		i.xc.Out.Info("sensor.error",
			ovars{
				"message": "port binding ipc port conflict",
				"type":    typ,
			})

		i.xc.Out.State("exited",
			ovars{
				"exit.code": code,
				"component": "container.inspector",
				"version":   v.Current(),
			})
	}

	i.xc.Exit(code)
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
		fmt.Println("slim: container stdout:")
		_, _ = outData.WriteTo(os.Stdout)
		fmt.Println("slim: container stderr:")
		_, _ = errData.WriteTo(os.Stdout)
		fmt.Println("slim: end of container logs =============")
	}
}

// ShutdownContainer terminates the container inspector instance execution
func (i *Inspector) ShutdownContainer(terminateOnly bool) error {
	if i.ContainerID == "" {
		//no container to shutdown...
		return nil
	}

	if i.isDone.IsOn() {
		return nil
	}

	logger := i.logger.WithField("op", "container.Inspector.ShutdownContainer")
	i.isDone.On()

	defer func() {
		i.shutdownContainerChannels()

		if i.DoShowContainerLogs {
			i.ShowContainerLogs()
		}

		err := i.APIClient.StopContainer(i.ContainerID, 9)

		if _, ok := err.(*dockerapi.ContainerNotRunning); ok {
			logger.Info("can't stop the slim container (container is not running)...")
		} else {
			errutil.WarnOn(err)
		}

		removeOption := dockerapi.RemoveContainerOptions{
			ID:            i.ContainerID,
			RemoveVolumes: true,
			Force:         true,
		}

		if err := i.APIClient.RemoveContainer(removeOption); err != nil {
			logger.Infof("error removing container ('%v')... terminating container", err)
			_ = i.APIClient.KillContainer(dockerapi.KillContainerOptions{ID: i.ContainerID})
		}
	}()

	if !terminateOnly {
		if !i.DoUseLocalMounts {
			deleteOrig := true
			if i.DoKeepTmpArtifacts {
				deleteOrig = false
			}

			//copy the container report
			reportLocalPath := filepath.Join(i.LocalVolumePath, ArtifactsDir, ReportArtifactTar)
			reportRemotePath := filepath.Join(app.DefaultArtifactsDirPath, report.DefaultContainerReportFileName)
			err := dockerutil.CopyFromContainer(i.APIClient, i.ContainerID, reportRemotePath, reportLocalPath, true, deleteOrig)
			if err != nil {
				logger.WithError(err).WithField("container", i.ContainerID).Error("dockerutil.CopyFromContainer")
				//can't call errutil.FailOn() because we won't cleanup the target container
				return err
			}

			if i.DoEnableMondel {
				//copy the monitor data event log (if available)
				mondelLocalPath := filepath.Join(i.LocalVolumePath, ArtifactsDir, MondelArtifactTar)
				mondelRemotePath := filepath.Join(app.DefaultArtifactsDirPath, report.DefaultMonDelFileName)
				err = dockerutil.CopyFromContainer(i.APIClient, i.ContainerID, mondelRemotePath, mondelLocalPath, true, deleteOrig)
				if err != nil {
					//not a failure because the log might not be there (just log it)
					logger.WithFields(log.Fields{
						"artifact.type": "mondel",
						"local.path":    mondelLocalPath,
						"remote.path":   mondelRemotePath,
						"err":           err,
					}).Debug("dockerutil.CopyFromContainer")
				}
			}

			/*
				//ALTERNATIVE WAY TO XFER THE FILE ARTIFACTS
				filesOutLocalPath := filepath.Join(i.LocalVolumePath, ArtifactsDir, FileArtifactsArchiveTar)
				filesTarRemotePath := filepath.Join(app.DefaultArtifactsDirPath, fileArtifactsTar)
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
			filesRemotePath := filepath.Join(app.DefaultArtifactsDirPath, app.ArtifactFilesDirName)
			err = dockerutil.CopyFromContainer(i.APIClient, i.ContainerID, filesRemotePath, filesOutLocalPath, false, false)
			if err != nil {
				logger.WithError(err).WithField("container", i.ContainerID).Error("dockerutil.CopyFromContainer")
				//can't call errutil.FailOn() because we won't cleanup the target container
				return err
			}

			//NOTE: possible enhancement (if the original filemode bits still get lost)
			//(alternative to archiving files in the container to preserve filemodes)
			//Rewrite the filemode bits using the data from creport.json,
			//but creport.json also needs to be enhanced to use
			//octal filemodes for the file records
			err = dockerutil.PrepareContainerDataArchive(filesOutLocalPath, fileArtifactsTar, app.ArtifactFilesDirName+"/", deleteOrig)
			if err != nil {
				logger.WithError(err).WithField("container", i.ContainerID).Error("dockerutil.PrepareContainerDataArchive")
				//can't call errutil.FailOn() because we won't cleanup the target container
				return err
			}
		}
	}

	return nil
}

// FinishMonitoring ends the target container monitoring activities
func (i *Inspector) FinishMonitoring() {
	if i.dockerEventStopCh == nil {
		if i.PrintState {
			i.xc.Out.Info("container.inspector",
				ovars{
					"message": "already finished monitoring",
				})
		}

		return
	}

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
	const op = "container.Inspector.initContainerChannels"

	var cn string
	if i.Overrides != nil {
		cn = i.Overrides.Network
	}

	// Top level IP info will not be populated when not using "bridge",
	// which is only set for backwards compatibility.
	//
	// https://github.com/moby/moby/issues/21658#issuecomment-203527083
	ipAddr := i.ContainerInfo.NetworkSettings.IPAddress
	if cn != "" {
		network, found := i.ContainerInfo.NetworkSettings.Networks[cn]
		errutil.FailWhen(!found, fmt.Sprintf("slim: error => expected NetworkSettings.Networks to contain %s: %v",
			cn, i.ContainerInfo.NetworkSettings.Networks))

		ipAddr = network.IPAddress
	}
	// If running in host mode, no IP may be set but the contained application
	// is listening on the exposed ports on localhost.
	if ipAddr == "" && i.isHostNetworked() {
		ipAddr = localHostIP
	}

	if i.PrintState {
		i.xc.Out.Info("container",
			ovars{
				"message": "obtained IP address",
				"ip":      ipAddr,
			})
	}

	var ipcMode string
	switch i.SensorIPCMode {
	case SensorIPCModeDirect, SensorIPCModeProxy:
		ipcMode = i.SensorIPCMode
	default:
		if i.InContainer || i.isHostNetworked() {
			ipcMode = SensorIPCModeDirect
		} else {
			ipcMode = SensorIPCModeProxy
		}
	}

	var cmdPort, evtPort string
	switch ipcMode {
	case SensorIPCModeDirect:
		i.TargetHost = ipAddr
		cmdPort = i.CmdPort.Port()
		evtPort = i.EvtPort.Port()
	case SensorIPCModeProxy:
		i.DockerHostIP = dockerhost.GetIP(i.APIClient)
		i.TargetHost = i.DockerHostIP
		cmdPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.CmdPort]
		evtPortBindings := i.ContainerInfo.NetworkSettings.Ports[i.EvtPort]
		cmdPort = cmdPortBindings[0].HostPort
		evtPort = evtPortBindings[0].HostPort
	}
	i.SensorIPCMode = ipcMode

	if i.SensorIPCEndpoint != "" {
		i.TargetHost = i.SensorIPCEndpoint
	}

	i.logger.WithFields(log.Fields{
		"op":                op,
		"in.container":      i.InContainer,
		"container.network": cn,
		"ipc.mode":          ipcMode,
		"target":            i.TargetHost,
		"port.cmd":          cmdPort,
		"port.evt":          evtPort,
	}).Debugf("target.container.ipc.connect")

	ipcClient, err := ipc.NewClient(i.TargetHost, cmdPort, evtPort, sensor.DefaultConnectWait)
	if err != nil {
		return err
	}

	i.ipcClient = ipcClient
	return nil
}

func (i *Inspector) shutdownContainerChannels() {
	const op = "container.Inspector.shutdownContainerChannels"
	if i.ipcClient != nil {
		if err := i.ipcClient.Stop(); err != nil {
			i.logger.WithFields(log.Fields{
				"op":    op,
				"error": err,
			}).Debug("shutting down channels")
		}
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
		//TODO: need to check if the volume has the sensor (otherwise delete and recreate)
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

func attachContainerToNetwork(
	logger *log.Entry,
	apiClient *dockerapi.Client,
	containerID string,
	netNameInfo NetNameInfo,
	networkLinks []string) error {
	//network names seem to work ok (no need to use need network IDs)
	options := dockerapi.NetworkConnectionOptions{
		Container: containerID,
		EndpointConfig: &dockerapi.EndpointConfig{
			Aliases: netNameInfo.Aliases,
		},
	}

	if len(networkLinks) > 0 {
		options.EndpointConfig.Links = networkLinks
	}

	if err := apiClient.ConnectNetwork(netNameInfo.FullName, options); err != nil {
		logger.Debugf("attachContainerToNetwork(%s,%s,%s): container network connect error - %v",
			containerID, netNameInfo.FullName, networkLinks, err)
		return err
	}

	return nil
}
