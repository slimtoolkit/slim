package profile

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/container"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/probes/http"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

// Profile command exit codes
const (
	ecpOther = iota + 1
	ecpNoEntrypoint
	ecpImageNotFound
)

type ovars = app.OutVars

//note: the runtime part of the 'profile' logic is a bit behind 'build'
//todo: refactor 'xray', 'profile' and 'build' to compose and reuse common logic

// OnCommand implements the 'profile' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	targetRef string,
	doPull bool,
	dockerConfigPath string,
	registryAccount string,
	registrySecret string,
	doShowPullLogs bool,
	crOpts *config.ContainerRunOptions,
	httpProbeOpts config.HTTPProbeOptions,
	portBindings map[docker.Port][]docker.PortBinding,
	doPublishExposedPorts bool,
	hostExecProbes []string,
	doRmFileArtifacts bool,
	copyMetaArtifactsLocation string,
	doRunTargetAsUser bool,
	doShowContainerLogs bool,
	overrides *config.ContainerOverrides,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	explicitVolumeMounts map[string]config.VolumeMount,
	//doKeepPerms bool,
	//pathPerms map[string]*fsutil.AccessInfo,
	excludePatterns map[string]*fsutil.AccessInfo,
	//includePaths map[string]*fsutil.AccessInfo,
	//includeBins map[string]*fsutil.AccessInfo,
	//includeExes map[string]*fsutil.AccessInfo,
	//doIncludeShell bool,
	doUseLocalMounts bool,
	doUseSensorVolume string,
	//doKeepTmpArtifacts bool,
	continueAfter *config.ContinueAfter,
	sensorIPCEndpoint string,
	sensorIPCMode string,
	logLevel string,
	logFormat string) {
	printState := true
	logger := log.WithFields(log.Fields{"app": appName, "command": Name})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewProfileCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted
	cmdReport.OriginalImage = targetRef

	xc.Out.State("started")
	xc.Out.Info("params",
		ovars{
			"target": targetRef,
		})

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := commands.ECTCommon | commands.ECCNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(xc, Name, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if overrides.Network == "host" && runtime.GOOS == "darwin" {
		xc.Out.Info("param.error",
			ovars{
				"status": "unsupported.network.mac",
				"value":  overrides.Network,
			})

		exitCode := commands.ECTCommon | commands.ECCBadNetworkName
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

		exitCode := commands.ECTCommon | commands.ECCBadNetworkName
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
					"message": "make sure the target image already exists locally",
				})

			exitCode := commands.ECTCommon | ecpImageNotFound
			xc.Out.State("exited", ovars{"exit.code": exitCode})
			xc.Exit(exitCode)
		}
	}

	//refresh the target refs
	targetRef = imageInspector.ImageRef

	xc.Out.State("image.inspection.start")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	xc.Out.Info("image",
		ovars{
			"id":           imageInspector.ImageInfo.ID,
			"size.bytes":   imageInspector.ImageInfo.VirtualSize,
			"size.human":   humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
			"architecture": imageInspector.ImageInfo.Architecture,
		})

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	xc.Out.State("image.inspection.done")
	xc.Out.State("container.inspection.start")

	//note:
	//not pre-processing links for 'profile' yet
	//need to copy the logic from 'build'
	//(better yet refactor to share code)
	hasClassicLinks := true //placeholder for now

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
		false, //doKeepTmpArtifacts,
		overrides,
		explicitVolumeMounts,
		nil, //baseMounts,
		nil, //baseVolumesFrom,
		portBindings,
		doPublishExposedPorts,
		hasClassicLinks,
		links,
		etcHostsMaps,
		dnsServers,
		dnsSearchDomains,
		doShowContainerLogs,
		doRunTargetAsUser,
		false, //doKeepPerms,
		nil,   //pathPerms,
		excludePatterns,
		nil,   //preservePaths,
		nil,   //includePaths,
		nil,   //includeBins,
		nil,   //includeExes,
		false, //doIncludeShell,
		false, //doIncludeWorkdir,
		false, //doIncludeCertAll
		false, //doIncludeCertBundles
		false, //doIncludeCertDirs
		false, //doIncludeCertPKAll
		false, //doIncludeCertPKDirs
		false, //doIncludeNew
		false, //doIncludeOSLibsNet
		nil,   //selectedNetNames
		//nil,
		gparams.Debug,
		logLevel,
		logFormat,
		gparams.InContainer,
		true,  //rtaSourcePT
		false, //doObfuscateMetadata
		sensorIPCEndpoint,
		sensorIPCMode,
		printState,
		config.AppNodejsInspectOptions{})
	errutil.FailOn(err)

	if len(containerInspector.FatContainerCmd) == 0 {
		xc.Out.Info("target.image.error",
			ovars{
				"status":  "no.entrypoint.cmd",
				"image":   targetRef,
				"message": "no ENTRYPOINT/CMD",
			})

		exitCode := commands.ECTBuild | ecpNoEntrypoint
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

	if config.CAMProbe == continueAfter.Mode {
		httpProbeOpts.Do = true
	}

	var probe *http.CustomProbe
	if httpProbeOpts.Do {
		var err error
		probe, err = http.NewContainerProbe(xc, containerInspector, httpProbeOpts, printState)
		errutil.FailOn(err)

		if len(probe.Ports()) == 0 {
			xc.Out.State("http.probe.error",
				ovars{
					"error":   "NO EXPOSED PORTS",
					"message": "expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services",
				})

			logger.Info("shutting down 'fat' container...")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()

			xc.Out.State("exited", ovars{"exit.code": -1})
			xc.Exit(-1)
		}

		probe.Start()
		continueAfter.ContinueChan = probe.DoneChan()
	}

	continueAfterMsg := "provide the expected input to allow the container inspector to continue its execution"
	switch continueAfter.Mode {
	case config.CAMTimeout:
		continueAfterMsg = "no input required, execution will resume after the timeout"
	case config.CAMProbe:
		continueAfterMsg = "no input required, execution will resume when HTTP probing is completed"
	}

	xc.Out.Info("continue.after",
		ovars{
			"mode":    continueAfter.Mode,
			"message": continueAfterMsg,
		})

	execFail := false

	modes := commands.GetContinueAfterModeNames(continueAfter.Mode)
	for _, mode := range modes {
		switch mode {
		//case config.CAMContainerProbe:
		/*
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

				buffer := &printbuffer.PrintBuffer{Prefix: fmt.Sprintf("%s[%s][exec]: output:", appName, Name)}
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
		*/
		case config.CAMEnter:
			xc.Out.Prompt("USER INPUT REQUIRED, PRESS <ENTER> WHEN YOU ARE DONE USING THE CONTAINER")
			creader := bufio.NewReader(os.Stdin)
			_, _, _ = creader.ReadLine()
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

			if probe != nil && probe.CallCount > 0 && probe.OkCount == 0 && httpProbeOpts.ExitOnFailure {
				xc.Out.Error("probe.error", "no.successful.calls")

				containerInspector.ShowContainerLogs()
				xc.Out.State("exited", ovars{"exit.code": -1})
				xc.Exit(-1)
			}
		case config.CAMHostExec:
			commands.RunHostExecProbes(printState, xc, hostExecProbes)
		case config.CAMAppExit:
			xc.Out.Prompt("waiting for the target app to exit")
			//TBD
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

	xc.Out.State("container.inspection.artifact.processing")

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		xc.Out.Info("results",
			ovars{
				"status":   "no data collected (no minified image generated)",
				"version":  v.Current(),
				"location": fsutil.ExeDir(),
			})

		xc.Out.State("exited", ovars{"exit.code": -1})
		xc.Exit(-1)
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutil.FailOn(err)

	xc.Out.State("container.inspection.done")
	xc.Out.State("completed")

	cmdReport.State = command.StateCompleted

	cmdReport.SeccompProfileName = imageInspector.SeccompProfileName
	cmdReport.AppArmorProfileName = imageInspector.AppArmorProfileName

	xc.Out.Info("results",
		ovars{
			"artifacts.seccomp": cmdReport.SeccompProfileName,
		})

	xc.Out.Info("results",
		ovars{
			"artifacts.apparmor": cmdReport.AppArmorProfileName,
		})

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
