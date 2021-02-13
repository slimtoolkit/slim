package profile

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container/probes/http"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

// Profile command exit codes
const (
	ecpOther = iota + 1
	ecpNoEntrypoint
	ecpImageNotFound
)

// OnCommand implements the 'profile' docker-slim command
func OnCommand(
	gparams *commands.GenericParams,
	targetRef string,
	doPull bool,
	doShowPullLogs bool,
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
	portBindings map[docker.Port][]docker.PortBinding,
	doPublishExposedPorts bool,
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
	ec *commands.ExecutionContext) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewProfileCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted
	cmdReport.OriginalImage = targetRef

	fmt.Printf("cmd=%s state=started\n", cmdName)
	fmt.Printf("cmd=%s info=params target=%v\n", cmdName, targetRef)

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("cmd=%s info=docker.connect.error message='%s'\n", cmdName, exitMsg)
		fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
		commands.Exit(commands.ECTCommon | commands.ECNoDockerConnectInfo)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if overrides.Network == "host" && runtime.GOOS == "darwin" {
		fmt.Printf("cmd=%s info=param.error status=unsupported.network.mac value=%s\n", cmdName, overrides.Network)
		fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
		commands.Exit(commands.ECTCommon | commands.ECBadNetworkName)
	}

	if !commands.ConfirmNetwork(logger, client, overrides.Network) {
		fmt.Printf("cmd=%s info=param.error status=unknown.network value=%s\n", cmdName, overrides.Network)
		fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
		commands.Exit(commands.ECTCommon | commands.ECBadNetworkName)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		if doPull {
			fmt.Printf("cmd=%s info=target.image status=not.found image='%v' message='trying to pull target image'\n", cmdName, targetRef)
			err := imageInspector.Pull(doShowPullLogs)
			errutil.FailOn(err)
		} else {
			fmt.Printf("cmd=%s info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", cmdName, targetRef)
			fmt.Printf("cmd=%s state=exited\n", cmdName)
			commands.Exit(commands.ECTCommon | ecpImageNotFound)
		}
	}

	fmt.Printf("cmd=%s state=image.inspection.start\n", cmdName)

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(gparams.StatePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	fmt.Printf("cmd=%s info=image id=%v size.bytes=%v size.human=%v\n",
		cmdName,
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	fmt.Printf("cmd=%s state=image.inspection.done\n", cmdName)
	fmt.Printf("cmd=%s state=container.inspection.start\n", cmdName)

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
		includePaths,
		includeBins,
		includeExes,
		doIncludeShell,
		gparams.Debug,
		gparams.InContainer,
		true,
		prefix)
	errutil.FailOn(err)

	if len(containerInspector.FatContainerCmd) == 0 {
		fmt.Printf("cmd=%s info=target.image.error status=no.entrypoint.cmd image='%v' message='no ENTRYPOINT/CMD'\n", cmdName, targetRef)
		fmt.Printf("cmd=%s state=exited\n", cmdName)
		commands.Exit(commands.ECTBuild | ecpNoEntrypoint)
	}

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutil.FailOn(err)

	fmt.Printf("cmd=%s info=container name=%v id=%v target.port.list=[%v] target.port.info=[%v] message='YOU CAN USE THESE PORTS TO INTERACT WITH THE CONTAINER'\n",
		cmdName,
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
			httpProbeAPISpecs,
			httpProbeAPISpecFiles,
			true, prefix)
		errutil.FailOn(err)
		if len(probe.Ports) == 0 {
			fmt.Printf("cmd=%s state=http.probe.error error='no exposed ports' message='expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services\n", cmdName)
			logger.Info("shutting down 'fat' container...")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()

			fmt.Printf("cmd=%s state=exited\n", cmdName)
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

	fmt.Printf("cmd=%s info=continue.after mode=%v message='%v'\n", cmdName, continueAfter.Mode, continueAfterMsg)

	switch continueAfter.Mode {
	case "enter":
		fmt.Printf("cmd=%s info=prompt message='USER INPUT REQUIRED, PRESS <ENTER> WHEN YOU ARE DONE USING THE CONTAINER'\n", cmdName)
		creader := bufio.NewReader(os.Stdin)
		_, _, _ = creader.ReadLine()
	case "signal":
		fmt.Printf("cmd=%s info=prompt message='send SIGUSR1 when you are done using the container'\n", cmdName)
		<-continueAfter.ContinueChan
		fmt.Printf("cmd=%s info=event message='got SIGUSR1'\n", cmdName)
	case "timeout":
		fmt.Printf("cmd=%s info=prompt message='waiting for the target container (%v seconds)'\n", cmdName, int(continueAfter.Timeout))
		<-time.After(time.Second * continueAfter.Timeout)
		fmt.Printf("cmd=%s info=event message='done waiting for the target container'\n", cmdName)
	case "probe":
		fmt.Printf("cmd=%s info=prompt message='waiting for the HTTP probe to finish'\n", cmdName)
		<-continueAfter.ContinueChan
		fmt.Printf("cmd=%s info=event message='HTTP probe is done'\n", cmdName)
	default:
		errutil.Fail("unknown continue-after mode")
	}

	fmt.Printf("cmd=%s state=container.inspection.finishing\n", cmdName)

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutil.WarnOn(err)

	fmt.Printf("cmd=%s state=container.inspection.artifact.processing\n", cmdName)

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		fmt.Printf("cmd=%s info=results status='no data collected (no minified image generated). (version=%v location='%v')'\n",
			cmdName,
			v.Current(), fsutil.ExeDir())
		fmt.Printf("cmd=%s state=exited\n", cmdName)
		return
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutil.FailOn(err)

	fmt.Printf("cmd=%s state=container.inspection.done\n", cmdName)
	fmt.Printf("cmd=%s state=completed\n", cmdName)
	cmdReport.State = command.StateCompleted

	if copyMetaArtifactsLocation != "" {
		toCopy := []string{
			report.DefaultContainerReportFileName,
			imageInspector.SeccompProfileName,
			imageInspector.AppArmorProfileName,
		}
		if !commands.CopyMetaArtifacts(logger,
			toCopy,
			artifactLocation, copyMetaArtifactsLocation) {
			fmt.Printf("cmd=%s info=artifacts message='could not copy meta artifacts'\n", cmdName)
		}
	}

	if err := commands.DoArchiveState(logger, client, artifactLocation, gparams.ArchiveState, stateKey); err != nil {
		fmt.Printf("cmd=%s info=state message='could not archive state'\n", cmdName)
		logger.Errorf("error archiving state - %v", err)
	}

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutil.Remove(artifactLocation)
		errutil.WarnOn(err)
	}

	fmt.Printf("cmd=%s state=done\n", cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		fmt.Printf("cmd=%s info=report file='%s'\n", cmdName, cmdReport.ReportLocation())
	}
}
