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
	"github.com/docker-slim/docker-slim/pkg/utils/errutils"
	"github.com/docker-slim/docker-slim/pkg/utils/fsutils"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/Sirupsen/logrus"
	"github.com/dustin/go-humanize"
)

// OnProfile implements the 'profile' docker-slim command
func OnProfile(doDebug bool,
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

	fmt.Printf("docker-slim: [profile] image=%v\n", imageRef)
	doRmFileArtifacts := false

	client := dockerclient.New(clientConfig)

	if doDebug {
		version.Print(client)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutils.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Println("docker-slim: [profile] target image not found -", imageRef)
		return
	}

	log.Info("docker-slim: inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutils.FailOn(err)

	localVolumePath, artifactLocation := fsutils.PrepareStateDirs(statePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation

	log.Infof("docker-slim: [%v] 'fat' image size => %v (%v)",
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	log.Info("docker-slim: processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutils.FailOn(err)

	containerInspector, err := container.NewInspector(client,
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
		doDebug)
	errutils.FailOn(err)

	log.Info("docker-slim: starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutils.FailOn(err)

	log.Info("docker-slim: watching container monitor...")

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
		fmt.Println("docker-slim: press <enter> when you are done using the container...")
		creader := bufio.NewReader(os.Stdin)
		_, _, _ = creader.ReadLine()
	case "signal":
		fmt.Println("docker-slim: send SIGUSR1 when you are done using the container...")
		<-continueAfter.ContinueChan
		fmt.Println("docker-slim: got SIGUSR1...")
	case "timeout":
		fmt.Printf("docker-slim: waiting for the target container (%v seconds)...\n", int(continueAfter.Timeout))
		<-time.After(time.Second * continueAfter.Timeout)
		fmt.Printf("docker-slim: done waiting for the target container...")
	case "probe":
		fmt.Println("docker-slim: waiting for the HTTP probe to finish...")
		<-continueAfter.ContinueChan
		fmt.Println("docker-slim: HTTP probe is done...")
	default:
		errutils.Fail("unknown continue-after mode")
	}

	containerInspector.FinishMonitoring()

	log.Info("docker-slim: shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutils.WarnOn(err)

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		fmt.Printf("docker-slim: [profile] no data collected (no minified image generated) - done. (version: %v)\n", v.Current())
		return
	}

	log.Info("docker-slim: processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutils.FailOn(err)

	if doRmFileArtifacts {
		log.Info("docker-slim: removing temporary artifacts...")
		err = fsutils.Remove(artifactLocation) //TODO: remove only the "files" subdirectory
		errutils.WarnOn(err)
	}

	fmt.Println("docker-slim: [profile] done.")
}
