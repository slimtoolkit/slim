package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

var ErrNoServiceImage = errors.New("no service image")

type ServiceError struct {
	Service string
	Op      string
	Info    string
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("compose.ServiceError: service=%s op=%s info='%s'",
		e.Service, e.Op, e.Info)
}

type ovars = app.OutVars

type ExecutionState string

const (
	XSNone        ExecutionState = "xs.none"
	XSCreated                    = "xs.created"
	XSStarted                    = "xs.started"
	XSStopping                   = "xs.stopping"
	XSStopped                    = "xs.stopped"
	XSRemoved                    = "xs.removed"
	XSExited                     = "xs.exited"
	XSExitedCrash                = "xs.exited.crash"
	XSError                      = "xs.error"
)

type ExecutionEvent string

const (
	XECreated     ExecutionEvent = "xe.container.created"
	XEStarted                    = "xe.container.started"
	XEStopping                   = "xe.container.stopping"
	XEStopped                    = "xe.container.stopped"
	XERemoved                    = "xe.container.removed"
	XEExited                     = "xe.container.exited"
	XEExitedCrash                = "xe.container.exited.crash"
	XEAPIError                   = "xe.api.error"
	XEInterrupt                  = "xe.interrupt"
)

type ExecutionEventInfo struct {
	Event ExecutionEvent
	Data  map[string]string
}

const (
	ComposeVerUnknown = 0
	ComposeVerOne     = 1
	ComposeVerTwo     = 2
	ComposeVerThree   = 3
)

const (
	ComposeVerOneStr   = "1"
	ComposeVerTwoStr   = "2"
	ComposeVerThreeStr = "3"
)

type ExecutionOptions struct {
	SvcStartWait int
}

type Execution struct {
	*ConfigInfo
	State             ExecutionState
	Selectors         *ServiceSelectors
	BuildImages       bool
	PullImages        bool
	OwnAllResources   bool
	AllServiceNames   map[string]struct{}
	AllServices       map[string]*ServiceInfo
	AllNetworks       map[string]*NetworkInfo
	PendingServices   map[string]struct{}
	RunningServices   map[string]*RunningService
	ActiveVolumes     map[string]*ActiveVolume
	ActiveNetworks    map[string]*ActiveNetwork
	StopTimeout       uint
	ContainerProbeSvc string

	options    *ExecutionOptions
	eventCh    chan *ExecutionEventInfo
	printState bool
	xc         *app.ExecutionContext
	logger     *log.Entry
	apiClient  *dockerapi.Client
}

type ConfigInfo struct {
	BaseComposeDir string
	Version        uint
	FullVersion    string
	Project        *types.Project
	Raw            map[string]interface{}
	RawList        []map[string]interface{}
}

type ServiceSelectors struct {
	Includes       map[string]struct{}
	Excludes       map[string]struct{}
	ServiceAllDeps string
}

func NewServiceSelectors(serviceAllDeps string,
	includes []string,
	excludes []string) *ServiceSelectors {
	selectors := &ServiceSelectors{
		Includes:       map[string]struct{}{},
		Excludes:       map[string]struct{}{},
		ServiceAllDeps: serviceAllDeps,
	}

	for _, val := range includes {
		selectors.Includes[val] = struct{}{}
	}

	for _, val := range excludes {
		selectors.Excludes[val] = struct{}{}
	}

	return selectors
}

type ServiceInfo struct {
	Selected        bool
	ShortName       string
	Name            string
	ContainerName   string
	AllDependencies []string
	Config          types.ServiceConfig
}

type NetworkInfo struct {
	Name   string
	Config types.NetworkConfig
}

type RunningService struct {
	Name          string
	ID            string
	ContainerName string
}

type ActiveVolume struct {
	ShortName string
	FullName  string
	ID        string
}

type ActiveNetwork struct {
	Name    string //full network name
	ID      string
	Created bool
}

const defaultStopTimeout = 7 //7 seconds

func NewConfigInfo(
	composeFiles []string,
	projectName string,
	workingDir string,
	envVars []string,
	environmentNoHost bool,
) (*ConfigInfo, error) {
	//not supporting compose profiles for now
	cv := &ConfigInfo{}

	var pcConfigFiles []types.ConfigFile
	for idx, composeFile := range composeFiles {
		fullComposeFilePath, err := filepath.Abs(composeFile)
		if err != nil {
			panic(err)
		}

		if idx == 0 {
			baseComposeDir := filepath.Dir(fullComposeFilePath)
			if projectName == "" {
				//default project name to the dir name for compose file
				projectName = filepath.Base(baseComposeDir)
			}

			if workingDir == "" {
				//all paths in the compose files are relative
				//to the base path of the first compose file
				//unless there's an explicitly provided working directory
				workingDir = baseComposeDir
			}

			cv.BaseComposeDir = workingDir
		}

		b, err := os.ReadFile(fullComposeFilePath)
		if err != nil {
			return nil, err
		}

		rawConfig, err := loader.ParseYAML(b)
		if err != nil {
			return nil, err
		}

		if idx == 0 {
			cv.Raw = rawConfig
		}

		cv.RawList = append(cv.RawList, rawConfig)

		cf := types.ConfigFile{
			Filename: composeFile,
			Content:  b, //pass raw bytes, so loader parses and interpolates
		}

		pcConfigFiles = append(pcConfigFiles, cf)
	}

	projectConfig := types.ConfigDetails{
		WorkingDir:  workingDir,
		ConfigFiles: pcConfigFiles,
	}

	if len(envVars) > 0 {
		projectConfig.Environment = map[string]string{}
		for _, evar := range envVars {
			parts := strings.SplitN(evar, "=", 2)
			if len(parts) == 2 {
				projectConfig.Environment[parts[0]] = parts[1]
			}
		}
	}

	if !environmentNoHost {
		if projectConfig.Environment == nil {
			projectConfig.Environment = map[string]string{}
		}

		//host env vars override explicit vars
		for _, evar := range os.Environ() {
			parts := strings.SplitN(evar, "=", 2)
			if len(parts) == 2 {
				projectConfig.Environment[parts[0]] = parts[1]
			}
		}
	}

	project, err := loader.Load(projectConfig, withProjectName(projectName))
	if err != nil {
		return nil, err
	}

	if project == nil {
		return nil, fmt.Errorf("no project info")
	}

	cv.Project = project

	return cv, nil
}

func NewExecution(
	xc *app.ExecutionContext,
	logger *log.Entry,
	apiClient *dockerapi.Client,
	composeFiles []string,
	selectors *ServiceSelectors,
	projectName string,
	workingDir string,
	envVars []string,
	environmentNoHost bool,
	containerProbeComposeSvc string,
	buildImages bool,
	pullImages bool,
	pullExcludes []string,
	ownAllResources bool,
	options *ExecutionOptions,
	eventCh chan *ExecutionEventInfo,
	printState bool) (*Execution, error) {
	if logger != nil {
		logger = logger.WithFields(log.Fields{"com": "compose.execution"})
	}

	configInfo, err := NewConfigInfo(composeFiles,
		projectName,
		workingDir,
		envVars,
		environmentNoHost)
	if err != nil {
		return nil, err
	}

	//todo: explicitly check for invalid/unknown service dependencies
	//todo: have an option to ignore/cleanup invalid/unknown service dependencies

	//not supporting compose profiles for now
	exe := &Execution{
		ConfigInfo:        configInfo,
		State:             XSNone,
		Selectors:         selectors,
		OwnAllResources:   ownAllResources,
		BuildImages:       buildImages,
		PullImages:        pullImages,
		AllServiceNames:   map[string]struct{}{},
		AllServices:       map[string]*ServiceInfo{},
		AllNetworks:       map[string]*NetworkInfo{},
		PendingServices:   map[string]struct{}{},
		RunningServices:   map[string]*RunningService{},
		ActiveVolumes:     map[string]*ActiveVolume{},
		ActiveNetworks:    map[string]*ActiveNetwork{},
		ContainerProbeSvc: containerProbeComposeSvc,
		StopTimeout:       defaultStopTimeout,
		apiClient:         apiClient,
		options:           options,
		eventCh:           eventCh,
		printState:        printState,
		xc:                xc,
		logger:            logger,
	}

	exe.initVersion()

	err = exe.initServices()
	if err != nil {
		return nil, err
	}
	exe.initNetworks()

	return exe, nil
}

type LoaderOptionsFn func(opts *loader.Options)

func withProjectName(name string) LoaderOptionsFn {
	return func(opts *loader.Options) {
		opts.Name = name
	}
}

func withResolvePaths(resolve bool) LoaderOptionsFn {
	return func(opts *loader.Options) {
		opts.ResolvePaths = resolve
	}
}

func withInterpolation(interpolation bool) LoaderOptionsFn {
	return func(opts *loader.Options) {
		opts.SkipInterpolation = !interpolation
	}
}

//loader.WithDiscardEnvFiles
//loader.WithSkipValidation

func (ref *Execution) ProjectName() string {
	return ref.Project.Name
}

func (ref *Execution) ProjectWorkingDir() string {
	return ref.Project.WorkingDir
}

func (ref *Execution) Service(name string) *ServiceInfo {
	return ref.AllServices[name]
}

func (ref *Execution) SelectedHaveImages() bool {
	for _, svc := range ref.AllServices {
		if svc.Selected && svc.Config.Image == "" {
			return false
		}
	}

	return true
}

type NetNameInfo struct {
	FullName string
	Aliases  []string
}

func (ref *Execution) ActiveServiceNetworks(svcName string) map[string]NetNameInfo {
	networks := map[string]NetNameInfo{}

	if svc, found := ref.AllServices[svcName]; found {
		for netKey, netConfig := range svc.Config.Networks {
			if netInfo, found := ref.AllNetworks[netKey]; found && netInfo != nil {
				info := NetNameInfo{
					FullName: netInfo.Name,
				}

				if netConfig != nil {
					info.Aliases = netConfig.Aliases
				}

				networks[netKey] = info
			}
		}
	}

	return networks
}

func (ref *Execution) ActiveNetworkNames() map[string]string {
	networks := map[string]string{}

	for netKey, netInfo := range ref.AllNetworks {
		if netInfo != nil {
			networks[netKey] = netInfo.Name
		}
	}

	return networks
}

func (ref *Execution) initServices() error {
	for _, service := range ref.Project.Services {
		name := service.Name
		info := &ServiceInfo{
			ShortName: name,
			Name:      fullServiceName(ref.Project.Name, name),
			Config:    service,
		}

		if err := ref.Project.WithServices([]string{name},
			func(svc types.ServiceConfig) error {
				if svc.Name != name {
					info.AllDependencies = append(info.AllDependencies, svc.Name)
				}
				return nil
			}); err != nil {
			return err
		}

		if ref.Selectors != nil {
			if ref.Selectors.ServiceAllDeps != "" {
				if ref.Selectors.ServiceAllDeps == name {
					info.Selected = false
					if len(info.AllDependencies) > 0 {
						ref.Selectors.Includes = map[string]struct{}{}
						for _, sname := range info.AllDependencies {
							ref.Selectors.Includes[sname] = struct{}{}
						}
					}
				}
			}
		} else {
			info.Selected = true
		}

		ref.AllServices[name] = info
		ref.AllServiceNames[name] = struct{}{}
	}

	if ref.Selectors != nil {
		for name := range ref.AllServices {
			if len(ref.Selectors.Includes) > 0 {
				if _, found := ref.Selectors.Includes[name]; found {
					ref.AllServices[name].Selected = true
				}
			} else if len(ref.Selectors.Excludes) > 0 {
				if _, found := ref.Selectors.Excludes[name]; !found {
					ref.AllServices[name].Selected = true
				}
			} else {
				ref.AllServices[name].Selected = true
			}
		}
	}

	if ref.ContainerProbeSvc != "" {
		ref.AllServices[ref.ContainerProbeSvc].Selected = true
		ref.Selectors.Includes[ref.ContainerProbeSvc] = struct{}{}
	}

	ref.logger.Debug("Execution.initServices: checking ref.AllServices[x].Selected")
	for shortName, svc := range ref.AllServices {
		ref.logger.Debugf("sname=%s/%s name=%s SELECTED?=%v\n",
			shortName,
			svc.ShortName,
			svc.Name,
			svc.Selected)
	}

	return nil
}

func (ref *Execution) initNetworks() {
	for key, network := range ref.Project.Networks {
		ref.AllNetworks[key] = &NetworkInfo{
			Name:   fullNetworkName(ref.Project.Name, key, network.Name),
			Config: network,
		}
	}

	if _, ok := ref.AllNetworks[defaultNetName]; !ok {
		ref.AllNetworks[defaultNetName] = &NetworkInfo{
			Name:   fullNetworkName(ref.Project.Name, defaultNetName, defaultNetName),
			Config: types.NetworkConfig{},
		}
	}
}

func (ref *Execution) initVersion() {
	ref.Version = ComposeVerUnknown
	fullVersion, ok := ref.Raw["version"].(string)
	if !ok {
		return
	}

	ref.FullVersion = fullVersion
	if ref.FullVersion != "" {
		switch ref.FullVersion[0:1] {
		case ComposeVerOneStr:
			ref.Version = ComposeVerOne
		case ComposeVerTwoStr:
			ref.Version = ComposeVerTwo
		case ComposeVerThreeStr:
			ref.Version = ComposeVerThree
		}
	}
}

func fullServiceName(project, service string) string {
	//todo: lower case
	return fmt.Sprintf("%s_%s", project, service)
}

func fullNetworkName(project, networkKey, networkName string) string {
	//todo: lower case
	fullName := fmt.Sprintf("%s_%s", project, networkKey)
	if networkName != "" && networkName != defaultNetName {
		fullName = networkName
	}
	return fullName
}

func (ref *Execution) Prepare() error {
	ref.logger.Debug("Execution.Prepare")

	if err := ref.PrepareServices(); err != nil {
		return err
	}

	if err := ref.CreateNetworks(); err != nil {
		return err
	}

	if err := ref.CreateVolumes(); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) PrepareServices() error {
	errCh := make(chan error, len(ref.AllServices))
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	for name := range ref.AllServices {
		wg.Add(1)
		go func(svcName string) {
			defer wg.Done()
			ref.logger.Debugf("Execution.Prepare: service=%s", svcName)
			if err := ref.PrepareService(ctx, svcName); err != nil {
				ref.logger.Debugf("Execution.Prepare: PrepareService(%s) error - %v", svcName, err)
				//errCh <- fmt.Errorf("error preparing service - %s (%s)", svcName, err.Error())
				errCh <- &ServiceError{Service: svcName, Op: "Execution.PrepareService", Info: err.Error()}
				ref.logger.Debugf("Execution.Prepare: PrepareService(%s) - CANCEL ALL PREPARE SVC", svcName)
				cancel()
			}
		}(name)
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}

	return nil
}

func (ref *Execution) PrepareService(ctx context.Context, name string) error {
	ref.logger.Debugf("Execution.PrepareService(%s)", name)
	serviceInfo, ok := ref.AllServices[name]
	if !ok {
		return fmt.Errorf("unknown service - %s", name)
	}

	service := serviceInfo.Config
	ref.logger.Debugf("Execution.PrepareService(%s): image=%s", name, service.Image)

	if service.Image == "" {
		imageName := fmt.Sprintf("%s_%s", ref.Project.Name, name)
		if ref.BuildImages && service.Build != nil {
			if err := buildImage(ctx, ref.apiClient, ref.BaseComposeDir, imageName, service.Build); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("cant build service image - %s", name)
		}
	} else {
		found, err := HasImage(ref.apiClient, service.Image)
		if err != nil {
			return err
		}

		if found {
			return nil
		}

		//building image as if service.PullPolicy == types.PullPolicyBuild
		//todo: have an explicit flag for this behavior
		if ref.BuildImages && service.Build != nil {
			if err := buildImage(ctx, ref.apiClient, ref.BaseComposeDir, service.Image, service.Build); err != nil {
				return err
			}
		}

		if ref.PullImages {
			//building image as if
			//service.PullPolicy == types.PullPolicyIfNotPresent || types.PullPolicyMissing
			if err := pullImage(ctx, ref.apiClient, service.Image); err != nil {
				return err
			}

			//note: might need a better image check (already in DS?)
			found, err := HasImage(ref.apiClient, service.Image)
			if err != nil {
				return err
			}

			if !found {
				ref.logger.Debugf("Execution.PrepareService(%s): image=%s - no image and image pull did not pull image", name, service.Image)
				return ErrNoServiceImage
			}
		} else {
			ref.logger.Debugf("Execution.PrepareService(%s): image=%s - no image and image pull is not enabled", name, service.Image)
			return ErrNoServiceImage
		}
	}

	return nil
}

func (ref *Execution) DiscoverResources() error {
	ref.logger.Debug("Execution.DiscoverResources")
	//discover existing containers, volumes, networks for project
	return nil
}

func (ref *Execution) Start() error {
	ref.logger.Debug("Execution.Start")
	if err := ref.StartServices(); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) StartServices() error {
	ref.logger.Debug("Execution.StartServices")

	if err := ref.Project.WithServices(nil,
		func(service types.ServiceConfig) error {
			ref.logger.Debugf("Execution.StartServices: service.Name='%s'", service.Name)
			fullSvcInfo, found := ref.AllServices[service.Name]
			if !found {
				return fmt.Errorf("unknown service - %s", service.Name)
			}

			ref.logger.Debugf("Execution.StartServices: starting service=%s (image=%s)", service.Name, fullSvcInfo.Config.Image)

			if ref.options.SvcStartWait > 0 {
				ref.logger.Debugf("Execution.StartServices: waiting %v seconds before starting service=%s (image=%s)", ref.options.SvcStartWait, service.Name, fullSvcInfo.Config.Image)
				time.Sleep(time.Duration(ref.options.SvcStartWait) * time.Second)
			}

			err := ref.StartService(service.Name)
			if err != nil {
				ref.logger.Debugf("Execution.StartServices: ref.StartService() error = %v", err)
				return err
			}

			return nil
		}); err != nil {
		ref.logger.Debugf("Execution.StartServices: ref.Project.WithServices error = %v", err)
		return err
	}

	return nil
}

func (ref *Execution) StartService(name string) error {
	ref.logger.Debugf("Execution.StartService(%s)", name)

	_, running := ref.RunningServices[name]
	if running {
		ref.logger.Debugf("Execution.StartService(%s) - already running", name)
		return nil
	}

	_, pending := ref.PendingServices[name]
	if pending {
		ref.logger.Debugf("Execution.StartService(%s) - already starting", name)
		return nil
	}

	serviceInfo, found := ref.AllServices[name]
	if !found {
		return fmt.Errorf("unknown service - %s", name)
	}

	if !serviceInfo.Selected {
		ref.logger.Debugf("Execution.StartService(%s) - NOT SELECTED", name)
		return nil
	} else {
		ref.logger.Debugf("Execution.StartService(%s) - selected", name)
	}

	service := serviceInfo.Config
	ref.PendingServices[name] = struct{}{}

	//note:
	//don't really need this because StartService()
	//is called in reverse dependency order
	dependencies := service.GetDependencies()
	for _, dep := range dependencies {
		ref.logger.Debugf("Execution.StartService(%s): start dependency=%s", name, dep)
		err := ref.StartService(dep)
		if err != nil {
			return err
		}
	}

	delete(ref.PendingServices, name)
	//todo: need to refactor to use container.Execution
	id, err := startContainer(
		ref.apiClient,
		ref.Project.Name,
		//serviceInfo.Name, //full service name
		ref.AllServiceNames,
		ref.BaseComposeDir,
		ref.ActiveVolumes,
		ref.ActiveNetworks,
		serviceInfo)
	//service)
	if err != nil {
		ref.logger.Debugf("Execution.StartService(%s): startContainer() error - %v", name, err)
		return err
	}

	//if fullName != serviceInfo.Name {
	//	return fmt.Errorf("mismatching service name: %s/%s", fullName, serviceInfo.Name)
	//}

	rsvc := &RunningService{
		Name:          serviceInfo.Name,
		ID:            id,
		ContainerName: serviceInfo.ContainerName,
	}

	ref.RunningServices[name] = rsvc
	ref.logger.Debugf("Execution.StartService(%s): runningService=%#v", name, rsvc)

	return nil
}

func (ref *Execution) StopServices() error {
	ref.logger.Debug("Execution.StopServices")

	for key := range ref.RunningServices {
		err := ref.StopService(key)
		if err != nil && key != ref.ContainerProbeSvc {
			return err
		}
	}
	return nil
}

func (ref *Execution) CleanupServices() error {
	ref.logger.Debug("Execution.CleanupServices")

	for key := range ref.RunningServices {
		err := ref.CleanupService(key)
		if err != nil && key != ref.ContainerProbeSvc {
			return err
		}
	}
	return nil
}

func (ref *Execution) StopService(key string) error {
	ref.logger.Debugf("Execution.StopService(%s)\n", key)
	service, running := ref.RunningServices[key]
	if !running {
		ref.logger.Debugf("Execution.StopService(%s) - no running service", key)
		if serviceInfo, found := ref.AllServices[key]; found && serviceInfo != nil {
			if !serviceInfo.Selected {
				ref.logger.Debugf("Execution.StopService(%s) - service not selected", key)
				return nil
			}
		}
		return nil
	}

	ref.logger.Debugf("Execution.StopService(%s): service=%+v", key, service)

	timeout := uint(defaultStopTimeout)
	if err := ref.apiClient.StopContainer(service.ID, timeout); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) CleanupService(key string) error {
	ref.logger.Debugf("Execution.CleanupService(%s)", key)
	service, running := ref.RunningServices[key]
	if !running {
		ref.logger.Debugf("Execution.CleanupService(%s) - no running service", key)
		if serviceInfo, found := ref.AllServices[key]; found && serviceInfo != nil {
			if !serviceInfo.Selected {
				ref.logger.Debugf("Execution.CleanupService(%s) - service not selected", key)
				return nil
			}
		}
		return nil
	}

	ref.logger.Debugf("Execution.CleanupService(%s): service=%+v", key, service)

	options := dockerapi.RemoveContainerOptions{
		ID:    service.ID,
		Force: true,
	}

	if err := ref.apiClient.RemoveContainer(options); err != nil {
		return err
	}

	return nil
}

const (
	rtLabelAppVersion = "tmp.version"
	rtLabelApp        = "ds.runtime.container.type"
	rtLabelProject    = "ds.engine.compose.project"
	rtLabelService    = "ds.engine.compose.service"
	rtLabelVolumeName = "ds.engine.compose.volume.name"
	rtLabelVolumeKey  = "ds.engine.compose.volume.key"
	rtLabelNetwork    = "ds.engine.compose.network"
)

func ExposedPorts(expose types.StringOrNumberList, ports []types.ServicePortConfig) map[dockerapi.Port]struct{} {
	exposed := map[dockerapi.Port]struct{}{}
	for _, key := range expose {
		exposed[dockerapi.Port(key)] = struct{}{}
	}

	for _, portInfo := range ports {
		key := fmt.Sprintf("%d/%s", portInfo.Target, portInfo.Protocol)
		exposed[dockerapi.Port(key)] = struct{}{}
	}

	return exposed
}

func MountsFromVolumeConfigs(
	baseComposeDir string,
	configs []types.ServiceVolumeConfig,
	tmpfsConfigs []string,
	activeVolumes map[string]*ActiveVolume) ([]dockerapi.HostMount, error) {
	mounts := []dockerapi.HostMount{}
	for _, c := range configs {
		log.Debugf("compose.MountsFromVolumeConfigs(): volumeConfig=%#v", c)

		mount := dockerapi.HostMount{
			Type:     c.Type,
			Target:   c.Target,
			ReadOnly: c.ReadOnly,
		}

		if c.Source == "" {
			mount.Type = types.VolumeTypeVolume
		} else {
			source := c.Source
			_, found := activeVolumes[source]
			if !found {
				if !filepath.IsAbs(source) {
					if strings.HasPrefix(source, "~/") {
						hd, _ := os.UserHomeDir()
						source = filepath.Join(hd, source[2:])
					} else {
						source = filepath.Join(baseComposeDir, source)
					}
				}

				log.Debugf("compose.MountsFromVolumeConfigs(): no active volume (orig.source='%s' source='%s' activeVolumes=%#v)",
					c.Source, source, activeVolumes)

				mount.Type = types.VolumeTypeBind
			} else {
				log.Debugf("compose.MountsFromVolumeConfigs(): activeVolume='%s'", source)
				mount.Type = types.VolumeTypeVolume
			}

			mount.Source = source
		}

		if c.Bind != nil {
			mount.BindOptions = &dockerapi.BindOptions{
				Propagation: c.Bind.Propagation,
			}
		}

		if c.Volume != nil {
			mount.VolumeOptions = &dockerapi.VolumeOptions{
				NoCopy: c.Volume.NoCopy,
			}
		}

		if c.Tmpfs != nil {
			mount.TempfsOptions = &dockerapi.TempfsOptions{
				SizeBytes: c.Tmpfs.Size,
			}
		}

		mounts = append(mounts, mount)
	}

	for _, tc := range tmpfsConfigs {
		mount := dockerapi.HostMount{
			Type:   types.VolumeTypeTmpfs,
			Target: tc,
		}

		mounts = append(mounts, mount)
	}

	return mounts, nil
}

func EnvVarsFromService(varMap types.MappingWithEquals, varFiles types.StringList) []string {
	var result []string
	for k, v := range varMap {
		record := k
		if v != nil {
			record = fmt.Sprintf("%s=%s", record, *v)
		}

		result = append(result, record)
	}

	for _, file := range varFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Debugf("compose.EnvVarsFromService: error reading '%s' - %v", file, err)
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			if strings.HasPrefix(line, "#") {
				continue
			}

			result = append(result, line)
		}
	}

	return result
}

func HasImage(dclient *dockerapi.Client, imageRef string) (bool, error) {
	if imageRef == "" || imageRef == "." || imageRef == ".." {
		return false, fmt.Errorf("bad image reference")
	}

	info, err := dockerutil.HasImage(dclient, imageRef)
	if err != nil {
		if err == dockerapi.ErrNoSuchImage ||
			err == dockerutil.ErrNotFound {
			return false, nil
		}

		return false, err
	}

	log.Debugf("compose.HasImage(%s): image identity - %#v", imageRef, info)

	var repo string
	var tag string
	if strings.Contains(imageRef, "@") {
		parts := strings.SplitN(imageRef, "@", 2)
		repo = parts[0]
		tag = parts[1]
	} else {
		if strings.Contains(imageRef, ":") {
			parts := strings.SplitN(imageRef, ":", 2)
			repo = parts[0]
			tag = parts[1]
		} else {
			repo = imageRef
			tag = "latest"
		}
	}

	for _, t := range info.ShortTags {
		if t == tag {
			log.Debugf("compose.HasImage(%s): FOUND IT - %s (%s)", imageRef, t, repo)
			return true, nil
		}
	}

	log.Debugf("compose.HasImage(%s): NOT FOUND IT", imageRef)
	return false, nil
}

type ImageIdentity struct {
	ID           string
	ShortTags    []string
	RepoTags     []string
	ShortDigests []string
	RepoDigests  []string
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

func pullImage(ctx context.Context, apiClient *dockerapi.Client, imageRef string) error {
	log.Debugf("pullImage(%s)", imageRef)

	var repo string
	var tag string

	if strings.Contains(imageRef, "@") {
		parts := strings.SplitN(imageRef, "@", 2)
		repo = parts[0]
		tag = parts[1]
	} else {
		if strings.Contains(imageRef, ":") {
			parts := strings.SplitN(imageRef, ":", 2)
			repo = parts[0]
			tag = parts[1]
		} else {
			repo = imageRef
			tag = "latest"
		}
	}

	var output bytes.Buffer
	options := dockerapi.PullImageOptions{
		//Registry:
		Repository:    repo,
		Tag:           tag,
		OutputStream:  &output,
		RawJSONStream: true,
		Context:       ctx,
	}
	/*
		{"status":"","progressDetail":{"current":NUM,"total":NUM},progress:"","id":""}
	*/

	//TODO: add support for auth
	auth := dockerapi.AuthConfiguration{}

	log.Debugf("pullImage(%s) repo=%s tag=%s", imageRef, repo, tag)
	if err := apiClient.PullImage(options, auth); err != nil {
		log.Debugf("pullImage: dockerapi.PullImage() error = %v", err)
		return err
	}

	fmt.Println("pull output:")
	fmt.Println(output.String())
	fmt.Println("pull output [DONE]")

	return nil
}

// TODO: move builder into pkg
func buildImage(ctx context.Context, apiClient *dockerapi.Client, basePath, imageName string, config *types.BuildConfig) error {
	log.Debugf("buildImage(%s,%s)", basePath, imageName)

	var output bytes.Buffer
	buildOptions := dockerapi.BuildImageOptions{
		Context:             ctx,
		Name:                imageName,
		Dockerfile:          config.Dockerfile,
		OutputStream:        &output,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		Target:              config.Target,
		RawJSONStream:       true,
	}
	/*
	   {"stream":""}
	*/

	//buildOptions.NetworkMode = config.Network

	for key, val := range config.Args {
		if val == nil {
			arg := dockerapi.BuildArg{
				Name: key,
			}

			buildOptions.BuildArgs = append(buildOptions.BuildArgs, arg)
			continue
		}

		arg := dockerapi.BuildArg{
			Name:  key,
			Value: *val,
		}
		buildOptions.BuildArgs = append(buildOptions.BuildArgs, arg)
	}

	for key, val := range config.Labels {
		buildOptions.Labels[key] = val
	}

	for _, val := range config.CacheFrom {
		buildOptions.CacheFrom = append(buildOptions.CacheFrom, val)
	}

	//TODO: investigate []string to string
	if len(config.ExtraHosts) > 0 {
		buildOptions.ExtraHosts = config.ExtraHosts[0]
	}

	if strings.HasPrefix(config.Context, "http://") || strings.HasPrefix(config.Context, "https://") {
		buildOptions.Remote = config.Context
	} else {
		contextDir := config.Context
		if !strings.HasPrefix(contextDir, "/") {
			contextDir = filepath.Join(basePath, contextDir)
		}

		if info, err := os.Stat(contextDir); err == nil && info.IsDir() {
			buildOptions.ContextDir = contextDir
		} else {
			return fmt.Errorf("invalid context directory - %s", contextDir)
		}
	}

	if err := apiClient.BuildImage(buildOptions); err != nil {
		log.Debugf("buildImage: dockerapi.BuildImage() error = %v", err)
		return err
	}

	fmt.Println("build output:")
	fmt.Println(output.String())
	fmt.Println("build output [DONE]")

	return nil
}

func durationToSeconds(d *types.Duration) int {
	if d == nil {
		return 0
	}

	return int(time.Duration(*d).Seconds())
}

func VolumesFrom(serviceNames map[string]struct{},
	volumesFrom []string) []string {
	var vfList []string
	for _, vf := range volumesFrom {
		//service_name
		//service_name:ro
		//container:container_name
		//container:container_name:rw
		if strings.HasPrefix(vf, "container:") {
			vfList = append(vfList, vf[len("container:"):])
		}

		if len(serviceNames) > 0 {
			if strings.Contains(vf, ":") {
				parts := strings.Split(vf, ":")
				vf = parts[0]
			}

			//todo: check that we get the right names here
			if _, found := serviceNames[vf]; found {
				vfList = append(vfList, vf)
			}
		}
	}

	return vfList
}

func startContainer(
	apiClient *dockerapi.Client,
	projectName string,
	//fullServiceName string,
	serviceNames map[string]struct{},
	baseComposeDir string,
	activeVolumes map[string]*ActiveVolume,
	activeNetworks map[string]*ActiveNetwork,
	//service types.ServiceConfig
	serviceInfo *ServiceInfo) (string, error) {
	service := serviceInfo.Config //todo - cleanup
	log.Debugf("startContainer(%s/%s/%s)", serviceInfo.Name, service.Name, service.ContainerName)
	log.Debugf("service.Image=%s", service.Image)
	log.Debugf("service.Entrypoint=%s", service.Entrypoint)
	log.Debugf("service.Command=%s", service.Command)
	log.Debugf("service.Ports=%#v", service.Ports)
	log.Debugf("service.Environment=%#v", service.Environment)

	labels := map[string]string{}
	for name, val := range service.Labels {
		labels[name] = val
	}

	getServiceImage := func() string {
		return service.Image
		/*
			todo: test first with building images
			if service.Image != "" {
				return service.Image
			}

			return projectName + "_" + service.Name
		*/
	}

	labels[rtLabelApp] = rtLabelAppVersion
	labels[rtLabelProject] = projectName
	labels[rtLabelService] = service.Name

	var netModeKey string
	netMode := service.NetworkMode
	if netMode == "" {
		log.Debugf("startContainer(): activeNetworks[%d]=%+v",
			len(activeNetworks), activeNetworks)

		if len(activeNetworks) > 0 {
			serviceNetworks := service.Networks
			log.Debugf("startContainer(): configured service network count - %d", len(serviceNetworks))
			if len(serviceNetworks) == 0 {
				log.Debug("startContainer(): adding default service network config")
				defaultNet := fmt.Sprintf("%s_default", projectName)
				serviceNetworks = map[string]*types.ServiceNetworkConfig{
					defaultNet: nil,
				}
			}

			for name := range serviceNetworks {
				log.Debugf("startContainer(): service network config - %s", name)
				if _, ok := activeNetworks[name]; ok {
					netMode = activeNetworks[name].Name
					netModeKey = name
					log.Debugf("startContainer(): found active network config - %s (netMode=%s)", name, netMode)
					break
				}
			}
		}

		if netMode == "" {
			netMode = "none"
			log.Debug("startContainer(): default netMode to none")
		}
	}

	if (netMode == "none" || netMode == "host") && len(service.Networks) > 0 {
		log.Debugf("startContainer(%s): incompatible network_mode and networks config", service.Name)
		return "", fmt.Errorf("startContainer(%s): incompatible network_mode and networks config", service.Name)
	}

	netAliases := []string{
		service.Name,
	}

	if len(service.Networks) != 0 {
		netConfig, ok := service.Networks[netModeKey]
		if ok && netConfig != nil {
			netAliases = append(netAliases, netConfig.Aliases...)
		}
	}

	endpointsConfig := map[string]*dockerapi.EndpointConfig{
		netMode: {
			Aliases: netAliases,
		},
	}

	log.Debugf("startContainer(%s): endpointsConfig=%+v", service.Name, endpointsConfig)

	mounts, err := MountsFromVolumeConfigs(baseComposeDir, service.Volumes, service.Tmpfs, activeVolumes)
	if err != nil {
		return "", err
	}

	//need to make it work with all container name checks
	serviceInfo.ContainerName = serviceInfo.Name
	if service.ContainerName != "" {
		serviceInfo.ContainerName = service.ContainerName
	}

	envVars := EnvVarsFromService(service.Environment, service.EnvFile)
	containerOptions := dockerapi.CreateContainerOptions{
		Name: serviceInfo.ContainerName,
		Config: &dockerapi.Config{
			Image:        getServiceImage(),
			Entrypoint:   []string(service.Entrypoint),
			Cmd:          []string(service.Command),
			WorkingDir:   service.WorkingDir,
			Env:          envVars,
			Hostname:     service.Hostname,
			Domainname:   service.DomainName,
			DNS:          []string(service.DNS),
			User:         service.User,
			ExposedPorts: ExposedPorts(service.Expose, service.Ports),
			Labels:       labels,
			//Volumes:    - covered by "volume" HostConfig.Mounts,
			StopSignal:  service.StopSignal,
			StopTimeout: durationToSeconds(service.StopGracePeriod),
			//Healthcheck *HealthConfig -> service.HealthCheck *HealthCheckConfig - todo
			SecurityOpts: service.SecurityOpt,
			//AttachStdout: true, //todo: revisit
			//AttachStderr: true, //todo: revisit
			AttachStdin:     false,             //for termnal
			Tty:             service.Tty,       //for termnal
			OpenStdin:       service.StdinOpen, //for termnal
			StdinOnce:       false,
			NetworkDisabled: netMode == "none", //service.NetworkMode == "disabled"
			MacAddress:      service.MacAddress,
		},
		HostConfig: &dockerapi.HostConfig{
			Mounts:         mounts,
			VolumesFrom:    VolumesFrom(serviceNames, service.VolumesFrom),
			VolumeDriver:   service.VolumeDriver,
			ReadonlyRootfs: service.ReadOnly,
			//Binds: - covered by "bind" HostConfig.Mounts
			CapAdd:       []string(service.CapAdd),
			CapDrop:      []string(service.CapDrop),
			PortBindings: portBindingsFromServicePortConfigs(service.Ports), //map[Port][]PortBinding  -> Ports []ServicePortConfig
			//Links:        service.Links, - not the same links
			DNS:         service.DNS,
			DNSOptions:  service.DNSOpts,
			DNSSearch:   service.DNSSearch,
			ExtraHosts:  service.ExtraHosts,
			UsernsMode:  service.UserNSMode,
			NetworkMode: netMode,
			IpcMode:     service.Ipc,
			Isolation:   service.Isolation,
			PidMode:     service.Pid,
			//RestartPolicy: dockerapi.RestartPolicy{Name: service.Restart},//RestartPolicy - todo: handle it the right way
			//Devices: ,//[]Device <- service.Devices
			//LogConfig: ,//LogConfig <- service.Logging
			SecurityOpt:  service.SecurityOpt,
			Privileged:   service.Privileged,
			CgroupParent: service.CgroupParent,
			//Memory: ,//int64 <- service.MemLimit
			//MemoryReservation: ,//int64
			//KernelMemory: ,//int64
			//MemorySwap: ,//int64
			CPUShares: service.CPUShares,
			CPUSet:    service.CPUSet,
			//Ulimits: ,//[]ULimit <- service.Ulimits
			OomScoreAdj: int(service.OomScoreAdj),
			//MemorySwappiness: ,//*int64
			//OOMKillDisable: ,//*bool
			ShmSize: int64(service.ShmSize),
			//Tmpfs: - covered by "tmpfs" HostConfig.Mounts
			Sysctls:  service.Sysctls,
			CPUCount: service.CPUCount,
			//CPUPercent: ,///int64 <- service.CPUPercent
			Runtime: service.Runtime, //string
		},
		NetworkingConfig: &dockerapi.NetworkingConfig{
			EndpointsConfig: endpointsConfig,
		},
	}

	if service.Init != nil {
		containerOptions.HostConfig.Init = *service.Init
	}

	if service.PidsLimit > 0 {
		containerOptions.HostConfig.PidsLimit = &service.PidsLimit
	}

	if service.Init != nil {
		containerOptions.HostConfig.Init = *service.Init
	}

	log.Debugf("startContainer(%s): creating container...", service.Name)
	containerInfo, err := apiClient.CreateContainer(containerOptions)
	if err != nil {
		log.Debugf("startContainer(%s): create container error - %v", service.Name, err)
		return "", err
	}

	log.Debugf("startContainer(%s): connecting container to networks...", service.Name)
	for key, serviceNet := range service.Networks {
		log.Debugf("startContainer(%s): service network key=%s info=%v", service.Name, key, serviceNet)
		netInfo, found := activeNetworks[key]
		if !found || netInfo == nil {
			log.Debugf("startContainer(%s): skipping network = '%s'", service.Name, key)
			continue
		}

		log.Debugf("startContainer(%s): found network key=%s netInfo=%#v", service.Name, key, netInfo)

		options := dockerapi.NetworkConnectionOptions{
			Container: containerInfo.ID,
			EndpointConfig: &dockerapi.EndpointConfig{
				Aliases: []string{
					service.Name,
				},
			},
		}

		if serviceNet != nil && len(serviceNet.Aliases) != 0 {
			options.EndpointConfig.Aliases = append(options.EndpointConfig.Aliases, serviceNet.Aliases...)
		}

		options.EndpointConfig.Aliases = append(options.EndpointConfig.Aliases, containerInfo.ID[:12])

		if len(service.Links) > 0 {
			var networkLinks []string
			for _, linkInfo := range service.Links {
				var linkTarget string
				var linkName string
				parts := strings.Split(linkInfo, ":")
				switch len(parts) {
				case 1:
					linkTarget = parts[0]
					linkName = parts[0]
				case 2:
					linkTarget = parts[0]
					linkName = parts[1]
				default:
					log.Debugf("startContainer(%s): service.Links: malformed link - %s", service.Name, linkInfo)
					continue
				}

				networkLinks = append(networkLinks, fmt.Sprintf("%s:%s", linkTarget, linkName))
			}

			options.EndpointConfig.Links = networkLinks
		}

		if err := apiClient.ConnectNetwork(netInfo.ID, options); err != nil {
			log.Debugf("startContainer(%s): container network connect error - %v", service.Name, err)
			log.Debugf("startContainer(%s): netInfo.ID=%s options=%#v", service.Name, netInfo.ID, options)
			return "", err
		}
	}

	log.Debugf("startContainer(%s): starting container id='%s' cn='%s'",
		service.Name, containerInfo.ID, serviceInfo.ContainerName)

	if err := apiClient.StartContainer(containerInfo.ID, nil); err != nil {
		log.Debugf("startContainer(%s): start container error - %v", service.Name, err)
		return "", err
	}

	return containerInfo.ID, nil
}

func portBindingsFromServicePortConfigs(configs []types.ServicePortConfig) map[dockerapi.Port][]dockerapi.PortBinding {
	result := map[dockerapi.Port][]dockerapi.PortBinding{}
	for _, config := range configs {
		var bindings []dockerapi.PortBinding
		var binding dockerapi.PortBinding

		if config.Published > 0 {
			binding.HostPort = fmt.Sprint(config.Published)
		}

		portKey := dockerapi.Port(fmt.Sprintf("%d/%s", config.Target, config.Protocol))
		result[portKey] = append(bindings, binding)
	}

	return result
}

func createVolume(apiClient *dockerapi.Client, projectName, volKey, volFullName string, config types.VolumeConfig) (string, error) {
	labels := config.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	labels[rtLabelApp] = rtLabelAppVersion
	labels[rtLabelProject] = projectName
	labels[rtLabelVolumeName] = volFullName
	labels[rtLabelVolumeKey] = volKey

	volumeOptions := dockerapi.CreateVolumeOptions{
		Name:       volFullName, //already includes the project prefix
		Driver:     config.Driver,
		DriverOpts: config.DriverOpts,
		Labels:     labels,
	}

	log.Debugf("createVolume(%s, %s, %s) volumeOptions=%#v", projectName, volKey, volFullName, volumeOptions)
	volumeInfo, err := apiClient.CreateVolume(volumeOptions)
	if err != nil {
		log.Debugf("apiClient.CreateVolume() error = %v", err)
		return "", err
	}

	return volumeInfo.Name, nil
}

func (ref *Execution) CreateVolumes() error {
	projectName := strings.Trim(ref.Project.Name, "-_")

	for key, volume := range ref.Project.Volumes {
		name := fmt.Sprintf("%s_%s", projectName, key)
		ref.logger.Debugf("CreateVolumes: key=%s gen.Name=%s volume.Name=%s", key, name, volume.Name)

		if volume.Name != "" {
			name = volume.Name
		}

		id, err := createVolume(ref.apiClient, projectName, key, name, volume)
		if err != nil {
			return err
		}

		ref.ActiveVolumes[key] = &ActiveVolume{
			ShortName: key,
			FullName:  name,
			ID:        id,
		}
	}

	return nil
}

func (ref *Execution) DeleteVolumes() error {
	for key, volume := range ref.ActiveVolumes {
		ref.logger.Debugf("DeleteVolumes: key/name=%s ID=%s", key, volume.ID)

		err := deleteVolume(ref.apiClient, volume.ID)
		if err != nil && key != ref.ContainerProbeSvc {
			return err
		}

		delete(ref.ActiveVolumes, key)
	}

	return nil
}

func deleteVolume(apiClient *dockerapi.Client, id string) error {
	removeOptions := dockerapi.RemoveVolumeOptions{
		Name:  id,
		Force: true,
	}

	err := apiClient.RemoveVolumeWithOptions(removeOptions)
	if err != nil {
		log.Debugf("dclient.RemoveVolumeWithOptions() error = %v", err)
		return err
	}

	return nil
}

const defaultNetName = "default"

func (ref *Execution) CreateNetworks() error {
	ref.logger.Debugf("Execution.CreateNetworks")
	projectName := strings.Trim(ref.Project.Name, "-_")

	for key, networkInfo := range ref.AllNetworks {
		network := networkInfo.Config
		ref.logger.Debugf("Execution.CreateNetworks: key=%s name=%s", key, network.Name)

		created, id, err := createNetwork(ref.apiClient, projectName, key, networkInfo.Name, network)
		if err != nil {
			return err
		}

		ref.ActiveNetworks[key] = &ActiveNetwork{
			Name:    networkInfo.Name,
			ID:      id,
			Created: created,
		}
	}

	//need to create the 'default' network if it's not created yet
	if _, ok := ref.ActiveNetworks[defaultNetName]; !ok {
		//var defaultNetConfig types.NetworkConfig

		defaultNetworkInfo := ref.AllNetworks[defaultNetName]

		created, id, err := createNetwork(ref.apiClient, projectName, defaultNetName, defaultNetworkInfo.Name, defaultNetworkInfo.Config)
		if err != nil {
			return err
		}

		ref.ActiveNetworks[defaultNetName] = &ActiveNetwork{
			Name:    defaultNetworkInfo.Name,
			ID:      id,
			Created: created,
		}
	}

	return nil
}

func createNetwork(apiClient *dockerapi.Client, projectName, name, fullName string, config types.NetworkConfig) (bool, string, error) {
	log.Debugf("createNetwork(%s,%s)", projectName, name)

	//fullName := fullNetworkName(projectName, name, config.Name)
	//fullName := fmt.Sprintf("%s_%s", projectName, name)
	//if config.Name != "" && config.Name != defaultNetName {
	//	fullName = config.Name
	//}

	labels := map[string]string{
		rtLabelApp:     rtLabelAppVersion,
		rtLabelProject: projectName,
		rtLabelNetwork: name,
	}

	driverOpts := map[string]interface{}{}
	for k, v := range config.DriverOpts {
		driverOpts[k] = v
	}

	options := dockerapi.CreateNetworkOptions{
		Name:       fullName,
		Driver:     config.Driver,
		Options:    driverOpts,
		Labels:     labels,
		Internal:   config.Internal,
		Attachable: config.Attachable,
	}

	if options.Driver == "" {
		options.Driver = "bridge"
	}

	mustFind := false
	if config.External.External {
		mustFind = true
		fullName = name
		if config.External.Name != "" {
			fullName = config.External.Name
		}
	}

	//not using config.Ipam for now

	filter := dockerapi.NetworkFilterOpts{
		"name": map[string]bool{
			fullName: true,
		},
	}

	log.Debugf("createNetwork(%s,%s): lookup '%s'", projectName, name, fullName)

	networkList, err := apiClient.FilteredListNetworks(filter)
	if err != nil {
		log.Debugf("listNetworks(%s): dockerapi.FilteredListNetworks() error = %v", name, err)
		return false, "", err
	}

	if len(networkList) == 0 || networkList[0].Name != fullName {
		if mustFind {
			return false, "", fmt.Errorf("no external network - %s", fullName)
		}

		log.Debugf("createNetwork(%s,%s): create '%s'", projectName, name, options.Name)
		networkInfo, err := apiClient.CreateNetwork(options)
		if err != nil {
			log.Debugf("apiClient.CreateNetwork() error = %v", err)
			return false, "", err
		}

		return true, networkInfo.ID, nil
	}

	log.Debugf("createNetwork(%s,%s): found network '%s' (id=%s)", projectName, name, fullName, networkList[0].ID)
	return false, networkList[0].ID, nil
}

func (ref *Execution) DeleteNetworks() error {
	for key, network := range ref.ActiveNetworks {
		ref.logger.Debugf("DeleteNetworks: key=%s name=%s ID=%s", key, network.Name, network.ID)
		if !network.Created {
			ref.logger.Debug("DeleteNetworks: skipping...")
			continue
		}

		err := deleteNetwork(ref.apiClient, network.ID)
		if err != nil && key != ref.ContainerProbeSvc {
			return err
		}

		delete(ref.ActiveNetworks, key)
	}

	return nil
}

func deleteNetwork(apiClient *dockerapi.Client, id string) error {
	err := apiClient.RemoveNetwork(id)
	if err != nil {
		log.Debugf("dclient.RemoveNetwork() error = %v", err)
		return err
	}

	return nil
}

func dumpComposeJSON(data *types.Project) {
	if pretty, err := json.MarshalIndent(data, "", "  "); err == nil {
		fmt.Printf("%s\n", string(pretty))
	}
}

func dumpRawJSON(data map[string]interface{}) {
	if pretty, err := json.MarshalIndent(data, "", "  "); err == nil {
		fmt.Printf("%s\n", string(pretty))
	}
}

func dumpConfig(config *types.Config) {
	fmt.Printf("\n\n")
	fmt.Printf("types.Config:\n%#v\n", config)
}

func (ref *Execution) Stop() error {
	ref.logger.Debugf("Execution.Stop")

	if err := ref.StopServices(); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) Cleanup() error {
	ref.logger.Debugf("Execution.Cleanup")
	//todo:
	//pass 'force'/'all' param to clean the spec resources
	//not created by this instance (except external resources)

	if err := ref.CleanupServices(); err != nil {
		return err
	}

	if err := ref.DeleteVolumes(); err != nil {
		return err
	}

	if err := ref.DeleteNetworks(); err != nil {
		return err
	}

	return nil
}
