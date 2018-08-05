package commands

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/builder"
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

// OnBuild implements the 'build' docker-slim command
func OnBuild(doDebug bool,
	statePath string,
	clientConfig *config.DockerClient,
	imageRef string,
	customImageTag string,
	doHTTPProbe bool,
	httpProbeCmds []config.HTTPProbeCmd,
	doRmFileArtifacts bool,
	doShowContainerLogs bool,
	doShowBuildLogs bool,
	imageOverrides map[string]bool,
	overrides *config.ContainerOverrides,
	links []string,
	etcHostsMaps []string,
	volumeMounts map[string]config.VolumeMount,
	excludePaths map[string]bool,
	includePaths map[string]bool,
	continueAfter *config.ContinueAfter) {
	logger := log.WithFields(log.Fields{"app": "docker-slim", "command": "build"})

	fmt.Println("docker-slim[build]: state=started")

	fmt.Printf("docker-slim[build]: info=params target=%v continue.mode=%v\n", imageRef, continueAfter.Mode)

	logger.Infof("image=%v http-probe=%v remove-file-artifacts=%v image-overrides=%+v entrypoint=%+v (%v) cmd=%+v (%v) workdir='%v' env=%+v expose=%+v",
		imageRef, doHTTPProbe, doRmFileArtifacts,
		imageOverrides,
		overrides.Entrypoint, overrides.ClearEntrypoint, overrides.Cmd, overrides.ClearCmd,
		overrides.Workdir, overrides.Env, overrides.ExposedPorts)

	client := dockerclient.New(clientConfig)

	if doDebug {
		version.Print(client)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutils.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Println("docker-slim[build]: target image not found -", imageRef)
		fmt.Println("docker-slim[build]: state=exited")
		return
	}

	fmt.Println("docker-slim[build]: state=inspecting.image")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutils.FailOn(err)

	localVolumePath, artifactLocation := fsutils.PrepareStateDirs(statePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation

	fmt.Printf("docker-slim[build]: info=image id=%v size.bytes=%v size.human=%v\n",
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutils.FailOn(err)

	fmt.Println("docker-slim[build]: state=inspecting.container")

	containerInspector, err := container.NewInspector(client,
		imageInspector,
		localVolumePath,
		overrides,
		links,
		etcHostsMaps,
		doShowContainerLogs,
		volumeMounts,
		excludePaths,
		includePaths,
		doDebug)
	errutils.FailOn(err)

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutils.FailOn(err)

	logger.Info("watching container monitor...")

	if "probe" == continueAfter.Mode {
		doHTTPProbe = true
	}

	if doHTTPProbe {
		probe, err := http.NewCustomProbe(containerInspector, httpProbeCmds, true, "docker-slim[build]:")
		errutils.FailOn(err)
		probe.Start()
		continueAfter.ContinueChan = probe.DoneChan()
	}

	switch continueAfter.Mode {
	case "enter":
		fmt.Println("docker-slim[build]: info=prompt message='press <enter> when you are done using the container'")
		creader := bufio.NewReader(os.Stdin)
		_, _, _ = creader.ReadLine()
	case "signal":
		fmt.Println("docker-slim[build]: info=prompt message='send SIGUSR1 when you are done using the container'")
		<-continueAfter.ContinueChan
		fmt.Println("docker-slim[build]: info=event message='got SIGUSR1'")
	case "timeout":
		fmt.Printf("docker-slim[build]: info=prompt message='waiting for the target container (%v seconds)'\n", int(continueAfter.Timeout))
		<-time.After(time.Second * continueAfter.Timeout)
		fmt.Printf("docker-slim[build]: info=event message='done waiting for the target container'")
	case "probe":
		fmt.Println("docker-slim[build]: info=prompt message='waiting for the HTTP probe to finish'")
		<-continueAfter.ContinueChan
		fmt.Println("docker-slim[build]: info=event message='HTTP probe is done'")
	default:
		errutils.Fail("unknown continue-after mode")
	}

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutils.WarnOn(err)

	fmt.Println("docker-slim[build]: state=processing")

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		fmt.Printf("docker-slim[build]: info=results status='no data collected (no minified image generated). (version: %v)'\n",
			v.Current())
		fmt.Println("docker-slim[build]: state=exited")
		return
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutils.FailOn(err)

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	fmt.Println("docker-slim[build]: state=building message='building minified image'")

	builder, err := builder.NewImageBuilder(client,
		customImageTag,
		imageInspector.ImageInfo,
		artifactLocation,
		doShowBuildLogs,
		imageOverrides,
		overrides)
	errutils.FailOn(err)

	if !builder.HasData {
		logger.Info("WARNING - no data artifacts")
	}

	err = builder.Build()

	if doShowBuildLogs {
		fmt.Println("docker-slim: [build] - build logs ====================")
		fmt.Println("%s", builder.BuildLog.String())
		fmt.Println("docker-slim: [build] - end of build logs =============")
	}

	errutils.FailOn(err)

	fmt.Println("docker-slim[build]: state=completed")

	/////////////////////////////
	newImageInspector, err := image.NewInspector(client, builder.RepoName)
	errutils.FailOn(err)

	if newImageInspector.NoImage() {
		fmt.Printf("docker-slim[build]: info=results message='minified image not found - %s'\n", builder.RepoName)
		fmt.Println("docker-slim[build]: state=exited")
		return
	}

	err = newImageInspector.Inspect()
	errutils.WarnOn(err)

	if err == nil {
		x := float64(imageInspector.ImageInfo.VirtualSize) / float64(newImageInspector.ImageInfo.VirtualSize)
		fmt.Printf("docker-slim[build]: info=results status='MINIFIED BY %.2fX [%v (%v) => %v (%v)]'\n",
			x,
			imageInspector.ImageInfo.VirtualSize,
			humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
			newImageInspector.ImageInfo.VirtualSize,
			humanize.Bytes(uint64(newImageInspector.ImageInfo.VirtualSize)))
	}

	fmt.Printf("docker-slim[build]: info=results  image.name=%v image.size='%v' data=%v\n",
		builder.RepoName,
		humanize.Bytes(uint64(newImageInspector.ImageInfo.VirtualSize)),
		builder.HasData)

	fmt.Printf("docker-slim[build]: info=results  artifacts.location='%v'\n", imageInspector.ArtifactLocation)
	fmt.Printf("docker-slim[build]: info=results  artifacts.report=%v\n", report.DefaultContainerReportFileName)
	fmt.Printf("docker-slim[build]: info=results  artifacts.dockerfile.original=Dockerfile.fat\n")
	fmt.Printf("docker-slim[build]: info=results  artifacts.dockerfile.new=Dockerfile\n")
	fmt.Printf("docker-slim[build]: info=results  artifacts.seccomp=%v\n", imageInspector.SeccompProfileName)
	fmt.Printf("docker-slim[build]: info=results  artifacts.apparmor=%v\n", imageInspector.AppArmorProfileName)

	/////////////////////////////

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutils.Remove(artifactLocation) //TODO: remove only the "files" subdirectory
		errutils.WarnOn(err)
	}

	fmt.Println("docker-slim[build]: state=done")
}
