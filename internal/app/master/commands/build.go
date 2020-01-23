package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

// Build command exit codes
const (
	ecbOther = iota + 1
	ecbBadCustomImageTag
)

// OnBuild implements the 'build' docker-slim command
func OnBuild(
	doCheckVersion bool,
	cmdReportLocation string,
	doDebug bool,
	statePath string,
	archiveState string,
	inContainer bool,
	isDSImage bool,
	clientConfig *config.DockerClient,
	buildFromDockerfile string,
	imageRef string,
	customImageTag string,
	fatImageTag string,
	doHTTPProbe bool,
	httpProbeCmds []config.HTTPProbeCmd,
	httpProbeRetryCount int,
	httpProbeRetryWait int,
	httpProbePorts []uint16,
	doHTTPProbeFull bool,
	doRmFileArtifacts bool,
	copyMetaArtifactsLocation string,
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
	doUseLocalMounts bool,
	doUseSensorVolume string,
	doKeepTmpArtifacts bool,
	continueAfter *config.ContinueAfter) {
	const cmdName = "build"
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(doCheckVersion, inContainer, isDSImage)

	cmdReport := report.NewBuildCommand(cmdReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.ImageReference = imageRef

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("%s[%s]: info=docker.connect.error message='%s'\n", appName, cmdName, exitMsg)
		fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
		os.Exit(ectCommon | ecNoDockerConnectInfo)
	}
	errutil.FailOn(err)

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)

	if buildFromDockerfile == "" {
		fmt.Printf("%s[%s]: info=params target=%v continue.mode=%v\n", appName, cmdName, imageRef, continueAfter.Mode)
	} else {
		fmt.Printf("%s[%s]: info=params context=%v/file=%v continue.mode=%v\n", appName, cmdName, imageRef, buildFromDockerfile, continueAfter.Mode)
	}

	if buildFromDockerfile != "" {
		fmt.Printf("%s[%s]: state=building message='building basic image'\n", appName, cmdName)
		//create a fat image name:
		//* use the explicit fat image tag if provided
		//* or create one based on the user provided (slim image) custom tag if it's available
		//* otherwise auto-generate a name
		var fatImageRepoNameTag string
		if fatImageTag != "" {
			fatImageRepoNameTag = fatImageTag
		} else if customImageTag != "" {
			citParts := strings.Split(customImageTag, ":")
			switch len(citParts) {
			case 1:
				fatImageRepoNameTag = fmt.Sprintf("%s.fat", customImageTag)
			case 2:
				fatImageRepoNameTag = fmt.Sprintf("%s.fat:%s", citParts[0], citParts[1])
			default:
				fmt.Printf("%s[%s]: info=param.error status=malformed.custom.image.tag value=%s\n", appName, cmdName, customImageTag)
				fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
				os.Exit(ectBuild | ecbBadCustomImageTag)
			}
		} else {
			fatImageRepoNameTag = fmt.Sprintf("docker-slim-tmp-fat-image.%v.%v",
				os.Getpid(), time.Now().UTC().Format("20060102150405"))
		}

		fmt.Printf("%s[%s]: info=basic.image.name value=%s\n", appName, cmdName, fatImageRepoNameTag)

		fatBuilder, err := builder.NewBasicImageBuilder(client,
			fatImageRepoNameTag,
			buildFromDockerfile,
			imageRef,
			doShowBuildLogs)
		errutil.FailOn(err)

		err = fatBuilder.Build()

		if doShowBuildLogs {
			fmt.Printf("%s[%s]: build logs (basic image) ====================\n", appName, cmdName)
			fmt.Println(fatBuilder.BuildLog.String())
			fmt.Printf("%s[%s]: end of build logs (basic image) =============\n", appName, cmdName)
		}

		errutil.FailOn(err)

		fmt.Printf("%s[%s]: state=basic.image.build.completed\n", appName, cmdName)

		imageRef = fatImageRepoNameTag
		//todo: remove the temporary fat image (should have a flag for it in case users want the fat image too)
	}

	logger.Infof("image=%v http-probe=%v remove-file-artifacts=%v image-overrides=%+v entrypoint=%+v (%v) cmd=%+v (%v) workdir='%v' env=%+v expose=%+v",
		imageRef, doHTTPProbe, doRmFileArtifacts,
		imageOverrideSelectors,
		overrides.Entrypoint, overrides.ClearEntrypoint, overrides.Cmd, overrides.ClearCmd,
		overrides.Workdir, overrides.Env, overrides.ExposedPorts)

	if doDebug {
		version.Print(prefix, logger, client, false, inContainer, isDSImage)
	}

	if !confirmNetwork(logger, client, overrides.Network) {
		fmt.Printf("%s[%s]: info=param.error status=unknown.network value=%s\n", appName, cmdName, overrides.Network)
		fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
		os.Exit(ectCommon | ecBadNetworkName)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Printf("%s[%s]: info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", appName, cmdName, imageRef)
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

	fmt.Printf("%s[%s]: state=image.inspection.start\n", appName, cmdName)

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(statePath, imageInspector.ImageInfo.ID)
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

	if imageInspector.DockerfileInfo != nil {
		if imageInspector.DockerfileInfo.ExeUser != "" {
			fmt.Printf("%s[%s]: info=image.users exe='%v' all='%v'\n",
				appName, cmdName,
				imageInspector.DockerfileInfo.ExeUser,
				strings.Join(imageInspector.DockerfileInfo.AllUsers, ","))
		}

		if len(imageInspector.DockerfileInfo.ImageStack) > 0 {
			cmdReport.ImageStack = imageInspector.DockerfileInfo.ImageStack

			for idx, layerInfo := range imageInspector.DockerfileInfo.ImageStack {
				fmt.Printf("%s[%s]: info=image.stack index=%v name='%v' id='%v'\n",
					appName, cmdName, idx, layerInfo.FullName, layerInfo.ID)
			}
		}

		if len(imageInspector.DockerfileInfo.ExposedPorts) > 0 {
			fmt.Printf("%s[%s]: info=image.exposed_ports list='%v'\n", appName, cmdName,
				strings.Join(imageInspector.DockerfileInfo.ExposedPorts, ","))
		}
	}

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
		doShowContainerLogs,
		volumeMounts,
		excludePaths,
		includePaths,
		includeBins,
		includeExes,
		doIncludeShell,
		doDebug,
		inContainer,
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
		probe, err := http.NewCustomProbe(containerInspector, httpProbeCmds,
			httpProbeRetryCount, httpProbeRetryWait, httpProbePorts, doHTTPProbeFull,
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
		fmt.Printf("%s[%s]: info=results status='no data collected (no minified image generated). (version=%v location='%s')'\n",
			appName, cmdName,
			v.Current(), fsutil.ExeDir())
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutil.FailOn(err)

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	fmt.Printf("%s[%s]: state=container.inspection.done\n", appName, cmdName)
	fmt.Printf("%s[%s]: state=building message='building minified image'\n", appName, cmdName)

	builder, err := builder.NewImageBuilder(client,
		customImageTag,
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

	if doShowBuildLogs {
		fmt.Printf("%s[%s]: build logs ====================\n", appName, cmdName)
		fmt.Println(builder.BuildLog.String())
		fmt.Printf("%s[%s]: end of build logs =============\n", appName, cmdName)
	}

	errutil.FailOn(err)

	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = report.CmdStateCompleted

	/////////////////////////////
	newImageInspector, err := image.NewInspector(client, builder.RepoName)
	errutil.FailOn(err)

	if newImageInspector.NoImage() {
		fmt.Printf("%s[%s]: info=results message='minified image not found - %s'\n", appName, cmdName, builder.RepoName)
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

	err = newImageInspector.Inspect()
	errutil.WarnOn(err)

	if err == nil {
		cmdReport.MinifiedBy = float64(imageInspector.ImageInfo.VirtualSize) / float64(newImageInspector.ImageInfo.VirtualSize)

		cmdReport.SourceImage = report.ImageMetadata{
			AllNames:      imageInspector.ImageRecordInfo.RepoTags,
			ID:            imageInspector.ImageRecordInfo.ID,
			Size:          imageInspector.ImageInfo.VirtualSize,
			SizeHuman:     humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)),
			CreateTime:    imageInspector.ImageInfo.Created.UTC().Format(time.RFC3339),
			Author:        imageInspector.ImageInfo.Author,
			DockerVersion: imageInspector.ImageInfo.DockerVersion,
			Architecture:  imageInspector.ImageInfo.Architecture,
			User:          imageInspector.ImageInfo.Config.User,
		}

		if len(imageInspector.ImageRecordInfo.RepoTags) > 0 {
			cmdReport.SourceImage.Name = imageInspector.ImageRecordInfo.RepoTags[0]
		}

		if len(imageInspector.ImageInfo.Config.ExposedPorts) > 0 {
			for k := range imageInspector.ImageInfo.Config.ExposedPorts {
				cmdReport.SourceImage.ExposedPorts = append(cmdReport.SourceImage.ExposedPorts, string(k))
			}
		}

		cmdReport.MinifiedImageSize = newImageInspector.ImageInfo.VirtualSize
		cmdReport.MinifiedImageSizeHuman = humanize.Bytes(uint64(newImageInspector.ImageInfo.VirtualSize))

		fmt.Printf("%s[%s]: info=results status='MINIFIED BY %.2fX [%v (%v) => %v (%v)]'\n",
			appName, cmdName,
			cmdReport.MinifiedBy,
			cmdReport.SourceImage.Size,
			cmdReport.SourceImage.SizeHuman,
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

	fmt.Printf("%s[%s]: info=results  image.name=%v image.size='%v' data=%v\n",
		appName, cmdName,
		cmdReport.MinifiedImage,
		cmdReport.MinifiedImageSizeHuman,
		cmdReport.MinifiedImageHasData)

	fmt.Printf("%s[%s]: info=results  artifacts.location='%v'\n", appName, cmdName, cmdReport.ArtifactLocation)
	fmt.Printf("%s[%s]: info=results  artifacts.report=%v\n", appName, cmdName, cmdReport.ContainerReportName)
	fmt.Printf("%s[%s]: info=results  artifacts.dockerfile.original=Dockerfile.fat\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=results  artifacts.dockerfile.new=Dockerfile\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=results  artifacts.seccomp=%v\n", appName, cmdName, cmdReport.SeccompProfileName)
	fmt.Printf("%s[%s]: info=results  artifacts.apparmor=%v\n", appName, cmdName, cmdReport.AppArmorProfileName)

	if cmdReport.ArtifactLocation != "" {
		creportPath := filepath.Join(cmdReport.ArtifactLocation, cmdReport.ContainerReportName)
		if creportData, err := ioutil.ReadFile(creportPath); err == nil {
			var creport report.ContainerReport
			if err := json.Unmarshal(creportData, &creport); err == nil {
				cmdReport.System = report.SystemMetadata{
					Type:    creport.System.Type,
					Release: creport.System.Release,
					OS:      creport.System.OS,
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
		if !copyMetaArtifacts(logger,
			toCopy,
			artifactLocation, copyMetaArtifactsLocation) {
			fmt.Printf("%s[%s]: info=artifacts message='could not copy meta artifacts'\n", appName, cmdName)
		}
	}

	if err := doArchiveState(logger, client, artifactLocation, archiveState, stateKey); err != nil {
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
