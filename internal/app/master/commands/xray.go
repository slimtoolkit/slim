package commands

import (
	"fmt"
	"os"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

// Info command exit codes
const (
	eciOther = iota + 1
)

// OnXray implements the 'xray' docker-slim command
func OnXray(
	doCheckVersion bool,
	cmdReportLocation string,
	doDebug bool,
	statePath string,
	archiveState string,
	inContainer bool,
	isDSImage bool,
	clientConfig *config.DockerClient,
	imageRef string,
	ec *ExecutionContext) {
	const cmdName = "xray"
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(doCheckVersion, inContainer, isDSImage)

	cmdReport := report.NewXrayCommand(cmdReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.OriginalImage = imageRef

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=params target=%v\n", appName, cmdName, imageRef)

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

	if doDebug {
		version.Print(prefix, logger, client, false, inContainer, isDSImage)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Printf("%s[%s]: info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", appName, cmdName, imageRef)
		fmt.Printf("%s[%s]: state=exited\n", appName, cmdName)
		return
	}

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

	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = report.CmdStateCompleted

	fmt.Printf("%s[%s]: state=done\n", appName, cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = report.CmdStateDone
	if cmdReport.Save() {
		fmt.Printf("%s[%s]: info=report file='%s'\n", appName, cmdName, cmdReport.ReportLocation())
	}
}
