package build

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	"github.com/docker-slim/docker-slim/pkg/util/printbuffer"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName
const composeProjectNamePat = "dsbuild_%v_%v"

// Build command exit codes
const (
	ecbOther = iota + 1
	ecbBadCustomImageTag
	ecbImageBuildError
	ecbNoEntrypoint
	ecbBadTargetComposeSvc
	ecbComposeSvcNoImage
)

type ovars = app.OutVars

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
	composeFile string,
	targetComposeSvc string,
	composeSvcNoPorts bool,
	depExcludeComposeSvcAll bool,
	depIncludeComposeSvcDeps string,
	depIncludeComposeSvcs []string,
	depExcludeComposeSvcs []string,
	composeNets []string,
	cbOpts *config.ContainerBuildOptions,
	crOpts *config.ContainerRunOptions,
	outputTags []string,
	doHTTPProbe bool,
	httpProbeCmds []config.HTTPProbeCmd,
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
	portBindings map[docker.Port][]docker.PortBinding,
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
	volumeMounts map[string]config.VolumeMount,
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
	doUseLocalMounts bool,
	doUseSensorVolume string,
	doKeepTmpArtifacts bool,
	continueAfter *config.ContinueAfter,
	execCmd string,
	execFileCmd string,
	deleteFatImage bool) {

	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewBuildCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted
	cmdReport.TargetReference = targetRef

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

		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	xc.Out.State("started")

	if composeFile != "" && targetComposeSvc != "" {
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
		errutil.FailOn(err)

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
		//todo: remove the temporary fat image (should have a flag for it in case users want the fat image too)
	}
	serviceAliases := []string{}
	var depServicesExe *compose.Execution
	if composeFile != "" {
		if targetComposeSvc != "" && depIncludeComposeSvcDeps != targetComposeSvc {
			var found bool
			for _, svcName := range depExcludeComposeSvcs {
				if svcName == targetComposeSvc {
					found = true
					break
				}
			}

			if !found {
				depExcludeComposeSvcs = append(depExcludeComposeSvcs, targetComposeSvc)
			}
		}

		// when more than one target is supported
		// this should be done per service name
		serviceAliases = depExcludeComposeSvcs
		logger.Debugf("compose: serviceAliases='%s'\n", serviceAliases)

		selectors := compose.NewServiceSelectors(depIncludeComposeSvcDeps,
			depIncludeComposeSvcs,
			depExcludeComposeSvcs)

		//todo: move compose flags to options
		options := &compose.ExecutionOptions{}

		logger.Debugf("compose: file='%s' selectors='%+v'\n",
			composeFile, selectors)

		composeProjectName := fmt.Sprintf(composeProjectNamePat, os.Getpid(), time.Now().UTC().Format("20060102150405"))

		exe, err := compose.NewExecution(xc,
			logger,
			client,
			composeFile,
			selectors,
			composeProjectName,
			"",    //workingDir (todo: add a flag)
			nil,   //environment (todo: add a flag)
			false, //buildImages
			doPull,
			nil,  //pullExcludes (todo: add a flag)
			true, //ownAllResources
			options,
			nil,  //eventCh
			true) //printState

		errutil.FailOn(err)

		if !depExcludeComposeSvcAll && !exe.SelectedHaveImages() {
			xc.Out.Info("compose.file.error",
				ovars{
					"status": "service.no.image",
					"file":   composeFile,
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
						"file":   composeFile,
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

			// sort and override, add environment variables
			for _, env := range overrides.Env {
				envComponents := strings.SplitN(env, "=", 1)
				if len(envComponents) == 2 {
					targetSvcInfo.Config.Environment[envComponents[0]] = &envComponents[1]
				}
			}
			// combine into overrides
			combineEnv := make([]string, 0)
			for key := range targetSvcInfo.Config.Environment {
				variable := fmt.Sprintf("%s=%s", key, *targetSvcInfo.Config.Environment[key])
				combineEnv = append(combineEnv, variable)
			}
			overrides.Env = combineEnv

			logger.Debugf("compose: Environment_Variables='%v'\n", overrides.Env)

			targetRef = targetSvcInfo.Config.Image

			if len(targetSvcInfo.Config.Entrypoint) > 0 {
				logger.Debug("using targetSvcInfo.Config.Entrypoint")
				overrides.Entrypoint = []string(targetSvcInfo.Config.Entrypoint)
			}

			if len(targetSvcInfo.Config.Command) > 0 {
				logger.Debug("using targetSvcInfo.Config.Command")
				overrides.Cmd = []string(targetSvcInfo.Config.Command)
			}

			if !composeSvcNoPorts {
				logger.Debug("using targetSvcInfo.Config.Ports")
				for _, p := range targetSvcInfo.Config.Ports {
					portKey := fmt.Sprintf("%v/%v", p.Target, p.Protocol)
					pbSet, found := portBindings[docker.Port(portKey)]
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
							pbSet = append(pbSet, docker.PortBinding{
								HostIP:   p.HostIP,
								HostPort: hostPort,
							})

							portBindings[docker.Port(portKey)] = pbSet
						}
					} else {
						portBindings[docker.Port(portKey)] = []docker.PortBinding{{
							HostIP:   p.HostIP,
							HostPort: fmt.Sprintf("%v", p.Published),
						}}
					}
				}
			}
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
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		if doPull {
			xc.Out.Info("target.image",
				ovars{
					"status":  "image.not.found",
					"image":   targetRef,
					"message": "trying to pull target image",
				})

			err := imageInspector.Pull(doShowPullLogs, dockerConfigPath, registryAccount, registrySecret)
			errutil.FailOn(err)
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
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	xc.Out.Info("image",
		ovars{
			"id":         imageInspector.ImageInfo.ID,
			"size.bytes": imageInspector.ImageInfo.VirtualSize,
			"size.human": humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
		})

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

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
	}

	xc.Out.State("image.inspection.done")

	selectedNetNames := map[string]string{}
	if depServicesExe != nil {
		xc.Out.State("container.dependencies.init.start")
		err = depServicesExe.Prepare()
		errutil.FailOn(err)
		err = depServicesExe.Start()
		errutil.FailOn(err)

		//todo:
		//need a better way to make sure the dependencies are ready
		//monitor docker events
		//use basic application level checks (probing)
		time.Sleep(3 * time.Second)
		xc.Out.State("container.dependencies.init.done")

		var selectedNetworks bool
		//might need more info (including alias info) when targeting compose services
		allNetNames := depServicesExe.ActiveNetworkNames()
		if len(composeNets) > 0 {
			for _, key := range composeNets {
				if net, found := allNetNames[key]; found {
					selectedNetworks = true
					selectedNetNames[key] = net
				}
			}
		}

		if targetComposeSvc != "" {
			svcNets := depServicesExe.ActiveServiceNetworks(targetComposeSvc)
			for key, name := range svcNets {
				selectedNetworks = true
				selectedNetNames[key] = name
			}
		}

		if !selectedNetworks {
			selectedNetNames = allNetNames
		}
	}

	xc.Out.State("container.inspection.start")

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
		portBindings,
		doPublishExposedPorts,
		links,
		etcHostsMaps,
		dnsServers,
		dnsSearchDomains,
		doRunTargetAsUser,
		doShowContainerLogs,
		volumeMounts,
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
		selectedNetNames,
		serviceAliases,
		gparams.Debug,
		gparams.InContainer,
		true,
		prefix)
	errutil.FailOn(err)

	if len(containerInspector.FatContainerCmd) == 0 {
		xc.Out.Info("target.image.error",
			ovars{
				"status":  "no.entrypoint.cmd",
				"image":   targetRef,
				"message": "no ENTRYPOINT/CMD",
			})

		exitCode := commands.ECTBuild | ecbNoEntrypoint
		xc.Out.State("exited", ovars{"exit.code": exitCode})
		xc.Exit(exitCode)
	}

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutil.FailOn(err)

	xc.Out.Info("container",
		ovars{
			"name":             containerInspector.ContainerName,
			"id":               containerInspector.ContainerID,
			"target.port.list": containerInspector.ContainerPortList,
			"target.port.info": containerInspector.ContainerPortsInfo,
			"message":          "YOU CAN USE THESE PORTS TO INTERACT WITH THE CONTAINER",
		})

	logger.Info("watching container monitor...")

	if strings.Contains(continueAfter.Mode, config.CAMProbe) {
		doHTTPProbe = true
	}

	var probe *http.CustomProbe
	if doHTTPProbe {
		var err error
		probe, err = http.NewCustomProbe(
			xc,
			containerInspector,
			httpProbeCmds,
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
		errutil.FailOn(err)

		if len(probe.Ports) == 0 {
			xc.Out.State("http.probe.error",
				ovars{
					"error":   "no exposed ports",
					"message": "expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services",
				})

			logger.Info("shutting down 'fat' container...")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()

			exitCode := commands.ECTBuild | ecbImageBuildError
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			xc.Exit(exitCode)
		}

		probe.Start()
		continueAfter.ContinueChan = probe.DoneChan()
	}

	continueAfterMsg := "provide the expected input to allow the container inspector to continue its execution"
	if continueAfter.Mode == config.CAMTimeout {
		continueAfterMsg = "no input required, execution will resume after the timeout"
	}

	if strings.Contains(continueAfter.Mode, config.CAMProbe) {
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
			exec, err := containerInspector.APIClient.CreateExec(docker.CreateExecOptions{
				Container:    containerInspector.ContainerID,
				Cmd:          cmd,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
			})
			errutil.FailOn(err)
			buffer := &printbuffer.PrintBuffer{Prefix: fmt.Sprintf("%s[%s][exec]: output:", appName, cmdName)}
			errutil.FailOn(containerInspector.APIClient.StartExec(exec.ID, docker.StartExecOptions{
				InputStream:  input,
				OutputStream: buffer,
				ErrorStream:  buffer,
			}))
			inspect, err := containerInspector.APIClient.InspectExec(exec.ID)
			errutil.FailOn(err)
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

		xc.Exit(exitCode)
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutil.FailOn(err)

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
	errutil.FailOn(err)

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
	errutil.FailOn(err)

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
