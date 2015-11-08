package main

import (
	"bufio"
	//"bytes"
	"fmt"
	//"io/ioutil"
	"os"
	//"path/filepath"
	//"strconv"
	//"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func onBuildCommand(imageRef string, doHttpProbe bool, doRmFileArtifacts bool) {
	fmt.Printf("docker-slim: [build] image=%v http-probe=%v remove-file-artifacts=%v\n",
		imageRef, doHttpProbe, doRmFileArtifacts)

	localVolumePath, artifactLocation := myAppDirs()

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

	containerInspector, err := NewContainerInspector(client, imageInspector, localVolumePath)
	failOnError(err)

	log.Info("docker-slim: starting instrumented 'fat' container...")
	err = containerInspector.RunContainer()
	failOnError(err)

	var httpProbePorts []string
	for nsPortKey, nsPortData := range containerInspector.ContainerInfo.NetworkSettings.Ports {
		if (nsPortKey == containerInspector.CmdPort) || (nsPortKey == containerInspector.EvtPort) {
			continue
		}

		httpProbePorts = append(httpProbePorts, nsPortData[0].HostPort)
	}

	log.Info("docker-slim: watching container monitor...")

	if doHttpProbe {
		startHTTPProbe(containerInspector.DockerHostIP, httpProbePorts)
	}

	fmt.Println("docker-slim: press any key when you are done using the container...")
	creader := bufio.NewReader(os.Stdin)
	_, _, _ = creader.ReadLine()

	containerInspector.FinishMonitoring()

	log.Info("docker-slim: shutting down 'fat' container...")
	err = containerInspector.ShutdownContainer()
	warnOnError(err)

	log.Info("docker-slim: generating AppArmor profile...")
	err = genAppArmorProfile(artifactLocation, imageInspector.AppArmorProfileName)
	failOnError(err)

	log.Info("docker-slim: building 'slim' image...")
	builder, err := NewImageBuilder(client, imageInspector.SlimImageRepo, imageInspector.ImageInfo, artifactLocation)
	failOnError(err)
	err = builder.Build()
	failOnError(err)

	log.Infoln("docker-slim: created new image:", builder.RepoName)

	if doRmFileArtifacts {
		log.Info("docker-slim: removing temporary artifacts...")
		err = os.RemoveAll(artifactLocation) //TODO: remove only the "files" subdirectory
		warnOnError(err)
	}

	fmt.Println("docker-slim: [build] done.")
}
