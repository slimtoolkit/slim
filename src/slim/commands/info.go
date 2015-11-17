package commands

import (
	"fmt"

	"internal/utils"
	"slim/inspectors/image"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func OnInfo(imageRef string) {
	fmt.Println("docker-slim: [info] image=", imageRef)

	_, artifactLocation := utils.PrepareSlimDirs()

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

	fmt.Println("docker-slim: [info] done.")
}
