package build

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/builder"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/compose"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/container"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/container/probes/http"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/consts"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	"github.com/docker-slim/docker-slim/pkg/util/printbuffer"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName
const composeProjectNamePat = "dsbuild_%v_%v"

// Build command exit codes
const (
	ecbOther = iota + 1
	ecbBadCustomImageTag
	ecbImageBuildError
	ecbImageAlreadyOptimized
	ecbOnbuildBaseImage
	ecbNoEntrypoint
	ecbBadTargetComposeSvc
	ecbComposeSvcNoImage
	ecbComposeSvcUnknownImage
)

type ovars = app.OutVars

func NewLogWriter(name string) *chanWriter {
	r, w := io.Pipe()
	cw := &chanWriter{
		Name: name,
		r:    r,
		w:    w,
	}
	go func() {
		s := bufio.NewScanner(cw.r)
		for s.Scan() {
			fmt.Println(name + ": " + string(s.Bytes()))
		}
	}()
	return cw
}

type chanWriter struct {
	Name string
	Chan chan string
	w    *io.PipeWriter
	r    *io.PipeReader
}

func (w *chanWriter) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

// OnCommand implements the 'build' docker-slim command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	targetRef string,
	doPull bool,
	dockerConfigPath string,
	registryAccount string,
	registrySecret string,
	doShowPullLogs bool,
	composeFiles []string,
	targetComposeSvc string,
	targetComposeSvcImage string,
	composeSvcStartWait int,
	composeSvcNoPorts bool,
	depExcludeComposeSvcAll bool,
	depIncludeComposeSvcDeps string,
	depIncludeTargetComposeSvcDeps bool,
	depIncludeComposeSvcs []string,
	depExcludeComposeSvcs []string,
	composeNets []string,
	composeEnvVars []string,
	composeEnvNoHost bool,
	composeWorkdir string,
	composeProjectName string,
	containerProbeComposeSvc string,
	cbOpts *config.ContainerBuildOptions,
	crOpts *config.ContainerRunOptions,
	outputTags []string,
	doHTTPProbe bool,
	httpProbeCmds []config.HTTPProbeCmd,
	httpProbeStartWait int,
	httpProbeRetryCount int,
	httpProbeRetryWait int,
	httpProbePorts []uint16,
	httpCrawlMaxDepth int,
	httpCrawlMaxPageCount int,
	httpCrawlConcurrency int,
	httpMaxConcurrentCrawlers int,
	doHTTPProbeFull bool,
	doHTTPProbeExitOnFailure bool,
	httpProbeAPISpecs []string,
	httpProbeAPISpecFiles []string,
	httpProbeApps []string,
	httpProbeProxyEndpoint string,
	httpProbeProxyPort int,
	portBindings map[dockerapi.Port][]dockerapi.PortBinding,
	doPublishExposedPorts bool,
	doRmFileArtifacts bool,
	copyMetaArtifactsLocation string,
	doRunTargetAsUser bool,
	doShowContainerLogs bool,
	doShowBuildLogs bool,
	imageOverrideSelectors map[string]bool,
	overrides *config.ContainerOverrides,
	instructions *config.ImageNewInstructions,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	explicitVolumeMounts map[string]config.VolumeMount,
	doKeepPerms bool,
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
	doUseLocalMounts bool,
	doUseSensorVolume string,
	doKeepTmpArtifacts bool,
	continueAfter *config.ContinueAfter,
	execCmd string,
	execFileCmd string,
	deleteFatImage bool,
	rtaOnbuildBaseImage bool,
	rtaSourcePT bool,
	sensorIPCEndpoint string,
	sensorIPCMode string) {

	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewBuildCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted
	cmdReport.TargetReference = targetRef

	cmdReportOnExit := func() {
		cmdReport.State = command.StateError
		if cmdReport.Save() {
			xc.Out.Info("report",
				ovars{
					"file": cmdReport.ReportLocation(),
				})
		}
	}

	xc.AddCleanupHandler(cmdReportOnExit)

	var customImageTag string
	var additionalTags []string

	if len(outputTags) > 0 {
		customImageTag = outputTags[0]

		if len(outputTags) > 1 {
			additionalTags = outputTags[1:]
		}
	}

	logger.Debugf("customImageTag='%s', additionalTags=%#v", customImageTag, additionalTags)

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}

		xc.Out.Error("docker.connect.error", exitMsg)

		exitCode := commands.ECTCommon | commands.ECNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		cmdReport.Error = "docker.connect.error"
		xc.Exit(exitCode)
	}
	xc.FailOn(err)

	xc.Out.State("started")

	if len(composeFiles) > 0 && targetComposeSvc != "" {
		xc.Out.Info("params",
			ovars{
				"target.type":   "compose.service",
				"target":        targetRef,
				"continue.mode": continueAfter.Mode,
				"rt.as.user":    doRunTargetAsUser,
				"keep.perms":    doKeepPerms,
				"tags":          strings.Join(outputTags, ","),
			})
	} else {
		if cbOpts.Dockerfile == "" {
			xc.Out.Info("params",
				ovars{
					"target.type":   "image",
					"target":        targetRef,
					"continue.mode": continueAfter.Mode,
					"rt.as.user":    doRunTargetAsUser,
					"keep.perms":    doKeepPerms,
					"tags":          strings.Join(outputTags, ","),
				})
		} else {
			xc.Out.Info("params",
				ovars{
					"target.type":   "dockerfile",
					"context":       targetRef,
					"file":          cbOpts.Dockerfile,
					"continue.mode": continueAfter.Mode,
					"rt.as.user":    doRunTargetAsUser,
					"keep.perms":    doKeepPerms,
				})
		}
	}

	if cbOpts.Dockerfile != "" {
		xc.Out.State("building",
			ovars{
				"message": "building basic image",
			})

		//create a fat image name:
		//* use the explicit fat image tag if provided
		//* or create one based on the user provided (slim image) custom tag if it's available
		//* otherwise auto-generate a name
		var fatImageRepoNameTag string
		if cbOpts.Tag != "" {
			fatImageRepoNameTag = cbOpts.Tag
		} else if customImageTag != "" {
			citParts := strings.Split(customImageTag, ":")
			switch len(citParts) {
			case 1:
				fatImageRepoNameTag = fmt.Sprintf("%s.fat", customImageTag)
			case 2:
				fatImageRepoNameTag = fmt.Sprintf("%s.fat:%s", citParts[0], citParts[1])
			default:
				xc.Out.Info("param.error",
					ovars{
						"status": "malformed.custom.image.tag",
						"value":  customImageTag,
					})

				exitCode := commands.ECTBuild | ecbBadCustomImageTag
				xc.Out.State("exited",
					ovars{
						"exit.code": exitCode,
						"version":   v.Current(),
						"location":  fsutil.ExeDir(),
					})

				cmdReport.Error = "malformed.custom.image.tag"
				xc.Exit(exitCode)
			}
		} else {
			fatImageRepoNameTag = fmt.Sprintf("docker-slim-tmp-fat-image.%v.%v",
				os.Getpid(), time.Now().UTC().Format("20060102150405"))
		}

		cbOpts.Tag = fatImageRepoNameTag

		xc.Out.Info("basic.image.info",
			ovars{
				"tag":        cbOpts.Tag,
				"dockerfile": cbOpts.Dockerfile,
				"context":    targetRef,
			})

		fatBuilder, err := builder.NewBasicImageBuilder(
			client,
			cbOpts,
			targetRef,
			doShowBuildLogs)
		xc.FailOn(err)

		err = fatBuilder.Build()

		if doShowBuildLogs || err != nil {
			xc.Out.LogDump("regular.image.build", fatBuilder.BuildLog.String(),
				ovars{
					"tag": cbOpts.Tag,
				})
		}

		if err != nil {
			xc.Out.Info("build.error",
				ovars{
					"status": "standard.image.build.error",
					"value":  err,
				})

			exitCode := commands.ECTBuild | ecbImageBuildError
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
					"version":   v.Current(),
					"location":  fsutil.ExeDir(),
				})

			xc.Exit(exitCode)
		}

		xc.Out.State("basic.image.build.completed")

		targetRef = fatImageRepoNameTag
	}

	var serviceAliases []string
	var depServicesExe *compose.Execution
	var baseVolumesFrom []string
	var baseMounts []dockerapi.HostMount
	if len(composeFiles) > 0 {
		if targetComposeSvc != "" && depIncludeComposeSvcDeps != targetComposeSvc {
			var found bool
			for _, svcName := range depExcludeComposeSvcs {
				if svcName == targetComposeSvc {
					found = true
					break
				}
			}

			if !found {
				//exclude the target service if the target service is not excluded already
				depExcludeComposeSvcs = append(depExcludeComposeSvcs, targetComposeSvc)
			}
		}

		if depIncludeTargetComposeSvcDeps {
			depIncludeComposeSvcDeps = targetComposeSvc
		}

		selectors := compose.NewServiceSelectors(
			depIncludeComposeSvcDeps,
			depIncludeComposeSvcs,
			depExcludeComposeSvcs)

		//todo: move compose flags to options
		options := &compose.ExecutionOptions{
			SvcStartWait: composeSvcStartWait,
		}

		logger.Debugf("compose: file(s)='%s' selectors='%+v'\n",
			strings.Join(composeFiles, ","), selectors)

		if composeProjectName == "" {
			composeProjectName = fmt.Sprintf(composeProjectNamePat, os.Getpid(), time.Now().UTC().Format("20060102150405"))
		}

		exe, err := compose.NewExecution(xc,
			logger,
			client,
			composeFiles,
			selectors,
			composeProjectName,
			composeWorkdir,
			composeEnvVars,
			composeEnvNoHost,
			containerProbeComposeSvc,
			false, //buildImages
			doPull,
			nil,  //pullExcludes (todo: add a flag)
			true, //ownAllResources
			options,
			nil,  //eventCh
			true) //printState

		xc.FailOn(err)

		if !depExcludeComposeSvcAll && !exe.SelectedHaveImages() {
			xc.Out.Info("compose.file.error",
				ovars{
					"status": "service.no.image",
					"files":  strings.Join(composeFiles, ","),
				})

			exitCode := commands.ECTBuild | ecbComposeSvcNoImage
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
					"version":   v.Current(),
					"location":  fsutil.ExeDir(),
				})

			xc.Exit(exitCode)
		}

		if targetComposeSvc != "" {
			targetSvcInfo := exe.Service(targetComposeSvc)
			if targetSvcInfo == nil {
				xc.Out.Info("target.compose.svc.error",
					ovars{
						"status": "unknown.compose.service",
						"target": targetComposeSvc,
						"files":  strings.Join(composeFiles, ","),
					})

				exitCode := commands.ECTBuild | ecbBadTargetComposeSvc
				xc.Out.State("exited",
					ovars{
						"exit.code": exitCode,
						"version":   v.Current(),
						"location":  fsutil.ExeDir(),
					})

				xc.Exit(exitCode)
			}

			serviceAliases = append(serviceAliases, targetSvcInfo.Config.Name)

			targetRef = targetSvcInfo.Config.Image
			if targetComposeSvcImage != "" {
				targetRef = commands.UpdateImageRef(logger, targetRef, targetComposeSvcImage)
				//shouldn't need to
				targetSvcInfo.Config.Image = targetRef
				logger.Debug("using target service override '%s' -> '%s' ", targetComposeSvcImage, targetRef)
			}

			if len(targetSvcInfo.Config.Entrypoint) > 0 {
				logger.Debug("using targetSvcInfo.Config.Entrypoint")
				overrides.Entrypoint = []string(targetSvcInfo.Config.Entrypoint)
			}

			if len(targetSvcInfo.Config.Command) > 0 {
				logger.Debug("using targetSvcInfo.Config.Command")
				overrides.Cmd = []string(targetSvcInfo.Config.Command)
			}

			if overrides.Workdir == "" {
				overrides.Workdir = targetSvcInfo.Config.WorkingDir
			}

			if overrides.Hostname == "" {
				overrides.Hostname = targetSvcInfo.Config.Hostname
			}

			labelMap := map[string]string{}
			for k, v := range targetSvcInfo.Config.Labels {
				labelMap[k] = v
			}

			for k, v := range overrides.Labels {
				labelMap[k] = v
			}

			overrides.Labels = labelMap

			if overrides.User != "" {
				overrides.User = targetSvcInfo.Config.User
			}

			//todo: add command flags for these fields too
			//targetSvcInfo.Config.DomainName

			//env vars
			//the env vars from compose are already "resolved" and must be "k=v"
			svcEnvVars := compose.EnvVarsFromService(
				targetSvcInfo.Config.Environment,
				targetSvcInfo.Config.EnvFile)

			emap := map[string]string{}
			//start with compose env vars
			for _, env := range svcEnvVars {
				envComponents := strings.SplitN(env, "=", 2)
				if len(envComponents) == 2 {
					emap[envComponents[0]] = envComponents[1]
				} else {
					logger.Debugf("svcEnvVars - unexpected env var: '%s'", env)
				}
			}
			//then use env vars from overrides
			for _, env := range overrides.Env {
				envComponents := strings.SplitN(env, "=", 2)
				if len(envComponents) == 2 {
					emap[envComponents[0]] = envComponents[1]
				} else {
					logger.Debugf("overrides.Env - unexpected env var: '%s'", env)
				}
			}

			// combine into overrides
			var combineEnv []string
			for key, val := range emap {
				variable := fmt.Sprintf("%s=%s", key, val)
				combineEnv = append(combineEnv, variable)
			}
			overrides.Env = combineEnv

			logger.Debugf("compose: Environment_Variables='%v'\n", overrides.Env)

			//expose ports
			svcExposedPorts := compose.ExposedPorts(targetSvcInfo.Config.Expose, targetSvcInfo.Config.Ports)
			if len(svcExposedPorts) > 0 && overrides.ExposedPorts == nil {
				overrides.ExposedPorts = map[dockerapi.Port]struct{}{}
			}

			for k, v := range svcExposedPorts {
				overrides.ExposedPorts[k] = v
			}

			//publish ports
			if !composeSvcNoPorts {
				logger.Debug("using targetSvcInfo.Config.Ports")
				for _, p := range targetSvcInfo.Config.Ports {
					portKey := fmt.Sprintf("%v/%v", p.Target, p.Protocol)
					pbSet, found := portBindings[dockerapi.Port(portKey)]
					if found {
						found := false
						hostPort := fmt.Sprintf("%v", p.Published)
						for _, pinfo := range pbSet {
							if pinfo.HostIP == p.HostIP && pinfo.HostPort == hostPort {
								found = true
								break
							}
						}

						if !found {
							pbSet = append(pbSet, dockerapi.PortBinding{
								HostIP:   p.HostIP,
								HostPort: hostPort,
							})

							portBindings[dockerapi.Port(portKey)] = pbSet
						}
					} else {
						portBindings[dockerapi.Port(portKey)] = []dockerapi.PortBinding{{
							HostIP:   p.HostIP,
							HostPort: fmt.Sprintf("%v", p.Published),
						}}
					}
				}
			}

			//make sure not to shadow baseMounts
			baseMounts, err = compose.MountsFromVolumeConfigs(
				exe.BaseComposeDir,
				targetSvcInfo.Config.Volumes,
				targetSvcInfo.Config.Tmpfs,
				exe.ActiveVolumes)
			xc.FailOn(err)

			logger.Debugf("compose targetSvcInfo - baseMounts(%d)", len(baseMounts))

			baseVolumesFrom = compose.VolumesFrom(exe.AllServiceNames,
				targetSvcInfo.Config.VolumesFrom)

			logger.Debugf("compose targetSvcInfo - baseVolumesFrom(%d)", len(baseVolumesFrom))
		}

		if !depExcludeComposeSvcAll {
			depServicesExe = exe
		}
	}

	logger.Infof("image=%v http-probe=%v remove-file-artifacts=%v image-overrides=%+v entrypoint=%+v (%v) cmd=%+v (%v) workdir='%v' env=%+v expose=%+v",
		targetRef, doHTTPProbe, doRmFileArtifacts,
		imageOverrideSelectors,
		overrides.Entrypoint, overrides.ClearEntrypoint, overrides.Cmd, overrides.ClearCmd,
		overrides.Workdir, overrides.Env, overrides.ExposedPorts)

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if overrides.Network == "host" && runtime.GOOS == "darwin" {
		xc.Out.Info("param.error",
			ovars{
				"status": "unsupported.network.mac",
				"value":  overrides.Network,
			})

		exitCode := commands.ECTCommon | commands.ECBadNetworkName
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		xc.Exit(exitCode)
	}

	if !commands.ConfirmNetwork(logger, client, overrides.Network) {
		xc.Out.Info("param.error",
			ovars{
				"status": "unknown.network",
				"value":  overrides.Network,
			})

		exitCode := commands.ECTCommon | commands.ECBadNetworkName
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		xc.Exit(exitCode)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	xc.FailOn(err)

	if imageInspector.NoImage() {
		if doPull {
			xc.Out.Info("target.image",
				ovars{
					"status":  "image.not.found",
					"image":   targetRef,
					"message": "trying to pull target image",
				})

			err := imageInspector.Pull(doShowPullLogs, dockerConfigPath, registryAccount, registrySecret)
			xc.FailOn(err)
		} else {
			xc.Out.Info("target.image.error",
				ovars{
					"status":  "image.not.found",
					"image":   targetRef,
					"message": "make sure the target image already exists locally (use --pull flag to auto-download it from registry)",
				})

			exitCode := commands.ECTBuild | ecbImageBuildError
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			xc.Exit(exitCode)
		}
	}

	//refresh the target refs
	targetRef = imageInspector.ImageRef
	cmdReport.TargetReference = imageInspector.ImageRef

	xc.Out.State("image.inspection.start")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	xc.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	xc.Out.Info("image",
		ovars{
			"id":         imageInspector.ImageInfo.ID,
			"size.bytes": imageInspector.ImageInfo.VirtualSize,
			"size.human": humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
		})

	if imageInspector.ImageInfo.Config != nil &&
		len(imageInspector.ImageInfo.Config.Labels) > 0 {
		for labelName := range imageInspector.ImageInfo.Config.Labels {
			if labelName == consts.ContainerLabelName {
				xc.Out.Info("target.image.error",
					ovars{
						"status":  "image.already.optimized",
						"image":   targetRef,
						"message": "the target image is already optimized",
					})

				exitCode := commands.ECTBuild | ecbImageAlreadyOptimized
				xc.Out.State("exited",
					ovars{
						"exit.code": exitCode,
					})

				cmdReport.Error = "image.already.optimized"
				xc.Exit(exitCode)
			}
		}
	}

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	xc.FailOn(err)

	if imageInspector.DockerfileInfo != nil {
		if imageInspector.DockerfileInfo.ExeUser != "" {
			xc.Out.Info("image.users",
				ovars{
					"exe": imageInspector.DockerfileInfo.ExeUser,
					"all": strings.Join(imageInspector.DockerfileInfo.AllUsers, ","),
				})
		}

		if len(imageInspector.DockerfileInfo.ImageStack) > 0 {
			cmdReport.ImageStack = imageInspector.DockerfileInfo.ImageStack

			for idx, layerInfo := range imageInspector.DockerfileInfo.ImageStack {
				xc.Out.Info("image.stack",
					ovars{
						"index": idx,
						"name":  layerInfo.FullName,
						"id":    layerInfo.ID,
					})
			}
		}

		if len(imageInspector.DockerfileInfo.ExposedPorts) > 0 {
			xc.Out.Info("image.exposed_ports",
				ovars{
					"list": strings.Join(imageInspector.DockerfileInfo.ExposedPorts, ","),
				})
		}

		if !rtaOnbuildBaseImage && imageInspector.DockerfileInfo.HasOnbuild {
			xc.Out.Info("target.image.error",
				ovars{
					"status":  "onbuild.base.image",
					"image":   targetRef,
					"message": "Runtime analysis for onbuild base images is not supported",
				})

			exitCode := commands.ECTBuild | ecbOnbuildBaseImage
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			cmdReport.Error = "onbuild.base.image"
			xc.Exit(exitCode)
		}
	}

	xc.Out.State("image.inspection.done")

	//validate links (check if target container exists, ignore&log if not)
	svcLinkMap := map[string]struct{}{}
	for _, linkInfo := range links {
		svcLinkMap[linkInfo] = struct{}{}
	}

	selectedNetNames := map[string]compose.NetNameInfo{}
	if depServicesExe != nil {
		xc.Out.State("container.dependencies.init.start")
		err = depServicesExe.Prepare()
		if err != nil {
			var svcErr *compose.ServiceError
			if errors.As(err, &svcErr) {
				xc.Out.Info("compose.file.error",
					ovars{
						"status":       "deps.unknown.image",
						"files":        strings.Join(composeFiles, ","),
						"service":      svcErr.Service,
						"pull.enabled": doPull,
						"message":      "Unknown dependency image (make sure to pull or build the images for your dependencies in compose)",
					})

				exitCode := commands.ECTBuild | ecbComposeSvcUnknownImage
				xc.Out.State("exited",
					ovars{
						"exit.code": exitCode,
						"version":   v.Current(),
						"location":  fsutil.ExeDir(),
					})

				xc.Exit(exitCode)
			}

			xc.FailOn(err)
		}

		err = depServicesExe.Start()
		if err != nil {
			depServicesExe.Stop()
			depServicesExe.Cleanup()
		}
		xc.FailOn(err)

		exeCleanup := func() {
			if depServicesExe != nil {
				xc.Out.State("container.dependencies.shutdown.start")
				err = depServicesExe.Stop()
				errutil.WarnOn(err)
				err = depServicesExe.Cleanup()
				errutil.WarnOn(err)
				xc.Out.State("container.dependencies.shutdown.done")
			}
		}

		xc.AddCleanupHandler(exeCleanup)

		//todo:
		//need a better way to make sure the dependencies are ready
		//monitor docker events
		//use basic application level checks (probing)
		time.Sleep(3 * time.Second)
		xc.Out.State("container.dependencies.init.done")

		//might need more info (including alias info) when targeting compose services
		allNetNames := depServicesExe.ActiveNetworkNames()

		if targetComposeSvc != "" {
			//if we are targetting a compose service and
			//we have explicitly selected compose networks (composeNets)
			//we use the selected subset of the configured networks for the target service
			composeNetsSet := map[string]struct{}{}
			for _, key := range composeNets {
				composeNetsSet[key] = struct{}{}
			}

			svcNets := depServicesExe.ActiveServiceNetworks(targetComposeSvc)
			for key, netNameInfo := range svcNets {
				if len(composeNets) > 0 {
					if _, found := composeNetsSet[key]; !found {
						continue
					}
				}

				selectedNetNames[key] = netNameInfo
			}
		} else {
			//we are not targetting a compose service,
			//but we do want to connect to the networks in compose
			if len(composeNets) > 0 {
				for _, key := range composeNets {
					if net, found := allNetNames[key]; found {
						selectedNetNames[key] = compose.NetNameInfo{
							FullName: net,
							//Aliases: serviceAliases, - we merge serviceAliases later
						}
					}
				}
			} else {
				//select/use all networks if specific networks are not selected
				for key, fullName := range allNetNames {
					selectedNetNames[key] = compose.NetNameInfo{
						FullName: fullName,
						//Aliases: serviceAliases, - we merge serviceAliases later
					}
				}
			}
		}
	}

	links = []string{} //reset&reuse
	if targetComposeSvc != "" && depServicesExe != nil {
		targetSvcInfo := depServicesExe.Service(targetComposeSvc)
		//convert service links to container links (after deps are started)
		targetSvcLinkMap := map[string]struct{}{}
		for _, linkInfo := range targetSvcInfo.Config.Links {
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
				logger.Debugf("targetSvcInfo.Config.Links: malformed link - %s", linkInfo)
				continue
			}

			linkSvcInfo := depServicesExe.Service(linkTarget)
			if linkSvcInfo == nil {
				logger.Debugf("targetSvcInfo.Config.Links: unknown service in link - %s", linkInfo)
				continue
			}

			logger.Debugf("targetSvcInfo.Config.Links: linkInfo=%s linkSvcInfo=%#v", linkInfo, linkSvcInfo)
			if linkSvcInfo.ContainerName == "" {
				logger.Debugf("targetSvcInfo.Config.Links: no container name - linkInfo=%s", linkInfo)
				continue
			}

			clink := fmt.Sprintf("%s:%s", linkSvcInfo.ContainerName, linkSvcInfo.ContainerName)
			targetSvcLinkMap[clink] = struct{}{}
			clink = fmt.Sprintf("%s:%s", linkSvcInfo.ContainerName, linkName)
			targetSvcLinkMap[clink] = struct{}{}
		}

		for k := range targetSvcLinkMap {
			links = append(links, k)
		}
	}

	for k := range svcLinkMap {
		links = append(links, k)
	}

	selectedNetworks := map[string]container.NetNameInfo{}
	for key, info := range selectedNetNames {
		aset := map[string]struct{}{}
		for _, a := range info.Aliases {
			aset[a] = struct{}{}
		}

		//merge serviceAliases with the main set of aliases
		for _, a := range serviceAliases {
			aset[a] = struct{}{}
		}
		var alist []string
		for a := range aset {
			alist = append(alist, a)
		}

		selectedNetworks[key] = container.NetNameInfo{
			Name:     key,
			FullName: info.FullName,
			Aliases:  alist,
		}
	}

	xc.Out.State("container.inspection.start")

	hasClassicLinks := true
	if targetComposeSvc != "" ||
		len(composeNets) > 0 ||
		overrides.Network != "" {
		hasClassicLinks = false
	}

	containerInspector, err := container.NewInspector(
		xc,
		crOpts,
		logger,
		client,
		statePath,
		imageInspector,
		localVolumePath,
		doUseLocalMounts,
		doUseSensorVolume,
		doKeepTmpArtifacts,
		overrides,
		explicitVolumeMounts,
		baseMounts,
		baseVolumesFrom,
		portBindings,
		doPublishExposedPorts,
		hasClassicLinks,
		links,
		etcHostsMaps,
		dnsServers,
		dnsSearchDomains,
		doRunTargetAsUser,
		doShowContainerLogs,
		doKeepPerms,
		pathPerms,
		excludePatterns,
		preservePaths,
		includePaths,
		includeBins,
		includeExes,
		doIncludeShell,
		doIncludeCertAll,
		doIncludeCertBundles,
		doIncludeCertDirs,
		doIncludeCertPKAll,
		doIncludeCertPKDirs,
		doIncludeNew,
		selectedNetworks,
		gparams.Debug,
		gparams.LogLevel,
		gparams.LogFormat,
		gparams.InContainer,
		sensorIPCEndpoint,
		sensorIPCMode,
		true,
		prefix)
	xc.FailOn(err)

	if len(containerInspector.FatContainerCmd) == 0 {
		xc.Out.Info("target.image.error",
			ovars{
				"status":  "no.entrypoint.cmd",
				"image":   targetRef,
				"message": "no ENTRYPOINT/CMD",
			})

		exitCode := commands.ECTBuild | ecbNoEntrypoint
		xc.Out.State("exited", ovars{"exit.code": exitCode})

		cmdReport.Error = "no.entrypoint.cmd"
		xc.Exit(exitCode)
	}

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	if err != nil && containerInspector.DoShowContainerLogs {
		containerInspector.ShowContainerLogs()
	}
	xc.FailOn(err)

	containerName := containerInspector.ContainerName
	containerID := containerInspector.ContainerID
	inspectorCleanup := func() {
		xc.Out.Info("container.inspector.cleanup",
			ovars{
				"name": containerName,
				"id":   containerID,
			})

		if containerInspector != nil {
			xc.Out.State("container.target.shutdown.start")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()
			xc.Out.State("container.target.shutdown.done")
		}
	}

	xc.AddCleanupHandler(inspectorCleanup)

	xc.Out.Info("container",
		ovars{
			"name":             containerInspector.ContainerName,
			"id":               containerInspector.ContainerID,
			"target.port.list": containerInspector.ContainerPortList,
			"target.port.info": containerInspector.ContainerPortsInfo,
			"message":          "YOU CAN USE THESE PORTS TO INTERACT WITH THE CONTAINER",
		})

	logger.Info("watching container monitor...")

	if hasContinueAfterMode(continueAfter.Mode, config.CAMProbe) {
		doHTTPProbe = true
	}

	var probe *http.CustomProbe
	if doHTTPProbe {
		var err error
		probe, err = http.NewCustomProbe(
			xc,
			containerInspector,
			httpProbeCmds,
			httpProbeStartWait,
			httpProbeRetryCount,
			httpProbeRetryWait,
			httpProbePorts,
			httpCrawlMaxDepth,
			httpCrawlMaxPageCount,
			httpCrawlConcurrency,
			httpMaxConcurrentCrawlers,
			doHTTPProbeFull,
			doHTTPProbeExitOnFailure,
			httpProbeAPISpecs,
			httpProbeAPISpecFiles,
			httpProbeApps,
			true,
			prefix)
		xc.FailOn(err)

		if len(probe.Ports) == 0 {
			xc.Out.State("http.probe.error",
				ovars{
					"error":   "no exposed ports",
					"message": "expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services",
				})

			//note: should be handled by inspectorCleanup
			//logger.Info("shutting down 'fat' container...")
			//containerInspector.FinishMonitoring()
			//_ = containerInspector.ShutdownContainer()

			exitCode := commands.ECTBuild | ecbImageBuildError
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			cmdReport.Error = "no.exposed.ports"
			xc.Exit(exitCode)
		}

		probe.Start()
		continueAfter.ContinueChan = probe.DoneChan()
	}

	continueAfterMsg := "provide the expected input to allow the container inspector to continue its execution"
	if continueAfter.Mode == config.CAMTimeout {
		continueAfterMsg = "no input required, execution will resume after the timeout"
	}

	if hasContinueAfterMode(continueAfter.Mode, config.CAMProbe) {
		continueAfterMsg = "no input required, execution will resume when HTTP probing is completed"
	}

	xc.Out.Info("continue.after",
		ovars{
			"mode":    continueAfter.Mode,
			"message": continueAfterMsg,
		})

	execFail := false

	modes := strings.Split(continueAfter.Mode, "&")
	for _, mode := range modes {
		//should work for the most parts except
		//when probe and signal are combined
		//because both need channels (TODO: fix)
		switch mode {
		case config.CAMContainerProbe:

			idsToLog := map[string]string{}
			idsToLog[targetRef] = containerInspector.ContainerID
			for name, svc := range depServicesExe.RunningServices {
				idsToLog[name] = svc.ID
			}
			//TODO:
			//need a flag to control logs for dep services
			//also good to leverage the logging capabilities in compose (TBD)
			for name, id := range idsToLog {
				name := name
				id := id
				go func() {
					err := client.Logs(dockerapi.LogsOptions{
						Container:    id,
						OutputStream: NewLogWriter(name + "-stdout"),
						ErrorStream:  NewLogWriter(name + "-stderr"),
						Follow:       true,
						Stdout:       true,
						Stderr:       true,
					})
					xc.FailOn(err)
				}()
			}

			svc, ok := depServicesExe.RunningServices[containerProbeComposeSvc]
			if !ok {
				xc.Out.State("error", ovars{"message": "container-prove-compose-svc not found in running services"})
				xc.Exit(1)
			}
			for {
				c, err := client.InspectContainerWithOptions(dockerapi.InspectContainerOptions{
					ID: svc.ID,
				})
				xc.FailOn(err)
				if c.State.Running {
					xc.Out.Info("wait for container.probe to finish")
				} else {
					if c.State.ExitCode != 0 {
						xc.Out.State("exited", ovars{"container.probe exit.code": c.State.ExitCode})
						xc.Exit(1)
					}
					break
				}
				time.Sleep(1 * time.Second)
			}
		case config.CAMEnter:
			xc.Out.Prompt("USER INPUT REQUIRED, PRESS <ENTER> WHEN YOU ARE DONE USING THE CONTAINER")
			creader := bufio.NewReader(os.Stdin)
			_, _, _ = creader.ReadLine()
		case config.CAMExec:
			var input *bytes.Buffer
			var cmd []string
			if len(execFileCmd) != 0 {
				input = bytes.NewBufferString(execFileCmd)
				cmd = []string{"sh", "-s"}
				for _, line := range strings.Split(string(execFileCmd), "\n") {
					xc.Out.Info("continue.after",
						ovars{
							"mode":  config.CAMExec,
							"shell": line,
						})
				}
			} else {
				input = bytes.NewBufferString("")
				cmd = []string{"sh", "-c", execCmd}
				xc.Out.Info("continue.after",
					ovars{
						"mode":  config.CAMExec,
						"shell": execCmd,
					})
			}
			exec, err := containerInspector.APIClient.CreateExec(dockerapi.CreateExecOptions{
				Container:    containerInspector.ContainerID,
				Cmd:          cmd,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
			})
			xc.FailOn(err)

			buffer := &printbuffer.PrintBuffer{Prefix: fmt.Sprintf("%s[%s][exec]: output:", appName, cmdName)}
			xc.FailOn(containerInspector.APIClient.StartExec(exec.ID, dockerapi.StartExecOptions{
				InputStream:  input,
				OutputStream: buffer,
				ErrorStream:  buffer,
			}))

			inspect, err := containerInspector.APIClient.InspectExec(exec.ID)
			xc.FailOn(err)
			errutil.FailWhen(inspect.Running, "still running")
			if inspect.ExitCode != 0 {
				execFail = true
			}

			xc.Out.Info("continue.after",
				ovars{
					"mode":     config.CAMExec,
					"exitcode": inspect.ExitCode,
				})
		case config.CAMSignal:
			xc.Out.Prompt("send SIGUSR1 when you are done using the container")
			<-continueAfter.ContinueChan
			xc.Out.Info("event",
				ovars{
					"message": "got SIGUSR1",
				})
		case config.CAMTimeout:
			xc.Out.Prompt(fmt.Sprintf("waiting for the target container (%v seconds)", int(continueAfter.Timeout)))
			<-time.After(time.Second * continueAfter.Timeout)
			xc.Out.Info("event",
				ovars{
					"message": "done waiting for the target container",
				})
		case config.CAMProbe:
			xc.Out.Prompt("waiting for the HTTP probe to finish")
			<-continueAfter.ContinueChan
			xc.Out.Info("event",
				ovars{
					"message": "HTTP probe is done",
				})

			if probe != nil && probe.CallCount > 0 && probe.OkCount == 0 {
				//make sure we show the container logs because none of the http probe calls were successful
				containerInspector.DoShowContainerLogs = true
			}
		default:
			errutil.Fail("unknown continue-after mode")
		}
	}

	xc.Out.State("container.inspection.finishing")

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutil.WarnOn(err)

	if execFail {
		xc.Out.Info("continue.after",
			ovars{
				"mode":    config.CAMExec,
				"message": "fatal: exec cmd failure",
			})

		exitCode := 1
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
			})

		cmdReport.Error = "exec.cmd.failure"
		xc.Exit(exitCode)
	}

	if depServicesExe != nil {
		xc.Out.State("container.dependencies.shutdown.start")
		err = depServicesExe.Stop()
		errutil.WarnOn(err)
		err = depServicesExe.Cleanup()
		errutil.WarnOn(err)
		xc.Out.State("container.dependencies.shutdown.done")
	}

	xc.Out.State("container.inspection.artifact.processing")

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		xc.Out.Info("results",
			ovars{
				"status":   "no data collected (no minified image generated)",
				"version":  v.Current(),
				"location": fsutil.ExeDir(),
			})

		exitCode := commands.ECTBuild | ecbImageBuildError
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
			})

		cmdReport.Error = "no.data.collected"
		xc.Exit(exitCode)
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	xc.FailOn(err)

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	xc.Out.State("container.inspection.done")
	xc.Out.State("building",
		ovars{
			"message": "building optimized image",
		})

	builder, err := builder.NewImageBuilder(client,
		customImageTag,
		additionalTags,
		imageInspector.ImageInfo,
		artifactLocation,
		doShowBuildLogs,
		imageOverrideSelectors,
		overrides,
		instructions)
	xc.FailOn(err)

	if !builder.HasData {
		logger.Info("WARNING - no data artifacts")
	}

	err = builder.Build()

	if doShowBuildLogs || err != nil {
		xc.Out.LogDump("optimized.image.build", builder.BuildLog.String(),
			ovars{
				"tag": customImageTag,
			})
	}

	if err != nil {
		xc.Out.Info("build.error",
			ovars{
				"status": "optimized.image.build.error",
				"error":  err,
			})

		exitCode := commands.ECTBuild | ecbImageBuildError
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		cmdReport.Error = "optimized.image.build.error"
		xc.Exit(exitCode)
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted

	if cbOpts.Dockerfile != "" {
		if deleteFatImage {
			xc.Out.Info("Dockerfile", ovars{
				"image.name":        cbOpts.Tag,
				"image.fat.deleted": "true",
			})
			var err = client.RemoveImage(cbOpts.Tag)
			errutil.WarnOn(err)
		} else {
			xc.Out.Info("Dockerfile", ovars{
				"image.name":        cbOpts.Tag,
				"image.fat.deleted": "false",
			})
		}
	}

	/////////////////////////////
	newImageInspector, err := image.NewInspector(client, builder.RepoName)
	xc.FailOn(err)

	if newImageInspector.NoImage() {
		xc.Out.Info("results",
			ovars{
				"message": "minified image not found",
				"image":   builder.RepoName,
			})

		exitCode := commands.ECTBuild | ecbImageBuildError
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
			})

		cmdReport.Error = "minified.image.not.found"
		xc.Exit(exitCode)
	}

	err = newImageInspector.Inspect()
	errutil.WarnOn(err)

	if err == nil {
		cmdReport.MinifiedBy = float64(imageInspector.ImageInfo.VirtualSize) / float64(newImageInspector.ImageInfo.VirtualSize)
		imgIdentity := dockerutil.ImageToIdentity(imageInspector.ImageInfo)
		cmdReport.SourceImage = report.ImageMetadata{
			Identity: report.ImageIdentity{
				ID:          imgIdentity.ID,
				Tags:        imgIdentity.ShortTags,
				Names:       imgIdentity.RepoTags,
				Digests:     imgIdentity.ShortDigests,
				FullDigests: imgIdentity.RepoDigests,
			},
			Size:          imageInspector.ImageInfo.VirtualSize,
			SizeHuman:     humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
			CreateTime:    imageInspector.ImageInfo.Created.UTC().Format(time.RFC3339),
			Author:        imageInspector.ImageInfo.Author,
			DockerVersion: imageInspector.ImageInfo.DockerVersion,
			Architecture:  imageInspector.ImageInfo.Architecture,
			User:          imageInspector.ImageInfo.Config.User,
			OS:            imageInspector.ImageInfo.OS,
		}

		for k := range imageInspector.ImageInfo.Config.ExposedPorts {
			cmdReport.SourceImage.ExposedPorts = append(cmdReport.SourceImage.ExposedPorts, string(k))
		}

		for k := range imageInspector.ImageInfo.Config.Volumes {
			cmdReport.SourceImage.Volumes = append(cmdReport.SourceImage.Volumes, k)
		}

		cmdReport.SourceImage.Labels = imageInspector.ImageInfo.Config.Labels
		cmdReport.SourceImage.EnvVars = imageInspector.ImageInfo.Config.Env

		cmdReport.MinifiedImageSize = newImageInspector.ImageInfo.VirtualSize
		cmdReport.MinifiedImageSizeHuman = humanize.Bytes(uint64(newImageInspector.ImageInfo.VirtualSize))

		xc.Out.Info("results",
			ovars{
				"status":         "MINIFIED",
				"by":             fmt.Sprintf("%.2fX", cmdReport.MinifiedBy),
				"size.original":  cmdReport.SourceImage.SizeHuman,
				"size.optimized": cmdReport.MinifiedImageSizeHuman,
			})
	} else {
		cmdReport.State = command.StateError
		cmdReport.Error = err.Error()
	}

	cmdReport.MinifiedImage = builder.RepoName
	cmdReport.MinifiedImageHasData = builder.HasData
	cmdReport.ArtifactLocation = imageInspector.ArtifactLocation
	cmdReport.ContainerReportName = report.DefaultContainerReportFileName
	cmdReport.SeccompProfileName = imageInspector.SeccompProfileName
	cmdReport.AppArmorProfileName = imageInspector.AppArmorProfileName

	xc.Out.Info("results",
		ovars{
			"image.name": cmdReport.MinifiedImage,
			"image.size": cmdReport.MinifiedImageSizeHuman,
			"has.data":   cmdReport.MinifiedImageHasData,
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.location": cmdReport.ArtifactLocation,
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.report": cmdReport.ContainerReportName,
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.dockerfile.reversed": "Dockerfile.fat",
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.dockerfile.optimized": "Dockerfile",
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.seccomp": cmdReport.SeccompProfileName,
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.apparmor": cmdReport.AppArmorProfileName,
		})

	if cmdReport.ArtifactLocation != "" {
		creportPath := filepath.Join(cmdReport.ArtifactLocation, cmdReport.ContainerReportName)
		if creportData, err := ioutil.ReadFile(creportPath); err == nil {
			var creport report.ContainerReport
			if err := json.Unmarshal(creportData, &creport); err == nil {
				cmdReport.System = report.SystemMetadata{
					Type:    creport.System.Type,
					Release: creport.System.Release,
					Distro:  creport.System.Distro,
				}
			} else {
				logger.Infof("could not read container report - json parsing error - %v", err)
			}
		} else {
			logger.Infof("could not read container report - %v", err)
		}

	}

	/////////////////////////////
	if copyMetaArtifactsLocation != "" {
		toCopy := []string{
			report.DefaultContainerReportFileName,
			imageInspector.SeccompProfileName,
			imageInspector.AppArmorProfileName,
		}
		if !commands.CopyMetaArtifacts(logger,
			toCopy,
			artifactLocation, copyMetaArtifactsLocation) {
			xc.Out.Info("artifacts",
				ovars{
					"message": "could not copy meta artifacts",
				})
		}
	}

	if err := commands.DoArchiveState(logger, client, artifactLocation, gparams.ArchiveState, stateKey); err != nil {
		xc.Out.Info("state",
			ovars{
				"message": "could not archive state",
			})

		logger.Errorf("error archiving state - %v", err)
	}

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutil.Remove(artifactLocation)
		errutil.WarnOn(err)
	}

	xc.Out.State("done")

	xc.Out.Info("commands",
		ovars{
			"message": "use the xray command to learn more about the optimize image",
		})

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}

func hasContinueAfterMode(modeSet, mode string) bool {
	for _, current := range strings.Split(modeSet, "&") {
		if current == mode {
			return true
		}
	}

	return false
}
