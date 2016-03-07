package commands

import (
	"bufio"
	"fmt"
	"os"

	"github.com/cloudimmunity/docker-slim/master/builder"
	"github.com/cloudimmunity/docker-slim/master/config"
	"github.com/cloudimmunity/docker-slim/master/docker/dockerclient"
	"github.com/cloudimmunity/docker-slim/master/inspectors/container"
	"github.com/cloudimmunity/docker-slim/master/inspectors/container/probes/http"
	"github.com/cloudimmunity/docker-slim/master/inspectors/image"
	"github.com/cloudimmunity/docker-slim/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/dustin/go-humanize"
)

func OnBuild(doDebug bool,
	clientConfig *config.DockerClient,
	imageRef string,
	customImageTag string,
	doHttpProbe bool,
	httpProbeCmds []config.HttpProbeCmd,
	doRmFileArtifacts bool,
	doShowContainerLogs bool,
	imageOverrides map[string]bool,
	overrides *config.ContainerOverrides,
	volumeMounts map[string]config.VolumeMount,
	excludePaths map[string]bool,
	includePaths map[string]bool) {
	fmt.Printf("docker-slim: [build] image=%v http-probe=%v remove-file-artifacts=%v image-overrides=%+v entrypoint=%+v (%v) cmd=%+v (%v) workdir='%v' env=%+v expose=%+v\n",
		imageRef, doHttpProbe, doRmFileArtifacts,
		imageOverrides,
		overrides.Entrypoint, overrides.ClearEntrypoint, overrides.Cmd, overrides.ClearCmd,
		overrides.Workdir, overrides.Env, overrides.ExposedPorts)

	client := dockerclient.New(clientConfig)

	imageInspector, err := image.NewInspector(client, imageRef)
	utils.FailOn(err)

	log.Info("docker-slim: inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	utils.FailOn(err)

	localVolumePath, artifactLocation := utils.PrepareSlimDirs(imageInspector.ImageInfo.ID)
	imageInspector.ArtifactLocation = artifactLocation

	log.Infof("docker-slim: [%v] 'fat' image size => %v (%v)\n",
		imageInspector.ImageInfo.ID,
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	log.Info("docker-slim: processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	utils.FailOn(err)

	containerInspector, err := container.NewInspector(client,
		imageInspector,
		localVolumePath,
		overrides,
		doShowContainerLogs,
		volumeMounts,
		excludePaths,
		includePaths,
		doDebug)
	utils.FailOn(err)

	log.Info("docker-slim: starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	utils.FailOn(err)

	log.Info("docker-slim: watching container monitor...")

	if doHttpProbe {
		probe, err := http.NewCustomProbe(containerInspector, httpProbeCmds)
		utils.FailOn(err)
		probe.Start()
	}

	fmt.Println("docker-slim: press any key when you are done using the container...")
	creader := bufio.NewReader(os.Stdin)
	_, _, _ = creader.ReadLine()

	containerInspector.FinishMonitoring()

	log.Info("docker-slim: shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	utils.WarnOn(err)

	log.Info("docker-slim: processing instrumented 'fat' container info...")
	err = containerInspector.ProcessCollectedData()
	utils.FailOn(err)

	if customImageTag == "" {
		customImageTag = imageInspector.SlimImageRepo
	}

	log.Info("docker-slim: building 'slim' image...")
	builder, err := builder.NewImageBuilder(client,
		customImageTag,
		imageInspector.ImageInfo,
		artifactLocation,
		imageOverrides,
		overrides)
	utils.FailOn(err)
	err = builder.Build()
	utils.FailOn(err)

	log.Infoln("docker-slim: created new image:", builder.RepoName)

	if doRmFileArtifacts {
		log.Info("docker-slim: removing temporary artifacts...")
		err = utils.RemoveArtifacts(artifactLocation) //TODO: remove only the "files" subdirectory
		utils.WarnOn(err)
	}

	fmt.Println("docker-slim: [build] done.")
}
