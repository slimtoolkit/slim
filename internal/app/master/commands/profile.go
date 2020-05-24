package commands

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container/probes/http"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

// Profile command exit codes
const (
	ecpOther = iota + 1
)

// OnProfile implements the 'profile' docker-slim command
func OnProfile(
	gparams *GenericParams,
	targetRef string,
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
	doRmFileArtifacts bool,
	copyMetaArtifactsLocation string,
	doRunTargetAsUser bool,
	doShowContainerLogs bool,
	overrides *config.ContainerOverrides,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	volumeMounts map[string]config.VolumeMount,
	doKeepPerms bool,
	pathPerms map[string]*fsutil.AccessInfo,
	excludePatterns map[string]*fsutil.AccessInfo,
	includePaths map[string]*fsutil.AccessInfo,
	includeBins map[string]*fsutil.AccessInfo,
	includeExes map[string]*fsutil.AccessInfo,
	doIncludeShell bool,
	doUseLocalMounts bool,
	doUseSensorVolume string,
	doKeepTmpArtifacts bool,
	continueAfter *config.ContinueAfter,
	ec *ExecutionContext) {
	const cmdName = "profile"
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewProfileCommand(gparams.ReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.OriginalImage = targetRef

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=params target=%v\n", appName, cmdName, targetRef)

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("%s[%s]: info=docker.connect.error message='%s'\n", appName, cmdName, exitMsg)
		fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
		os.Exit(ectCommon | ecNoDockerConnectInfo)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if !confirmNetwork(logger, client, overrides.Network) {
		fmt.Printf("%s[%s]: info=param.error status=unknown.network value=%s\n", appName, cmdName, overrides.Network)
		fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
		os.Exit(ectCommon | ecBadNetworkName)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Printf("%s[%s]: info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", appName, cmdName, targetRef)
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

	fmt.Printf("%s[%s]: state=image.inspection.start\n", appName, cmdName)

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	fmt.Printf("%s[%s]: info=image id=%v size.bytes=%v size.human=%v\n",
		appName, cmdName,
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	fmt.Printf("%s[%s]: state=image.inspection.done\n", appName, cmdName)
	fmt.Printf("%s[%s]: state=container.inspection.start\n", appName, cmdName)

	containerInspector, err := container.NewInspector(
		logger,
		client,
		statePath,
		imageInspector,
		localVolumePath,
		doUseLocalMounts,
		doUseSensorVolume,
		doKeepTmpArtifacts,
		overrides,
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
		includePaths,
		includeBins,
		includeExes,
		doIncludeShell,
		gparams.Debug,
		gparams.InContainer,
		true,
		prefix)
	errutil.FailOn(err)

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutil.FailOn(err)

	fmt.Printf("%s[%s]: info=container name=%v id=%v target.port.list=[%v] target.port.info=[%v] message='YOU CAN USE THESE PORTS TO INTERACT WITH THE CONTAINER'\n",
		appName, cmdName,
		containerInspector.ContainerName,
		containerInspector.ContainerID,
		containerInspector.ContainerPortList,
		containerInspector.ContainerPortsInfo)

	logger.Info("watching container monitor...")

	if "probe" == continueAfter.Mode {
		doHTTPProbe = true
	}

	if doHTTPProbe {
		probe, err := http.NewCustomProbe(
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
			true, prefix)
		errutil.FailOn(err)
		if len(probe.Ports) == 0 {
			fmt.Printf("%s[%s]: state=http.probe.error error='no exposed ports' message='expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services\n", appName, cmdName)
			logger.Info("shutting down 'fat' container...")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()

			fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
			return
		}

		probe.Start()
		continueAfter.ContinueChan = probe.DoneChan()
	}

	continueAfterMsg := "provide the expected input to allow the container inspector to continue its execution"
	switch continueAfter.Mode {
	case "timeout":
		continueAfterMsg = "no input required, execution will resume after the timeout"
	case "probe":
		continueAfterMsg = "no input required, execution will resume when HTTP probing is completed"
	}

	fmt.Printf("%s[%s]: info=continue.after mode=%v message='%v'\n", appName, cmdName, continueAfter.Mode, continueAfterMsg)

	switch continueAfter.Mode {
	case "enter":
		fmt.Printf("%s[%s]: info=prompt message='USER INPUT REQUIRED, PRESS <ENTER> WHEN YOU ARE DONE USING THE CONTAINER'\n", appName, cmdName)
		creader := bufio.NewReader(os.Stdin)
		_, _, _ = creader.ReadLine()
	case "signal":
		fmt.Printf("%s[%s]: info=prompt message='send SIGUSR1 when you are done using the container'\n", appName, cmdName)
		<-continueAfter.ContinueChan
		fmt.Printf("%s[%s]: info=event message='got SIGUSR1'\n", appName, cmdName)
	case "timeout":
		fmt.Printf("%s[%s]: info=prompt message='waiting for the target container (%v seconds)'\n", appName, cmdName, int(continueAfter.Timeout))
		<-time.After(time.Second * continueAfter.Timeout)
		fmt.Printf("%s[%s]: info=event message='done waiting for the target container'\n", appName, cmdName)
	case "probe":
		fmt.Printf("%s[%s]: info=prompt message='waiting for the HTTP probe to finish'\n", appName, cmdName)
		<-continueAfter.ContinueChan
		fmt.Printf("%s[%s]: info=event message='HTTP probe is done'\n", appName, cmdName)
	default:
		errutil.Fail("unknown continue-after mode")
	}

	fmt.Printf("%s[%s]: state=container.inspection.finishing\n", appName, cmdName)

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutil.WarnOn(err)

	fmt.Printf("%s[%s]: state=container.inspection.artifact.processing\n", appName, cmdName)

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		fmt.Printf("%s[%s]: info=results status='no data collected (no minified image generated). (version=%v location='%v')'\n",
			appName, cmdName,
			v.Current(), fsutil.ExeDir())
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutil.FailOn(err)

	fmt.Printf("%s[%s]: state=container.inspection.done\n", appName, cmdName)
	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = report.CmdStateCompleted

	if copyMetaArtifactsLocation != "" {
		toCopy := []string{
			report.DefaultContainerReportFileName,
			imageInspector.SeccompProfileName,
			imageInspector.AppArmorProfileName,
		}
		if !copyMetaArtifacts(logger,
			toCopy,
			artifactLocation, copyMetaArtifactsLocation) {
			fmt.Printf("%s[%s]: info=artifacts message='could not copy meta artifacts'\n", appName, cmdName)
		}
	}

	if err := doArchiveState(logger, client, artifactLocation, gparams.ArchiveState, stateKey); err != nil {
		fmt.Printf("%s[%s]: info=state message='could not archive state'\n", appName, cmdName)
		logger.Errorf("error archiving state - %v", err)
	}

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutil.Remove(artifactLocation)
		errutil.WarnOn(err)
	}

	fmt.Printf("%s[%s]: state=done\n", appName, cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = report.CmdStateDone
	if cmdReport.Save() {
		fmt.Printf("%s[%s]: info=report file='%s'\n", appName, cmdName, cmdReport.ReportLocation())
	}
}
