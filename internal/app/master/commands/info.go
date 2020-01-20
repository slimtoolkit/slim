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

// OnInfo implements the 'info' docker-slim command
func OnInfo(
	doCheckVersion bool,
	cmdReportLocation string,
	doDebug bool,
	statePath string,
	archiveState string,
	inContainer bool,
	isDSImage bool,
	clientConfig *config.DockerClient,
	imageRef string) {
	logger := log.WithFields(log.Fields{"app": "docker-slim", "command": "info"})

	viChan := version.CheckAsync(doCheckVersion, inContainer, isDSImage)

	cmdReport := report.NewInfoCommand(cmdReportLocation)
	cmdReport.State = report.CmdStateStarted
	cmdReport.OriginalImage = imageRef

	fmt.Println("docker-slim[info]: state=started")
	fmt.Printf("docker-slim[info]: info=params target=%v\n", imageRef)

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("docker-slim[info]: info=docker.connect.error message='%s'\n", exitMsg)
		fmt.Printf("docker-slim[info]: state=exited version=%s\n", v.Current())
		os.Exit(-777)
	}
	errutil.FailOn(err)

	if doDebug {
		version.Print("docker-slim[info]:", logger, client, false, inContainer, isDSImage)
	}

	imageInspector, err := image.NewInspector(client, imageRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		fmt.Printf("docker-slim[info]: info=target.image.error status=not.found image='%v' message='make sure the target image already exists locally'\n", imageRef)
		fmt.Println("docker-slim[info]: state=exited")
		return
	}

	logger.Info("inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	errutil.FailOn(err)

	localVolumePath, artifactLocation, statePath, stateKey := fsutil.PrepareImageStateDirs(statePath, imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation
	logger.Debugf("localVolumePath=%v, artifactLocation=%v, statePath=%v, stateKey=%v", localVolumePath, artifactLocation, statePath, stateKey)

	fmt.Printf("docker-slim[info]: info=image id=%v size.bytes=%v size.human=%v\n",
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	logger.Info("processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	errutil.FailOn(err)

	fmt.Println("docker-slim[info]: state=completed")
	cmdReport.State = report.CmdStateCompleted

	fmt.Println("docker-slim[info]: state=done")

	vinfo := <-viChan
	version.PrintCheckVersion("docker-slim[info]:", vinfo)

	cmdReport.State = report.CmdStateDone
	if cmdReport.Save() {
		fmt.Printf("docker-slim[info]: info=report file='%s'\n", cmdReport.ReportLocation())
	}
}
