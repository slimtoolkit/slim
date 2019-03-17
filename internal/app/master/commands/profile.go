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
	"github.com/docker-slim/docker-slim/pkg/utils/errutils"
	"github.com/docker-slim/docker-slim/pkg/utils/fsutils"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/Sirupsen/logrus"
	"github.com/dustin/go-humanize"
)

// OnProfile implements the 'profile' docker-slim command
func OnProfile(
	cmdReportLocation string,
	doDebug bool,
	statePath string,
	clientConfig *config.DockerClient,
	imageRef string,
	doHTTPProbe bool,
	httpProbeCmds []config.HTTPProbeCmd,
	doShowContainerLogs bool,
	overrides *config.ContainerOverrides,
	links []string,
	etcHostsMaps []string,
	dnsServers []string,
	dnsSearchDomains []string,
	volumeMounts map[string]config.VolumeMount,
	excludePaths map[string]bool,
	includePaths map[string]bool,
	continueAfter *config.ContinueAfter) {
	logger := log.WithFields(log.Fields{"app": "docker-slim", "command": "profile"})

	cmdReport := report.NewProfileCommand(cmdReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.OriginalImage = imageRef

	fmt.Println("docker-slim[profile]: state=started")
	fmt.Printf("docker-slim[profile]: info=params target=%v\n", imageRef)
	doRmFileArtifacts := false

	client := dockerclient.New(clientConfig)

	if doDebug {
		version.Print(client)
	}

	if !confirmNetwork(logger, client, overrides.Network) {
		fmt.Printf("docker-slim[profile]: info=param.error status=unknown.network value=%s\n", overrides.Network)
		fmt.Printf("docker-slim[profile]: state=exited version=%s\n", v.Current())
		os.Exit(-111)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutils.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Println("docker-slim[profile]: target image not found -", imageRef)
		fmt.Println("docker-slim[profile]: state=exited")
		return
	}

	fmt.Println("docker-slim[profile]: state=inspecting.image")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutils.FailOn(err)

	localVolumePath, artifactLocation, statePath := fsutils.PrepareStateDirs(statePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation

	fmt.Printf("docker-slim[profile]: info=image id=%v size.bytes=%v size.human=%v\n",
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutils.FailOn(err)

	fmt.Println("docker-slim[profile]: state=inspecting.container")

	containerInspector, err := container.NewInspector(client,
		statePath,
		imageInspector,
		localVolumePath,
		overrides,
		links,
		etcHostsMaps,
		dnsServers,
		dnsSearchDomains,
		doShowContainerLogs,
		volumeMounts,
		excludePaths,
		includePaths,
		doDebug,
		true,
		"docker-slim[profile]:")
	errutils.FailOn(err)

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutils.FailOn(err)

	logger.Info("watching container monitor...")

	if "probe" == continueAfter.Mode {
		doHTTPProbe = true
	}

	if doHTTPProbe {
		probe, err := http.NewCustomProbe(containerInspector, httpProbeCmds, true, "docker-slim[profile]:")
		errutils.FailOn(err)
		probe.Start()
		continueAfter.ContinueChan = probe.DoneChan()
	}

	switch continueAfter.Mode {
	case "enter":
		fmt.Println("docker-slim[profile]: info=prompt message='press <enter> when you are done using the container'")
		creader := bufio.NewReader(os.Stdin)
		_, _, _ = creader.ReadLine()
	case "signal":
		fmt.Println("docker-slim[profile]: info=prompt message='send SIGUSR1 when you are done using the container'")
		<-continueAfter.ContinueChan
		fmt.Println("docker-slim[profile]: info=event message='got SIGUSR1'")
	case "timeout":
		fmt.Printf("docker-slim[profile]: info=prompt message='waiting for the target container (%v seconds)'\n", int(continueAfter.Timeout))
		<-time.After(time.Second * continueAfter.Timeout)
		fmt.Printf("docker-slim[profile]: info=event message='done waiting for the target container'")
	case "probe":
		fmt.Println("docker-slim[profile]: info=prompt message='waiting for the HTTP probe to finish'")
		<-continueAfter.ContinueChan
		fmt.Println("docker-slim[profile]: info=event message='HTTP probe is done'")
	default:
		errutils.Fail("unknown continue-after mode")
	}

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutils.WarnOn(err)

	fmt.Println("docker-slim[profile]: state=processing")

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		fmt.Printf("docker-slim[profile]: info=results status='no data collected (no minified image generated). (version: %v)'\n",
			v.Current())
		fmt.Println("docker-slim[profile]: state=exited")
		return
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutils.FailOn(err)

	fmt.Println("docker-slim[profile]: state=completed")
	cmdReport.State = report.CmdStateCompleted

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutils.Remove(artifactLocation) //TODO: remove only the "files" subdirectory
		errutils.WarnOn(err)
	}

	fmt.Println("docker-slim[profile]: state=done")
	cmdReport.State = report.CmdStateDone
	cmdReport.Save()
}
