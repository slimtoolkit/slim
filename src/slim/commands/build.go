package commands

import (
	"bufio"
	"fmt"
	"os"

	"internal/utils"
	"slim/builder"
	"slim/inspectors/container"
	"slim/inspectors/container/probes/http"
	"slim/inspectors/image"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func OnBuild(imageRef string, doHttpProbe bool, doRmFileArtifacts bool) {
	fmt.Printf("docker-slim: [build] image=%v http-probe=%v remove-file-artifacts=%v\n",
		imageRef, doHttpProbe, doRmFileArtifacts)

	localVolumePath, artifactLocation := utils.PrepareSlimDirs()

	client, _ := docker.NewClientFromEnv()

	imageInspector, err := image.NewInspector(client, imageRef, artifactLocation)
	utils.FailOn(err)

	log.Info("docker-slim: inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	utils.FailOn(err)

	log.Infof("docker-slim: 'fat' image size => %v (%v)\n",
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	log.Info("docker-slim: processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	utils.FailOn(err)

	containerInspector, err := container.NewInspector(client, imageInspector, localVolumePath)
	utils.FailOn(err)

	log.Info("docker-slim: starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	utils.FailOn(err)

	log.Info("docker-slim: watching container monitor...")

	if doHttpProbe {
		probe, err := http.NewRootProbe(containerInspector)
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

	log.Info("docker-slim: building 'slim' image...")
	builder, err := builder.NewImageBuilder(client, imageInspector.SlimImageRepo, imageInspector.ImageInfo, artifactLocation)
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
