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

	"github.com/docker-slim/docker-slim/pkg/aflag"
	"github.com/docker-slim/docker-slim/pkg/app"
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

	containertypes "github.com/docker/docker/api/types/container"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// Container inspector constants
const (
	SensorIPCModeDirect     = "direct"
	SensorIPCModeProxy      = "proxy"
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

type ovars = app.OutVars

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

type NetNameInfo struct {
	Name     string
	FullName string
	Aliases  []string
}

// TODO(estroz): move all fields configured only after RunContainer is called
// to a InspectorRunResponse struct returned by RunContainer.

// Inspector is a container execution inspector
type Inspector struct {
	ContainerInfo                  *dockerapi.Container
	ContainerPortsInfo             string
	ContainerPortList              string
	AvailablePorts                 map[dockerapi.Port]dockerapi.PortBinding // Ports found to be available for probing.
	ContainerID                    string
	ContainerName                  string
	FatContainerCmd                []string
	LocalVolumePath                string
	DoUseLocalMounts               bool
	DoIncludeAppNuxtDir            bool
	DoIncludeAppNuxtBuildDir       bool
	DoIncludeAppNuxtDistDir        bool
	DoIncludeAppNuxtStaticDir      bool
	DoIncludeAppNuxtNodeModulesDir bool
	DoIncludeAppNextDir            bool
	DoIncludeAppNextBuildDir       bool
	DoIncludeAppNextDistDir        bool
	DoIncludeAppNextStaticDir      bool
	DoIncludeAppNextNodeModulesDir bool
	SensorVolumeName               string
	DoKeepTmpArtifacts             bool
	StatePath                      string
	CmdPort                        dockerapi.Port
	EvtPort                        dockerapi.Port
	DockerHostIP                   string
	ImageInspector                 *image.Inspector
	APIClient                      *dockerapi.Client
	Overrides                      *config.ContainerOverrides
	ExplicitVolumeMounts           map[string]config.VolumeMount
	BaseMounts                     []dockerapi.HostMount
	BaseVolumesFrom                []string
	DoPublishExposedPorts          bool
	HasClassicLinks                bool
	Links                          []string
	EtcHostsMaps                   []string
	DNSServers                     []string
	DNSSearchDomains               []string
	DoShowContainerLogs            bool
	RunTargetAsUser                bool
	KeepPerms                      bool
	PathPerms                      map[string]*fsutil.AccessInfo
	ExcludePatterns                map[string]*fsutil.AccessInfo
	PreservePaths                  map[string]*fsutil.AccessInfo
	IncludePaths                   map[string]*fsutil.AccessInfo
	IncludeBins                    map[string]*fsutil.AccessInfo
	IncludeExes                    map[string]*fsutil.AccessInfo
	IncludeNodePackages            []string
	DoIncludeShell                 bool
	DoIncludeCertAll               bool
	DoIncludeCertBundles           bool
	DoIncludeCertDirs              bool
	DoIncludeCertPKAll             bool
	DoIncludeCertPKDirs            bool
	DoIncludeNew                   bool
	SelectedNetworks               map[string]NetNameInfo
	DoDebug                        bool
	LogLevel                       string
	LogFormat                      string
	PrintState                     bool
	PrintPrefix                    string
	InContainer                    bool
	RTASourcePT                    bool
	SensorIPCEndpoint              string
	SensorIPCMode                  string
	TargetHost                     string
	dockerEventCh                  chan *dockerapi.APIEvents
	dockerEventStopCh              chan struct{}
	isDone                         aflag.Type
	ipcClient                      *ipc.Client
	logger                         *log.Entry
	xc                             *app.ExecutionContext
	crOpts                         *config.ContainerRunOptions
	portBindings                   map[dockerapi.Port][]dockerapi.PortBinding
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
	doIncludeAppNuxtDir bool,
	doIncludeAppNuxtBuildDir bool,
	doIncludeAppNuxtDistDir bool,
	doIncludeAppNuxtStaticDir bool,
	doIncludeAppNuxtNodeModulesDir bool,
	doIncludeAppNextDir bool,
	doIncludeAppNextBuildDir bool,
	doIncludeAppNextDistDir bool,
	doIncludeAppNextStaticDir bool,
	doIncludeAppNextNodeModulesDir bool,
	includeNodePackages []string,
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
	runTargetAsUser bool,
	showContainerLogs bool,
	keepPerms bool,
	pathPerms map[string]*fsutil.AccessInfo,
	excludePatterns map[string]*fsutil.AccessInfo,
	preservePaths map[string]*fsutil.AccessInfo,
	includePaths map[string]*fsutil.AccessInfo,
	includeBins map[string]*fsutil.AccessInfo,
	includeExes map[string]*fsutil.AccessInfo,
	doIncludeShell bool,
	doIncludeCertAll bool,
	doIncludeCertBundles bool,
	doIncludeCertDirs bool,
	doIncludeCertPKAll bool,
	doIncludeCertPKDirs bool,
	doIncludeNew bool,
	selectedNetworks map[string]NetNameInfo,
	//serviceAliases []string,
	doDebug bool,
	logLevel string,
	logFormat string,
	inContainer bool,
	rtaSourcePT bool,
	sensorIPCEndpoint string,
	sensorIPCMode string,
	printState bool,
	printPrefix string) (*Inspector, error) {

	logger = logger.WithFields(log.Fields{"component": "container.inspector"})
	inspector := &Inspector{
		logger:                         logger,
		StatePath:                      statePath,
		LocalVolumePath:                localVolumePath,
		DoUseLocalMounts:               doUseLocalMounts,
		DoIncludeAppNuxtDir:            doIncludeAppNuxtDir,
		DoIncludeAppNuxtBuildDir:       doIncludeAppNuxtBuildDir,
		DoIncludeAppNuxtDistDir:        doIncludeAppNuxtDistDir,
		DoIncludeAppNuxtStaticDir:      doIncludeAppNuxtStaticDir,
		DoIncludeAppNuxtNodeModulesDir: doIncludeAppNuxtNodeModulesDir,
		DoIncludeAppNextDir:            doIncludeAppNextDir,
		DoIncludeAppNextBuildDir:       doIncludeAppNextBuildDir,
		DoIncludeAppNextDistDir:        doIncludeAppNextDistDir,
		DoIncludeAppNextStaticDir:      doIncludeAppNextStaticDir,
		DoIncludeAppNextNodeModulesDir: doIncludeAppNextNodeModulesDir,
		IncludeNodePackages:            includeNodePackages,
		SensorVolumeName:               sensorVolumeName,
		DoKeepTmpArtifacts:             doKeepTmpArtifacts,
		CmdPort:                        cmdPortSpecDefault,
		EvtPort:                        evtPortSpecDefault,
		ImageInspector:                 imageInspector,
		APIClient:                      client,
		Overrides:                      overrides,
		ExplicitVolumeMounts:           explicitVolumeMounts,
		BaseMounts:                     baseMounts,
		BaseVolumesFrom:                baseVolumesFrom,
		DoPublishExposedPorts:          doPublishExposedPorts,
		HasClassicLinks:                hasClassicLinks,
		Links:                          links,
		EtcHostsMaps:                   etcHostsMaps,
		DNSServers:                     dnsServers,
		DNSSearchDomains:               dnsSearchDomains,
		DoShowContainerLogs:            showContainerLogs,
		RunTargetAsUser:                runTargetAsUser,
		KeepPerms:                      keepPerms,
		PathPerms:                      pathPerms,
		ExcludePatterns:                excludePatterns,
		PreservePaths:                  preservePaths,
		IncludePaths:                   includePaths,
		IncludeBins:                    includeBins,
		IncludeExes:                    includeExes,
		DoIncludeShell:                 doIncludeShell,
		DoIncludeCertAll:               doIncludeCertAll,
		DoIncludeCertBundles:           doIncludeCertBundles,
		DoIncludeCertDirs:              doIncludeCertDirs,
		DoIncludeCertPKAll:             doIncludeCertPKAll,
		DoIncludeCertPKDirs:            doIncludeCertPKDirs,
		DoIncludeNew:                   doIncludeNew,
		SelectedNetworks:               selectedNetworks,
		DoDebug:                        doDebug,
		LogLevel:                       logLevel,
		LogFormat:                      logFormat,
		PrintState:                     printState,
		PrintPrefix:                    printPrefix,
		InContainer:                    inContainer,
		RTASourcePT:                    rtaSourcePT,
		SensorIPCEndpoint:              sensorIPCEndpoint,
		SensorIPCMode:                  sensorIPCMode,
		xc:                             xc,
		crOpts:                         crOpts,
		portBindings:                   portBindings,
	}

	if overrides == nil {
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Entrypoint...)
		inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Cmd...)
	} else {
		if len(overrides.Entrypoint) > 0 || overrides.ClearEntrypoint {
			inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Entrypoint...)

			if len(overrides.Cmd) > 0 || overrides.ClearCmd {
				inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Cmd...)
			}
			//note: not using CMD from image if there's an override for ENTRYPOINT
		} else {
			inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Entrypoint...)

			if len(overrides.Cmd) > 0 || overrides.ClearCmd {
				inspector.FatContainerCmd = append(inspector.FatContainerCmd, overrides.Cmd...)
			} else {
				inspector.FatContainerCmd = append(inspector.FatContainerCmd, imageInspector.ImageInfo.Config.Cmd...)
			}
		}
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

		i.xc.Exit(-125)
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
				i.logger.Errorf("RunContainer: invalid bind format in crOpts.HostConfig.Binds => %s", vb)
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
		//"%s:/opt/dockerslim/artifacts"
		//artifactsMountInfo = fmt.Sprintf(ArtifactsMountPat, artifactsPath)
		//volumeBinds = append(volumeBinds, artifactsMountInfo)
		vm := dockerapi.HostMount{
			Type:   "bind",
			Source: artifactsPath,
			Target: "/opt/dockerslim/artifacts",
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	} else {
		//artifactsMountInfo = ArtifactsVolumePath
		//configVolumes[artifactsMountInfo] = struct{}{}
		vm := dockerapi.HostMount{
			Type:   "volume",
			Target: ArtifactsVolumePath,
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
			Target:   "/opt/dockerslim/bin/docker-slim-sensor",
			ReadOnly: true,
		}

		mkey := fmt.Sprintf("%s:%s:%s", vm.Type, vm.Source, vm.Target)
		allMountsMap[mkey] = vm
	} else {
		//sensorMountInfo = fmt.Sprintf(VolumeSensorMountPat, volumeName)
		vm := dockerapi.HostMount{
			Type:     "volume",
			Source:   volumeName,
			Target:   "/opt/dockerslim/bin",
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
			i.logger.Debugf("RunContainer: HostConfig.NetworkMode => %v", i.Overrides.Network)
		}

		if containerOptions.NetworkingConfig.EndpointsConfig == nil {
			containerOptions.NetworkingConfig.EndpointsConfig = map[string]*dockerapi.EndpointConfig{}
		}
		containerOptions.NetworkingConfig.EndpointsConfig[i.Overrides.Network] = &dockerapi.EndpointConfig{}
		i.logger.Debugf("RunContainer: NetworkingConfig.EndpointsConfig => %v", i.Overrides.Network)
	}

	// adding this separately for better visibility...
	if i.HasClassicLinks && len(i.Links) > 0 {
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

	if len(i.SelectedNetworks) > 0 {
		var networkLinks []string
		if !i.HasClassicLinks && len(i.Links) > 0 {
			networkLinks = i.Links
		}

		i.logger.Debugf("RunContainer: SelectedNetworks => %#v", i.SelectedNetworks)
		for key, netNameInfo := range i.SelectedNetworks {
			err = attachContainerToNetwork(i.logger, i.APIClient, i.ContainerID, netNameInfo, networkLinks)
			if err != nil {
				i.logger.Debugf("RunContainer: AttachContainerToNetwork(%s,%+v) key=%s error => %#v", i.ContainerID, netNameInfo, key, err)
				return err
			}
		}
	}

	if err := i.APIClient.AddEventListener(i.dockerEventCh); err != nil {
		i.logger.Debugf("RunContainer: i.APIClient.AddEventListener error => %v", err)
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
							i.xc.Exit(-123)
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

	inspectContainerOpts := dockerapi.InspectContainerOptions{ID: i.ContainerID, Size: true}
	if i.ContainerInfo, err = i.APIClient.InspectContainerWithOptions(inspectContainerOpts); err != nil {
		return err
	}

	errutil.FailWhen(i.ContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")

	if hCfg := i.ContainerInfo.HostConfig; hCfg != nil && !i.isHostNetworked() {
		i.logger.Debugf("RunContainer: container HostConfig.NetworkMode => %s len(ports)=%d",
			hCfg.NetworkMode, len(i.ContainerInfo.NetworkSettings.Ports))
		errutil.FailWhen(len(i.ContainerInfo.NetworkSettings.Ports) < len(commsExposedPorts), "docker-slim: error => missing comms ports")
	}

	i.logger.Debugf("RunContainer: container NetworkSettings.Ports => %#v", i.ContainerInfo.NetworkSettings.Ports)

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
	cmd.IncludeCertAll = i.DoIncludeCertAll
	cmd.IncludeCertBundles = i.DoIncludeCertBundles
	cmd.IncludeCertDirs = i.DoIncludeCertDirs
	cmd.IncludeCertPKAll = i.DoIncludeCertPKAll
	cmd.IncludeCertPKDirs = i.DoIncludeCertPKDirs
	cmd.IncludeNew = i.DoIncludeNew

	if runAsUser != "" {
		cmd.AppUser = runAsUser

		if strings.ToLower(runAsUser) != "root" {
			cmd.RunTargetAsUser = i.RunTargetAsUser
		}
	}

	cmd.IncludeAppNuxtDir = i.DoIncludeAppNuxtDir
	cmd.IncludeAppNuxtBuildDir = i.DoIncludeAppNuxtBuildDir
	cmd.IncludeAppNuxtDistDir = i.DoIncludeAppNuxtDistDir
	cmd.IncludeAppNuxtStaticDir = i.DoIncludeAppNuxtStaticDir
	cmd.IncludeAppNuxtNodeModulesDir = i.DoIncludeAppNuxtNodeModulesDir

	cmd.IncludeAppNextDir = i.DoIncludeAppNextDir
	cmd.IncludeAppNextBuildDir = i.DoIncludeAppNextBuildDir
	cmd.IncludeAppNextDistDir = i.DoIncludeAppNextDistDir
	cmd.IncludeAppNextStaticDir = i.DoIncludeAppNextStaticDir
	cmd.IncludeAppNextNodeModulesDir = i.DoIncludeAppNextNodeModulesDir

	cmd.IncludeNodePackages = i.IncludeNodePackages

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

				i.logger.Debug("timeout waiting for the docker-slim container to start...")
				continue
			}

			return err
		}

		if evt == nil || evt.Name == "" {
			i.logger.Warn("empty event waiting for the docker-slim container to start (trying again)...")
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

			i.xc.Exit(-124)
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
		fmt.Println("docker-slim: container stdout:")
		_, _ = outData.WriteTo(os.Stdout)
		fmt.Println("docker-slim: container stderr:")
		_, _ = errData.WriteTo(os.Stdout)
		fmt.Println("docker-slim: end of container logs =============")
	}
}

// ShutdownContainer terminates the container inspector instance execution
func (i *Inspector) ShutdownContainer() error {
	if i.isDone.IsOn() {
		return nil
	}

	i.isDone.On()
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
		errutil.FailWhen(!found, fmt.Sprintf("docker-slim: error => expected NetworkSettings.Networks to contain %s: %v",
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

	ipcClient, err := ipc.NewClient(i.TargetHost, cmdPort, evtPort, defaultConnectWait)
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
