package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app"
	//"github.com/docker-slim/docker-slim/pkg/app/master/config"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

var ErrNoServiceImage = errors.New("no service image")

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

type ExecutionEvenInfo struct {
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
}

type Execution struct {
	State           ExecutionState
	Selectors       *ServiceSelectors
	BuildImages     bool
	PullImages      bool
	OwnAllResources bool
	BaseComposeDir  string
	Version         uint
	FullVersion     string
	Project         *types.Project
	Raw             map[string]interface{}
	AllServices     map[string]*ServiceInfo
	AllNetworks     map[string]*NetworkInfo
	PendingServices map[string]struct{}
	RunningServices map[string]*RunningService
	ActiveVolumes   map[string]*ActiveVolume
	ActiveNetworks  map[string]*ActiveNetwork
	StopTimeout     uint

	options    *ExecutionOptions
	eventCh    chan *ExecutionEvenInfo
	printState bool
	xc         *app.ExecutionContext
	logger     *log.Entry
	apiClient  *dockerapi.Client
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
	AllDependencies []string
	Config          types.ServiceConfig
}

type NetworkInfo struct {
	Name   string
	Config types.NetworkConfig
}

type RunningService struct {
	Name string
	ID   string
}

type ActiveVolume struct {
	Name string
	ID   string
}

type ActiveNetwork struct {
	Name    string
	ID      string
	Created bool
}

const defaultStopTimeout = 7 //7 seconds

func NewExecution(
	xc *app.ExecutionContext,
	logger *log.Entry,
	apiClient *dockerapi.Client,
	composeFile string,
	selectors *ServiceSelectors,
	projectName string,
	workingDir string,
	environment map[string]string,
	buildImages bool,
	pullImages bool,
	pullExcludes []string,
	ownAllResources bool,
	options *ExecutionOptions,
	eventCh chan *ExecutionEvenInfo,
	printState bool) (*Execution, error) {
	if logger != nil {
		logger = logger.WithFields(log.Fields{"com": "compose.execution"})
	}

	fullComposeFilePath, err := filepath.Abs(composeFile)
	if err != nil {
		panic(err)
	}

	baseComposeDir := filepath.Dir(fullComposeFilePath)

	if projectName == "" {
		//default project name to the dir name for compose file
		projectName = filepath.Base(baseComposeDir)
	}

	b, err := ioutil.ReadFile(composeFile)
	if err != nil {
		return nil, err
	}

	rawConfig, err := loader.ParseYAML(b)
	if err != nil {
		return nil, err
	}

	if workingDir == "" {
		workingDir = baseComposeDir
	}

	projectConfig := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: composeFile, Config: rawConfig},
		},
	}

	if len(environment) > 0 {
		projectConfig.Environment = environment
	}

	project, err := loader.Load(projectConfig, withProjectName(projectName))
	if err != nil {
		return nil, err
	}

	if project == nil {
		return nil, fmt.Errorf("no project info")
	}

	//not supporting compose profiles for now
	exe := &Execution{
		State:           XSNone,
		Selectors:       selectors,
		OwnAllResources: ownAllResources,
		BuildImages:     buildImages,
		PullImages:      pullImages,
		BaseComposeDir:  baseComposeDir,
		Project:         project,
		Raw:             rawConfig,
		AllServices:     map[string]*ServiceInfo{},
		AllNetworks:     map[string]*NetworkInfo{},
		PendingServices: map[string]struct{}{},
		RunningServices: map[string]*RunningService{},
		ActiveVolumes:   map[string]*ActiveVolume{},
		ActiveNetworks:  map[string]*ActiveNetwork{},
		StopTimeout:     defaultStopTimeout,
		apiClient:       apiClient,
		options:         options,
		eventCh:         eventCh,
		printState:      printState,
		xc:              xc,
		logger:          logger,
	}

	exe.initVersion()
	exe.initServices()
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

func (ref *Execution) ActiveServiceNetworks(svcName string) map[string]string {
	networks := map[string]string{}

	if svc, found := ref.AllServices[svcName]; found {
		for netKey := range svc.Config.Networks {
			if netInfo, found := ref.AllNetworks[netKey]; found && netInfo != nil {
				networks[netKey] = netInfo.Name
			}
		}
	}

	return networks
}

//////////

func (ref *Execution) initServices() error {
	///////////////////////////////////////
	fmt.Println("Execution.initServices: TMP EXPERIMENT")
	fmt.Println("Execution.initServices: raw ref.Project.Services")
	for _, service := range ref.Project.Services {
		fmt.Printf("service.Name='%s' (deps=%+v)\n", service.Name, service.GetDependencies())
	}

	fmt.Println("Execution.initServices: raw ref.Project.WithServices")
	var serviceNames []string //nil - means all
	if err := ref.Project.WithServices(serviceNames,
		func(service types.ServiceConfig) error {
			fmt.Printf("service.Name='%s' (deps=%+v)\n", service.Name, service.GetDependencies())
			return nil
		}); err != nil {
		fmt.Println("ref.Project.WithServices error =", err)
	}
	fmt.Println("Execution.initServices: raw ref.Project.WithServices = DONE")

	///////////////////////////////////////
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
			fmt.Println("[2] ref.Project.WithServices error =", err)
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

	//TMP
	fmt.Println("Execution.initServices: checking ref.AllServices[x].Selected")

	for shortName, svc := range ref.AllServices {
		fmt.Printf("sname=%s/%s name=%s SELECTED?=%v\n",
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
	return fmt.Sprintf("%s_%s", project, service)
}

func fullNetworkName(project, networkKey, networkName string) string {
	fullName := fmt.Sprintf("%s_%s", project, networkKey)
	if networkName != "" && networkName != defaultNetName {
		fullName = networkName
	}
	return fullName
}

func (ref *Execution) Prepare() error {
	fmt.Println("Execution.Prepare")

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
	fmt.Println("Execution.Prepare")

	errCh := make(chan error, len(ref.AllServices))
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	for name := range ref.AllServices {
		wg.Add(1)
		go func(svcName string) {
			defer wg.Done()
			fmt.Printf("Execution.Prepare: service=%s\n", svcName)
			if err := ref.PrepareService(ctx, svcName); err != nil {
				fmt.Printf("Execution.Prepare: PrepareService(%s) error - %v\n", svcName, err)
				errCh <- err
				fmt.Printf("Execution.Prepare: PrepareService(%s) - CANCEL ALL PREPARE SVC\n", svcName)
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
	fmt.Printf("Execution.PrepareService(%s)\n", name)
	serviceInfo, ok := ref.AllServices[name]
	if !ok {
		return fmt.Errorf("unknown service - %s", name)
	}

	service := serviceInfo.Config
	fmt.Printf("Execution.PrepareService(%s): image=%s\n", name, service.Image)

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
				return ErrNoServiceImage
			}
		} else {
			return ErrNoServiceImage
		}
	}

	return nil
}

func (ref *Execution) DiscoverResources() error {
	fmt.Println("Execution.DiscoverResources")
	//discover existing containers, volumes, networks for project
	return nil
}

func (ref *Execution) Start() error {
	fmt.Println("Execution.Start")
	if err := ref.StartServices(); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) StartServices() error {
	fmt.Println("Execution.StartServices")

	if err := ref.Project.WithServices(nil,
		func(service types.ServiceConfig) error {
			fmt.Printf("Execution.StartServices: service.Name='%s'\n", service.Name)
			fullSvcInfo, found := ref.AllServices[service.Name]
			if !found {
				return fmt.Errorf("unknown service - %s", service.Name)
			}

			fmt.Printf("Execution.StartServices: starting service=%s (image=%s)\n", service.Name, fullSvcInfo.Config.Image)
			err := ref.StartService(service.Name)
			if err != nil {
				fmt.Printf("Execution.StartServices: ref.StartService() error = %v\n", err)
				return err
			}

			return nil
		}); err != nil {
		fmt.Println("Execution.StartServices: ref.Project.WithServices error =", err)
		return err
	}

	return nil
}

func (ref *Execution) StartService(name string) error {
	fmt.Printf("Execution.StartService(%s)\n", name)

	_, running := ref.RunningServices[name]
	if running {
		fmt.Printf("Execution.StartService(%s) - already running\n", name)
		return nil
	}

	_, pending := ref.PendingServices[name]
	if pending {
		fmt.Printf("Execution.StartService(%s) - already starting\n", name)
		return nil
	}

	serviceInfo, found := ref.AllServices[name]
	if !found {
		return fmt.Errorf("unknown service - %s", name)
	}

	if !serviceInfo.Selected {
		fmt.Printf("Execution.StartService(%s) - NOT SELECTED\n", name)
		return nil
	} else {
		fmt.Printf("Execution.StartService(%s) - selected\n", name)
	}

	service := serviceInfo.Config
	ref.PendingServices[name] = struct{}{}

	//note:
	//don't really need this because StartService()
	//is called in reverse dependency order
	dependencies := service.GetDependencies()
	for _, dep := range dependencies {
		fmt.Printf("Execution.StartService(%s): start dependency=%s\n", name, dep)
		err := ref.StartService(dep)
		fmt.Printf("\n\n")
		if err != nil {
			return err
		}
	}

	serviceNames := map[string]struct{}{}
	for n := range ref.AllServices {
		serviceNames[n] = struct{}{}
	}

	delete(ref.PendingServices, name)
	//todo: need to refactor to use container.Execution
	id, err := startContainer(
		ref.apiClient,
		ref.Project.Name,
		serviceInfo.Name,
		serviceNames,
		ref.BaseComposeDir,
		ref.ActiveVolumes,
		ref.ActiveNetworks,
		service)
	if err != nil {
		fmt.Printf("Execution.StartService(%s): startContainer() error - %v\n", name, err)
		return err
	}

	//if fullName != serviceInfo.Name {
	//	return fmt.Errorf("mismatching service name: %s/%s", fullName, serviceInfo.Name)
	//}

	ref.RunningServices[name] = &RunningService{
		Name: serviceInfo.Name,
		ID:   id,
	}
	return nil
}

func (ref *Execution) StopServices() error {
	fmt.Println("Execution.StopServices")

	for key := range ref.RunningServices {
		err := ref.StopService(key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ref *Execution) CleanupServices() error {
	fmt.Println("Execution.CleanupServices")

	for key := range ref.RunningServices {
		err := ref.CleanupService(key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ref *Execution) StopService(key string) error {
	fmt.Printf("Execution.StopService(%s)\n", key)
	service, running := ref.RunningServices[key]
	if !running {
		fmt.Printf("Execution.StopService(%s) - no running service\n", key)
		if serviceInfo, found := ref.AllServices[key]; found && serviceInfo != nil {
			if !serviceInfo.Selected {
				fmt.Printf("Execution.StopService(%s) - service not selected\n", key)
				return nil
			}
		}
		return nil
	}

	fmt.Printf("Execution.StopService(%s): service=%+v\n", key, service)

	timeout := uint(defaultStopTimeout)
	if err := ref.apiClient.StopContainer(service.ID, timeout); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) CleanupService(key string) error {
	fmt.Printf("Execution.CleanupService(%s)\n", key)
	service, running := ref.RunningServices[key]
	if !running {
		fmt.Printf("Execution.CleanupService(%s) - no running service\n", key)
		if serviceInfo, found := ref.AllServices[key]; found && serviceInfo != nil {
			if !serviceInfo.Selected {
				fmt.Printf("Execution.CleanupService(%s) - service not selected\n", key)
				return nil
			}
		}
		return nil
	}

	fmt.Printf("Execution.CleanupService(%s): service=%+v\n", key, service)

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
	rtLabelApp        = "runtime.container.type"
	rtLabelProject    = "ds.engine.compose.project"
	rtLabelService    = "ds.engine.compose.service"
	rtLabelVolume     = "ds.engine.compose.volume"
	rtLabelNetwork    = "ds.engine.compose.network"
)

func exposedPorts(expose types.StringOrNumberList, ports []types.ServicePortConfig) map[dockerapi.Port]struct{} {
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

func hostMountsFromVolumeConfigs(
	baseComposeDir string,
	configs []types.ServiceVolumeConfig,
	activeVolumes map[string]*ActiveVolume) (map[string]struct{}, []dockerapi.HostMount, error) {
	volumes := map[string]struct{}{}
	mounts := []dockerapi.HostMount{}
	for _, c := range configs {
		source := c.Source
		if _, ok := activeVolumes[source]; !ok {
			if !strings.HasPrefix(source, "~/") && !filepath.IsAbs(source) {
				source = filepath.Join(baseComposeDir, source)
			}
		}

		mount := dockerapi.HostMount{
			Type:     c.Type,
			Target:   c.Target,
			Source:   c.Source,
			ReadOnly: c.ReadOnly,
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

		if mount.Target != "" {
			volumes[mount.Target] = struct{}{}
		}
	}

	return volumes, mounts, nil
}

func envVarsFromService(varMap types.MappingWithEquals, varFiles types.StringList) []string {
	var result []string
	for k, v := range varMap {
		record := k
		if v != nil {
			record = fmt.Sprintf("%s=%s", record, *v)
		}

		result = append(result, record)
	}

	for _, file := range varFiles {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Printf("envVarsFromService: error reading '%s' - %v\n", err)
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

	info, err := HasImageX(dclient, imageRef)
	if err != nil {
		if err == dockerapi.ErrNoSuchImage {
			return false, nil
		}

		return false, err
	}

	fmt.Printf("HasImage(%s): image identity - %#v\n", imageRef, info)

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
			fmt.Printf("HasImage(%s): FOUND IT - %s (%s)\n", imageRef, t, repo)
			return true, nil
		}
	}

	fmt.Printf("HasImage(%s): NOT FOUND IT\n", imageRef)
	return false, nil
}

func HasImageX(dclient *dockerapi.Client, imageRef string) (*ImageIdentity, error) {
	//NOTES:
	//ListImages doesn't filter by image ID (must use ImageInspect instead)
	//Check images by name:tag, full or partial image ID or name@digest
	if imageRef == "" || imageRef == "." || imageRef == ".." {
		return nil, fmt.Errorf("bad image reference")
	}

	imageInfo, err := dclient.InspectImage(imageRef)
	if err != nil {
		if err == dockerapi.ErrNoSuchImage {
			fmt.Printf("HasImage(%s): dclient.InspectImage - dockerapi.ErrNoSuchImage", imageRef)
			return nil, err
		}

		return nil, err
	}

	return ImageToIdentity(imageInfo), nil
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
	fmt.Printf("pullImage(%s)\n", imageRef)

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

	fmt.Printf("pullImage(%s) repo=%s tag=%s\n", imageRef, repo, tag)
	if err := apiClient.PullImage(options, auth); err != nil {
		fmt.Printf("pullImage: dockerapi.PullImage() error = %v\n", err)
		return err
	}

	fmt.Println("pull output:")
	fmt.Println(output.String())
	fmt.Println("pull output [DONE]")

	return nil
}

//TODO: move builder into pkg
func buildImage(ctx context.Context, apiClient *dockerapi.Client, basePath, imageName string, config *types.BuildConfig) error {
	fmt.Printf("buildImage(%s,%s)\n", basePath, imageName)

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
		fmt.Printf("buildImage: dockerapi.BuildImage() error = %v\n", err)
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

func startContainer(
	apiClient *dockerapi.Client,
	projectName string,
	serviceName string,
	serviceNames map[string]struct{},
	baseComposeDir string,
	activeVolumes map[string]*ActiveVolume,
	activeNetworks map[string]*ActiveNetwork,
	service types.ServiceConfig) (string, error) {
	fmt.Printf("startContainer(%s)\n", service.Name)
	fmt.Printf("service.Image=%s\n", service.Image)
	fmt.Printf("service.Entrypoint=%s\n", service.Entrypoint)
	fmt.Printf("service.Command=%s\n", service.Command)
	fmt.Printf("service.Ports=%#v\n", service.Ports)
	fmt.Printf("service.Environment=%#v\n", service.Environment)

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
		fmt.Printf("startContainer(): activeNetworks[%d]=%+v\n",
			len(activeNetworks), activeNetworks)

		if len(activeNetworks) > 0 {
			serviceNetworks := service.Networks
			fmt.Printf("startContainer(): configured service network count - %d\n", len(serviceNetworks))
			if len(serviceNetworks) == 0 {
				fmt.Printf("startContainer(): adding default service network config\n")
				defaultNet := fmt.Sprintf("%s_default", projectName)
				serviceNetworks = map[string]*types.ServiceNetworkConfig{
					defaultNet: nil,
				}
			}

			for name := range serviceNetworks {
				fmt.Printf("startContainer(): service network config - %s\n", name)
				if _, ok := activeNetworks[name]; ok {
					netMode = activeNetworks[name].Name
					netModeKey = name
					fmt.Printf("startContainer(): found active network config - %s (netMode=%s)\n", name, netMode)
					break
				}
			}
		}

		if netMode == "" {
			netMode = "none"
			fmt.Printf("startContainer(): default netMode to none\n")
		}
	}

	if (netMode == "none" || netMode == "host") && len(service.Networks) > 0 {
		fmt.Printf("startContainer(%s): incompatible network_mode and networks config\n", service.Name)
		os.Exit(-1)
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

	fmt.Printf("startContainer(%s): endpointsConfig=%+v\n", service.Name, endpointsConfig)

	volumes, mounts, err := hostMountsFromVolumeConfigs(baseComposeDir, service.Volumes, activeVolumes)
	if err != nil {
		panic(err)
	}

	getVolumesFrom := func() []string {
		var vfList []string
		for _, vf := range service.VolumesFrom {
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

				if _, found := serviceNames[vf]; found {
					vfList = append(vfList, vf)
				}
			}
		}

		return vfList
	}

	//todo: use service.ContainerName instead of serviceName when available
	//need to make it work with all container name checks
	envVars := envVarsFromService(service.Environment, service.EnvFile)
	containerOptions := dockerapi.CreateContainerOptions{
		Name: serviceName,
		Config: &dockerapi.Config{
			Image:        getServiceImage(),
			Entrypoint:   []string(service.Entrypoint), //strslice.StrSlice(s.Entrypoint), <- CHECK!!!
			Cmd:          []string(service.Command),    //strslice.StrSlice(s.Command), <- CHECK!!!
			WorkingDir:   service.WorkingDir,
			Env:          envVars,
			Hostname:     service.Hostname,
			Domainname:   service.DomainName,
			DNS:          []string(service.DNS),
			User:         service.User,
			ExposedPorts: exposedPorts(service.Expose, service.Ports),
			Labels:       labels,
			Volumes:      volumes,
			StopSignal:   service.StopSignal,
			StopTimeout:  durationToSeconds(service.StopGracePeriod),
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
			VolumesFrom:    getVolumesFrom(),
			VolumeDriver:   service.VolumeDriver,
			ReadonlyRootfs: service.ReadOnly,
			//Binds: todo
			CapAdd:       []string(service.CapAdd),
			CapDrop:      []string(service.CapDrop),
			PortBindings: portBindingsFromServicePortConfigs(service.Ports), //map[Port][]PortBinding  -> Ports []ServicePortConfig
			Links:        service.Links,
			DNS:          service.DNS,
			DNSOptions:   service.DNSOpts,
			DNSSearch:    service.DNSSearch,
			ExtraHosts:   service.ExtraHosts,
			UsernsMode:   service.UserNSMode,
			NetworkMode:  netMode,
			IpcMode:      service.Ipc,
			Isolation:    service.Isolation,
			PidMode:      service.Pid,
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
			//Tmpfs: ,//map[string]string
			Sysctls:  service.Sysctls,
			CPUCount: service.CPUCount, //shuld c
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

	fmt.Printf("startContainer(%s): creating container...\n", service.Name)
	containerInfo, err := apiClient.CreateContainer(containerOptions)
	if err != nil {
		fmt.Printf("startContainer(%s): create container error - %v\n", service.Name, err)
		return "", err
	}

	fmt.Printf("startContainer(%s): connecting container to networks...\n", service.Name)
	for key, serviceNet := range service.Networks {
		fmt.Printf("startContainer(%s): service network key=%s info=%v\n", service.Name, key, serviceNet)
		netInfo, found := activeNetworks[key]
		if !found || netInfo == nil {
			fmt.Printf("startContainer(%s): skipping network = '%s'\n", service.Name, key)
			continue
		}

		fmt.Printf("startContainer(%s): found network key=%s netInfo=%#v\n", service.Name, key, netInfo)

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

		if err := apiClient.ConnectNetwork(netInfo.ID, options); err != nil {
			fmt.Printf("startContainer(%s): container network connect error - %v\n", service.Name, err)
			fmt.Printf("startContainer(%s): netInfo.ID=%s options=%#v\n", service.Name, netInfo.ID, options)
			return "", err
		}
	}

	fmt.Printf("startContainer(%s): starting container id=%s\n", service.Name, containerInfo.ID)
	if err := apiClient.StartContainer(containerInfo.ID, nil); err != nil {
		fmt.Printf("startContainer(%s): start container error - %v\n", service.Name, err)
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

func createVolume(apiClient *dockerapi.Client, projectName, name string, config types.VolumeConfig) (string, error) {
	id := fmt.Sprintf("%s_%s", projectName, name)
	labels := map[string]string{
		rtLabelApp:     rtLabelAppVersion,
		rtLabelProject: projectName,
		rtLabelVolume:  name,
	}

	volumeOptions := dockerapi.CreateVolumeOptions{
		Name:       id,
		Driver:     config.Driver,
		DriverOpts: config.DriverOpts,
		Labels:     labels,
	}

	volumeInfo, err := apiClient.CreateVolume(volumeOptions)
	if err != nil {
		fmt.Printf("dclient.CreateVolume() error = %v\n", err)
		return "", err
	}

	return volumeInfo.Name, nil
}

func (ref *Execution) CreateVolumes() error {
	projectName := strings.Trim(ref.Project.Name, "-_")

	for key, volume := range ref.Project.Volumes {
		fmt.Printf("CreateVolumes: key=%s name=%s\n", key, volume.Name)
		name := key
		if volume.Name != "" {
			name = volume.Name
		}

		id, err := createVolume(ref.apiClient, projectName, name, volume)
		if err != nil {
			return err
		}

		ref.ActiveVolumes[name] = &ActiveVolume{
			Name: name,
			ID:   id,
		}
	}

	return nil
}

func (ref *Execution) DeleteVolumes() error {
	for key, volume := range ref.ActiveVolumes {
		fmt.Printf("DeleteVolumes: key/name=%s ID=%s\n", key, volume.ID)

		err := deleteVolume(ref.apiClient, volume.ID)
		if err != nil {
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
		fmt.Printf("dclient.RemoveVolumeWithOptions() error = %v\n", err)
		return err
	}

	return nil
}

const defaultNetName = "default"

func (ref *Execution) CreateNetworks() error {
	fmt.Printf("Execution.CreateNetworks\n")
	projectName := strings.Trim(ref.Project.Name, "-_")

	for key, networkInfo := range ref.AllNetworks {
		network := networkInfo.Config
		fmt.Printf("Execution.CreateNetworks: key=%s name=%s\n", key, network.Name)

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
	fmt.Printf("createNetwork(%s,%s)\n", projectName, name)

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

	fmt.Printf("createNetwork(%s,%s): lookup '%s'\n", projectName, name, fullName)

	networkList, err := apiClient.FilteredListNetworks(filter)
	if err != nil {
		fmt.Printf("listNetworks(%s): dockerapi.FilteredListNetworks() error = %v", name, err)
		return false, "", err
	}

	if len(networkList) == 0 || networkList[0].Name != fullName {
		if mustFind {
			return false, "", fmt.Errorf("no external network - %s", fullName)
		}

		fmt.Printf("createNetwork(%s,%s): create '%s'\n", projectName, name, options.Name)
		networkInfo, err := apiClient.CreateNetwork(options)
		if err != nil {
			fmt.Printf("apiClient.CreateNetwork() error = %v\n", err)
			return false, "", err
		}

		return true, networkInfo.ID, nil
	}

	fmt.Printf("createNetwork(%s,%s): found network '%s' (id=%s)\n", projectName, name, fullName, networkList[0].ID)
	return false, networkList[0].ID, nil
}

func (ref *Execution) DeleteNetworks() error {
	for key, network := range ref.ActiveNetworks {
		fmt.Printf("DeleteNetworks: key=%s name=%s ID=%s\n", key, network.Name, network.ID)
		if !network.Created {
			fmt.Printf("DeleteNetworks: skipping...\n")
			continue
		}

		err := deleteNetwork(ref.apiClient, network.ID)
		if err != nil {
			return err
		}

		delete(ref.ActiveNetworks, key)
	}

	return nil
}

func deleteNetwork(apiClient *dockerapi.Client, id string) error {
	err := apiClient.RemoveNetwork(id)
	if err != nil {
		fmt.Printf("dclient.RemoveNetwork() error = %v\n", err)
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
	fmt.Println("Execution.Stop")

	if err := ref.StopServices(); err != nil {
		return err
	}

	return nil
}

func (ref *Execution) Cleanup() error {
	fmt.Println("Execution.Cleanup")
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
