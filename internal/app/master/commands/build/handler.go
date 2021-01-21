package build

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/builder"
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
	"github.com/docker-slim/docker-slim/pkg/util/printbuffer"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	"github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

// Build command exit codes
const (
	ecbOther = iota + 1
	ecbBadCustomImageTag
	ecbImageBuildError
	ecbNoEntrypoint
)

// OnCommand implements the 'build' docker-slim command
func OnCommand(
	gparams *commands.GenericParams,
	targetRef string,
	buildFromDockerfile string,
	customImageTag string,
	fatImageTag string,
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
	includePaths map[string]*fsutil.AccessInfo,
	includeBins map[string]*fsutil.AccessInfo,
	includeExes map[string]*fsutil.AccessInfo,
	doIncludeShell bool,
	doUseLocalMounts bool,
	doUseSensorVolume string,
	doKeepTmpArtifacts bool,
	continueAfter *config.ContinueAfter,
	ec *commands.ExecutionContext,
	execCmd string,
	execFileCmd string) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewBuildCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted
	cmdReport.TargetReference = targetRef

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

	fmt.Printf("cmd=%s state=started\n", cmdName)

	if buildFromDockerfile == "" {
		fmt.Printf("cmd=%s info=params target=%v continue.mode=%v rt.as.user=%v keep.perms=%v\n",
			cmdName, targetRef, continueAfter.Mode, doRunTargetAsUser, doKeepPerms)
	} else {
		fmt.Printf("cmd=%s info=params context=%v/file=%v continue.mode=%v rt.as.user=%v keep.perms=%v\n",
			cmdName, targetRef, buildFromDockerfile, continueAfter.Mode, doRunTargetAsUser, doKeepPerms)
	}

	if buildFromDockerfile != "" {
		fmt.Printf("cmd=%s state=building message='building basic image'\n", cmdName)
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
				fmt.Printf("cmd=%s info=param.error status=malformed.custom.image.tag value=%s\n", cmdName, customImageTag)
				fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
				commands.Exit(commands.ECTBuild | ecbBadCustomImageTag)
			}
		} else {
			fatImageRepoNameTag = fmt.Sprintf("docker-slim-tmp-fat-image.%v.%v",
				os.Getpid(), time.Now().UTC().Format("20060102150405"))
		}

		fmt.Printf("cmd=%s info=basic.image.info tag=%s dockerfile=%s context=%s\n",
			cmdName, fatImageRepoNameTag, buildFromDockerfile, targetRef)

		fatBuilder, err := builder.NewBasicImageBuilder(client,
			fatImageRepoNameTag,
			buildFromDockerfile,
			targetRef,
			doShowBuildLogs)
		errutil.FailOn(err)

		err = fatBuilder.Build()

		if doShowBuildLogs || err != nil {
			fmt.Printf("cmd=%s build logs (standard image) ====================\n", cmdName)
			fmt.Println(fatBuilder.BuildLog.String())
			fmt.Printf("cmd=%s end of build logs (standard image) =============\n", cmdName)
		}

		if err != nil {
			fmt.Printf("cmd=%s info=build.error status=standard.image.build.error value='%v'\n", cmdName, err)
			fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
			commands.Exit(commands.ECTBuild | ecbImageBuildError)
		}

		fmt.Printf("cmd=%s state=basic.image.build.completed\n", cmdName)

		targetRef = fatImageRepoNameTag
		//todo: remove the temporary fat image (should have a flag for it in case users want the fat image too)
	}

	logger.Infof("image=%v http-probe=%v remove-file-artifacts=%v image-overrides=%+v entrypoint=%+v (%v) cmd=%+v (%v) workdir='%v' env=%+v expose=%+v",
		targetRef, doHTTPProbe, doRmFileArtifacts,
		imageOverrideSelectors,
		overrides.Entrypoint, overrides.ClearEntrypoint, overrides.Cmd, overrides.ClearCmd,
		overrides.Workdir, overrides.Env, overrides.ExposedPorts)

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if !commands.ConfirmNetwork(logger, client, overrides.Network) {
		fmt.Printf("cmd=%s info=param.error status=unknown.network value=%s\n", cmdName, overrides.Network)
		fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
		commands.Exit(commands.ECTCommon | commands.ECBadNetworkName)
	}

	imageInspector, err := image.NewInspector(client, targetRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Printf("cmd=%s info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", cmdName, targetRef)
		fmt.Printf("cmd=%s state=exited\n", cmdName)
		commands.Exit(commands.ECTBuild | ecbImageBuildError)
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

	if imageInspector.DockerfileInfo != nil {
		if imageInspector.DockerfileInfo.ExeUser != "" {
			fmt.Printf("cmd=%s info=image.users exe='%v' all='%v'\n",
				cmdName,
				imageInspector.DockerfileInfo.ExeUser,
				strings.Join(imageInspector.DockerfileInfo.AllUsers, ","))
		}

		if len(imageInspector.DockerfileInfo.ImageStack) > 0 {
			cmdReport.ImageStack = imageInspector.DockerfileInfo.ImageStack

			for idx, layerInfo := range imageInspector.DockerfileInfo.ImageStack {
				fmt.Printf("cmd=%s info=image.stack index=%v name='%v' id='%v'\n",
					cmdName, idx, layerInfo.FullName, layerInfo.ID)
			}
		}

		if len(imageInspector.DockerfileInfo.ExposedPorts) > 0 {
			fmt.Printf("cmd=%s info=image.exposed_ports list='%v'\n", cmdName,
				strings.Join(imageInspector.DockerfileInfo.ExposedPorts, ","))
		}
	}

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
		commands.Exit(commands.ECTBuild | ecbNoEntrypoint)
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

	var probe *http.CustomProbe
	if doHTTPProbe {
		var err error
		probe, err = http.NewCustomProbe(
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
			true,
			prefix)
		errutil.FailOn(err)

		if len(probe.Ports) == 0 {
			fmt.Printf("cmd=%s state=http.probe.error error='no exposed ports' message='expose your service port with --expose or disable HTTP probing with --http-probe=false if your containerized application doesnt expose any network services\n", cmdName)
			logger.Info("shutting down 'fat' container...")
			containerInspector.FinishMonitoring()
			_ = containerInspector.ShutdownContainer()

			fmt.Printf("cmd=%s state=exited\n", cmdName)
			commands.Exit(commands.ECTBuild | ecbImageBuildError)
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

	execFail := false

	switch continueAfter.Mode {
	case "enter":
		fmt.Printf("cmd=%s info=prompt message='USER INPUT REQUIRED, PRESS <ENTER> WHEN YOU ARE DONE USING THE CONTAINER'\n", cmdName)
		creader := bufio.NewReader(os.Stdin)
		_, _, _ = creader.ReadLine()
	case "exec":
		var input *bytes.Buffer
		var cmd []string
		if len(execFileCmd) != 0 {
			input = bytes.NewBufferString(execFileCmd)
			cmd = []string{"sh", "-s"}
			for _, line := range strings.Split(string(execFileCmd), "\n") {
				fmt.Printf("cmd=%s mode=exec shell='%s'\n", cmdName, line)
			}
		} else {
			input = bytes.NewBufferString("")
			cmd = []string{"sh", "-c", execCmd}
			fmt.Printf("cmd=%s mode=exec shell='%s'\n", cmdName, execCmd)
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
		fmt.Printf("cmd=%s mode=exec exitcode=%d\n", cmdName, inspect.ExitCode)
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
		if probe != nil && probe.CallCount > 0 && probe.OkCount == 0 {
			//make sure we show the container logs because none of the http probe calls were successful
			containerInspector.DoShowContainerLogs = true
		}
	default:
		errutil.Fail("unknown continue-after mode")
	}

	fmt.Printf("cmd=%s state=container.inspection.finishing\n", cmdName)

	containerInspector.FinishMonitoring()

	logger.Info("shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	errutil.WarnOn(err)

	if execFail {
		fmt.Printf("cmd=%s mode=exec message='fatal: exec cmd failure'\n", cmdName)
		os.Exit(1)
	}

	fmt.Printf("cmd=%s state=container.inspection.artifact.processing\n", cmdName)

	if !containerInspector.HasCollectedData() {
		imageInspector.ShowFatImageDockerInstructions()
		fmt.Printf("cmd=%s info=results status='no data collected (no minified image generated). (version=%v location='%s')'\n",
			cmdName,
			v.Current(), fsutil.ExeDir())
		fmt.Printf("cmd=%s state=exited\n", cmdName)
		return
	}

	logger.Info("processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	errutil.FailOn(err)

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	fmt.Printf("cmd=%s state=container.inspection.done\n", cmdName)
	fmt.Printf("cmd=%s state=building message='building optimized image'\n", cmdName)

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

	if doShowBuildLogs || err != nil {
		fmt.Printf("cmd=%s build logs (optimized image) ====================\n", cmdName)
		fmt.Println(builder.BuildLog.String())
		fmt.Printf("cmd=%s end of build logs (optimized image) =============\n", cmdName)
	}

	if err != nil {
		fmt.Printf("cmd=%s info=build.error status=optimized.image.build.error value='%v'\n", cmdName, err)
		fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
		commands.Exit(commands.ECTBuild | ecbImageBuildError)
	}

	fmt.Printf("cmd=%s state=completed\n", cmdName)
	cmdReport.State = command.StateCompleted

	/////////////////////////////
	newImageInspector, err := image.NewInspector(client, builder.RepoName)
	errutil.FailOn(err)

	if newImageInspector.NoImage() {
		fmt.Printf("cmd=%s info=results message='minified image not found - %s'\n", cmdName, builder.RepoName)
		fmt.Printf("cmd=%s state=exited\n", cmdName)
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

		fmt.Printf("cmd=%s info=results status='MINIFIED BY %.2fX [%v (%v) => %v (%v)]'\n",
			cmdName,
			cmdReport.MinifiedBy,
			cmdReport.SourceImage.Size,
			cmdReport.SourceImage.SizeHuman,
			cmdReport.MinifiedImageSize,
			cmdReport.MinifiedImageSizeHuman)
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

	fmt.Printf("cmd=%s info=results  image.name=%v image.size='%v' data=%v\n",
		cmdName,
		cmdReport.MinifiedImage,
		cmdReport.MinifiedImageSizeHuman,
		cmdReport.MinifiedImageHasData)

	fmt.Printf("cmd=%s info=results  artifacts.location='%v'\n", cmdName, cmdReport.ArtifactLocation)
	fmt.Printf("cmd=%s info=results  artifacts.report=%v\n", cmdName, cmdReport.ContainerReportName)
	fmt.Printf("cmd=%s info=results  artifacts.dockerfile.original=Dockerfile.fat\n", cmdName)
	fmt.Printf("cmd=%s info=results  artifacts.dockerfile.new=Dockerfile\n", cmdName)
	fmt.Printf("cmd=%s info=results  artifacts.seccomp=%v\n", cmdName, cmdReport.SeccompProfileName)
	fmt.Printf("cmd=%s info=results  artifacts.apparmor=%v\n", cmdName, cmdReport.AppArmorProfileName)

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
	fmt.Printf("cmd=%s info=commands message='use the xray command to learn more about the optimize image'\n", cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		fmt.Printf("cmd=%s info=report file='%s'\n", cmdName, cmdReport.ReportLocation())
	}

}
