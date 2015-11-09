package main

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func onInfoCommand(imageRef string) {
	fmt.Println("docker-slim: [info] image=", imageRef)

	_, artifactLocation := myAppDirs()

	client, _ := docker.NewClientFromEnv()

	imageInspector, err := NewImageInspector(client, imageRef, artifactLocation)
	failOnError(err)

	log.Info("docker-slim: inspecting 'fat' image metadata...")
	err = imageInspector.Inspect()
	failOnError(err)

	log.Infof("docker-slim: 'fat' image size => %v (%v)\n",
		imageInspector.ImageInfo.VirtualSize,
		humanize.Bytes(uint64(imageInspector.ImageInfo.VirtualSize)))

	log.Info("docker-slim: processing 'fat' image info...")
	err = imageInspector.ProcessCollectedData()
	failOnError(err)

	fmt.Println("docker-slim: [info] done.")
}
