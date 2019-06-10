package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/builder"
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

	log "github.com/Sirupsen/logrus"
	"github.com/dustin/go-humanize"
)

// OnBuild implements the 'build' docker-slim command
func OnBuild(
	doCheckVersion bool,
	cmdReportLocation string,
	doDebug bool,
	statePath string,
	clientConfig *config.DockerClient,
	imageRef string,
	customImageTag string,
	doHTTPProbe bool,
	httpProbeCmds []config.HTTPProbeCmd,
	httpProbeRetryCount int,
	httpProbeRetryWait int,
	httpProbePorts []uint16,
	doHTTPProbeFull bool,
	doRmFileArtifacts bool,
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
	excludePaths map[string]bool,
	includePaths map[string]bool,
	includeBins map[string]bool,
	includeExes map[string]bool,
	doIncludeShell bool,
	continueAfter *config.ContinueAfter) {
	logger := log.WithFields(log.Fields{"app": "docker-slim", "command": "build"})

	viChan := version.CheckAsync(doCheckVersion)

	cmdReport := report.NewBuildCommand(cmdReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.OriginalImage = imageRef

	fmt.Println("docker-slim[build]: state=started")
	fmt.Printf("docker-slim[build]: info=params target=%v continue.mode=%v\n", imageRef, continueAfter.Mode)

	logger.Infof("image=%v http-probe=%v remove-file-artifacts=%v image-overrides=%+v entrypoint=%+v (%v) cmd=%+v (%v) workdir='%v' env=%+v expose=%+v",
		imageRef, doHTTPProbe, doRmFileArtifacts,
		imageOverrideSelectors,
		overrides.Entrypoint, overrides.ClearEntrypoint, overrides.Cmd, overrides.ClearCmd,
		overrides.Workdir, overrides.Env, overrides.ExposedPorts)

	client := dockerclient.New(clientConfig)

	if doDebug {
		version.Print(client, false)
	}

	if !confirmNetwork(logger, client, overrides.Network) {
		fmt.Printf("docker-slim[build]: info=param.error status=unknown.network value=%s\n", overrides.Network)
		fmt.Printf("docker-slim[build]: state=exited version=%s\n", v.Current())
		os.Exit(-111)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Println("docker-slim[build]: target image not found -", imageRef)
		fmt.Println("docker-slim[build]: state=exited")
		return
	}

	fmt.Println("docker-slim[build]: state=inspecting.image")

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath := fsutil.PrepareImageStateDirs(statePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation

	fmt.Printf("docker-slim[build]: info=image id=%v size.bytes=%v size.human=%v\n",
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	if imageInspector.DockerfileInfo != nil {
		if imageInspector.DockerfileInfo.ExeUser != "" {
			fmt.Printf("docker-slim[build]: info=image.users exe='%v' all='%v'\n",
				imageInspector.DockerfileInfo.ExeUser,
				strings.Join(imageInspector.DockerfileInfo.AllUsers, ","))
		}

		if len(imageInspector.DockerfileInfo.Layers) > 0 {
			for idx, layerInfo := range imageInspector.DockerfileInfo.Layers {
				fmt.Printf("docker-slim[build]: info=image.layers index=%v name='%v' tags='%v'\n",
					idx, layerInfo.Name, strings.Join(layerInfo.Tags, ","))
			}
		}

		if len(imageInspector.DockerfileInfo.ExposedPorts) > 0 {
			fmt.Printf("docker-slim[build]: info=image.exposed_ports list='%v'\n",
				strings.Join(imageInspector.DockerfileInfo.ExposedPorts, ","))
		}
	}

	fmt.Println("docker-slim[build]: state=inspecting.container")

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
		includeBins,
		includeExes,
		doIncludeShell,
		doDebug,
		true,
		"docker-slim[build]:")
	errutil.FailOn(err)

	logger.Info("starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	errutil.FailOn(err)

	fmt.Printf("docker-slim[build]: info=container name=%v id=%v target.port.list=[%v] target.port.info=[%v]\n",
		containerInspector.ContainerName,
		containerInspector.ContainerID,
		containerInspector.ContainerPortList,
		containerInspector.ContainerPortsInfo)

	logger.Info("watching container monitor...")

	if "probe" == continueAfter.Mode {
		doHTTPProbe = true
	}

	if doHTTPProbe {
		probe, err := http.NewCustomProbe(containerInspector, httpProbeCmds,
			httpProbeRetryCount, httpProbeRetryWait, httpProbePorts, doHTTPProbeFull,
			true, "docker-slim[build]:")
		errutil.FailOn(err)
		if len(probe.Ports) == 0 {
			fmt.Println("docker-slim[build]: state=http.probe.error error='no exposed ports' message='expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services")
			logger.Info("shutting down 'fat' container...")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()

			fmt.Println("docker-slim[build]: state=exited")
			return
		}

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
		errutil.Fail("unknown continue-after mode")
	}

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutil.WarnOn(err)

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
	errutil.FailOn(err)

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	fmt.Println("docker-slim[build]: state=building message='building minified image'")

	builder, err := builder.NewImageBuilder(client,
		customImageTag,
		imageInspector.ImageInfo,
		artifactLocation,
		doShowBuildLogs,
		imageOverrideSelectors,
		overrides,
		instructions)
	errutil.FailOn(err)

	/*
	   func NewImageBuilder(client *docker.Client,
	   	imageRepoName string,
	   	imageInfo *docker.Image,
	   	artifactLocation string,
	   	showBuildLogs bool,
	   	imageOverrides map[string]bool,
	   	overrides *config.ContainerOverrides,
	   	instructions *config.ImageNewInstructions)
	*/

	if !builder.HasData {
		logger.Info("WARNING - no data artifacts")
	}

	err = builder.Build()

	if doShowBuildLogs {
		fmt.Println("docker-slim[build]: build logs ====================")
		fmt.Println(builder.BuildLog.String())
		fmt.Println("docker-slim[build]: end of build logs =============")
	}

	errutil.FailOn(err)

	fmt.Println("docker-slim[build]: state=completed")
	cmdReport.State = report.CmdStateCompleted

	/////////////////////////////
	newImageInspector, err := image.NewInspector(client, builder.RepoName)
	errutil.FailOn(err)

	if newImageInspector.NoImage() {
		fmt.Printf("docker-slim[build]: info=results message='minified image not found - %s'\n", builder.RepoName)
		fmt.Println("docker-slim[build]: state=exited")
		return
	}

	err = newImageInspector.Inspect()
	errutil.WarnOn(err)

	if err == nil {
		cmdReport.MinifiedBy = float64(imageInspector.ImageInfo.VirtualSize) / float64(newImageInspector.ImageInfo.VirtualSize)
		cmdReport.OriginalImageSize = imageInspector.ImageInfo.VirtualSize
		cmdReport.OriginalImageSizeHuman = humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize))
		cmdReport.MinifiedImageSize = newImageInspector.ImageInfo.VirtualSize
		cmdReport.MinifiedImageSizeHuman = humanize.Bytes(uint64(newImageInspector.ImageInfo.VirtualSize))

		fmt.Printf("docker-slim[build]: info=results status='MINIFIED BY %.2fX [%v (%v) => %v (%v)]'\n",
			cmdReport.MinifiedBy,
			cmdReport.OriginalImageSize,
			cmdReport.OriginalImageSizeHuman,
			cmdReport.MinifiedImageSize,
			cmdReport.MinifiedImageSizeHuman)
	} else {
		cmdReport.State = report.CmdStateError
		cmdReport.Error = err.Error()
	}

	cmdReport.MinifiedImage = builder.RepoName
	cmdReport.MinifiedImageHasData = builder.HasData
	cmdReport.ArtifactLocation = imageInspector.ArtifactLocation
	cmdReport.ContainerReportName = report.DefaultContainerReportFileName
	cmdReport.SeccompProfileName = imageInspector.SeccompProfileName
	cmdReport.AppArmorProfileName = imageInspector.AppArmorProfileName

	fmt.Printf("docker-slim[build]: info=results  image.name=%v image.size='%v' data=%v\n",
		cmdReport.MinifiedImage,
		cmdReport.MinifiedImageSizeHuman,
		cmdReport.MinifiedImageHasData)

	fmt.Printf("docker-slim[build]: info=results  artifacts.location='%v'\n", cmdReport.ArtifactLocation)
	fmt.Printf("docker-slim[build]: info=results  artifacts.report=%v\n", cmdReport.ContainerReportName)
	fmt.Printf("docker-slim[build]: info=results  artifacts.dockerfile.original=Dockerfile.fat\n")
	fmt.Printf("docker-slim[build]: info=results  artifacts.dockerfile.new=Dockerfile\n")
	fmt.Printf("docker-slim[build]: info=results  artifacts.seccomp=%v\n", cmdReport.SeccompProfileName)
	fmt.Printf("docker-slim[build]: info=results  artifacts.apparmor=%v\n", cmdReport.AppArmorProfileName)

	/////////////////////////////

	if doRmFileArtifacts {
		logger.Info("removing temporary artifacts...")
		err = fsutil.Remove(artifactLocation) //TODO: remove only the "files" subdirectory
		errutil.WarnOn(err)
	}

	fmt.Println("docker-slim[build]: state=done")

	vinfo := <-viChan
	version.PrintCheckVersion(vinfo)

	cmdReport.State = report.CmdStateDone
	cmdReport.Save()
}
