package main

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func onInfoCommand(imageRef string) {
	fmt.Println("docker-slim: [info] image=", imageRef)

	client, _ := docker.NewClientFromEnv()

	fmt.Println("docker-slim: inspecting 'fat' image metadata...")

	imageInfo, err := client.InspectImage(imageRef)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			log.Fatalf("docker-slim: could not find target image")
		}
		log.Fatalf("docker-slim: InspectImage(%v) error => %v", imageRef, err)
	}

	var imageRecord docker.APIImages
	imageList, err := client.ListImages(docker.ListImagesOptions{All: true})
	failOnError(err)
	for _, r := range imageList {
		if r.ID == imageInfo.ID {
			imageRecord = r
			break
		}
	}

	if imageRecord.ID == "" {
		log.Fatalf("docker-slim: could not find target image in the image list")
	}

	log.Infof("docker-slim: 'fat' image size => %v (%v)\n",
		imageInfo.VirtualSize, humanize.Bytes(uint64(imageInfo.VirtualSize)))

	fatImageDockerInstructions, err := genDockerfileFromHistory(client, imageRef)
	failOnError(err)

	localVolumePath := filepath.Join(myFileDir(), "container")

	artifactLocation := filepath.Join(localVolumePath, "artifacts")
	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
		artifactDir, err = os.Stat(artifactLocation)
		failOnError(err)
	}

	failWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	log.Info("docker-slim: saving 'fat' image info...")

	fatImageDockerfileLocation := filepath.Join(artifactLocation, "Dockerfile.fat")
	err = saveDockerfileData(fatImageDockerfileLocation, fatImageDockerInstructions)
	failOnError(err)

	fmt.Println("docker-slim: [info] done.")
}
