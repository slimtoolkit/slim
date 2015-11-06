package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func onBuildCommand(imageRef string, doHttpProbe bool, doRmFileArtifacts bool) {
	fmt.Printf("docker-slim: [build] image=%v http-probe=%v remove-file-artifacts=%v\n",
		imageRef, doHttpProbe, doRmFileArtifacts)

	client, _ := docker.NewClientFromEnv()

	log.Info("docker-slim: inspecting 'fat' image metadata...")
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
		log.Fatal("docker-slim: could not find target image in the image list")
	}

	appArmorProfileName := "apparmor-profile"
	slimImageRepo := "slim"
	if len(imageRecord.RepoTags) > 0 {
		if rtInfo := strings.Split(imageRecord.RepoTags[0], ":"); len(rtInfo) > 1 {
			slimImageRepo = fmt.Sprintf("%s.slim", rtInfo[0])
			if nameParts := strings.Split(rtInfo[0], "/"); len(nameParts) > 1 {
				appArmorProfileName = strings.Join(nameParts, "-")
			} else {
				appArmorProfileName = rtInfo[0]
			}
			appArmorProfileName = fmt.Sprintf("%s-apparmor-profile", appArmorProfileName)
		}
	}

	log.Infof("docker-slim: 'fat' image size => %v (%v)\n",
		imageInfo.VirtualSize, humanize.Bytes(uint64(imageInfo.VirtualSize)))

	fatImageDockerInstructions, err := genDockerfileFromHistory(client, imageRef)
	failOnError(err)

	imageMeta := struct {
		RepoName     string
		ID           string
		Entrypoint   []string
		Cmd          []string
		WorkingDir   string
		Env          []string
		ExposedPorts map[docker.Port]struct{}
		Volumes      map[string]struct{}
		OnBuild      []string
		User         string
	}{
		slimImageRepo,
		imageInfo.ID,
		imageInfo.Config.Entrypoint,
		imageInfo.Config.Cmd,
		imageInfo.Config.WorkingDir,
		imageInfo.Config.Env,
		imageInfo.Config.ExposedPorts,
		imageInfo.Config.Volumes,
		imageInfo.Config.OnBuild,
		imageInfo.Config.User,
	}

	var fatContainerCmd []string
	if len(imageInfo.Config.Entrypoint) > 0 {
		fatContainerCmd = append(fatContainerCmd, imageInfo.Config.Entrypoint...)
	}

	if len(imageInfo.Config.Cmd) > 0 {
		fatContainerCmd = append(fatContainerCmd, imageInfo.Config.Cmd...)
	}

	localVolumePath := filepath.Join(myFileDir(), "container")

	artifactLocation := filepath.Join(localVolumePath, "artifacts")
	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
		artifactDir, err = os.Stat(artifactLocation)
		failOnError(err)
	}

	/*
		NOTE: not using IPC for now...
		ipcLocation := filepath.Join(localVolumePath,"ipc")
		_, err = os.Stat(ipcLocation)
		if os.IsNotExist(err) {
			os.MkdirAll(ipcLocation, 0777)
			_, err = os.Stat(ipcLocation)
			failOnError(err)
		}
	*/

	failWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	log.Info("docker-slim: saving 'fat' image info...")

	fatImageDockerfileLocation := filepath.Join(artifactLocation, "Dockerfile.fat")
	err = saveDockerfileData(fatImageDockerfileLocation, fatImageDockerInstructions)
	failOnError(err)

	mountInfo := fmt.Sprintf("%s:/opt/dockerslim", localVolumePath)

	var cmdPort docker.Port = "65501/tcp"
	var evtPort docker.Port = "65502/tcp"

	containerOptions := docker.CreateContainerOptions{
		Name: "dockerslimk",
		Config: &docker.Config{
			Image: imageRef,
			// NOTE: specifying Mounts here doesn't work :)
			//Mounts: []docker.Mount{{
			//        Source: localVolumePath,
			//        Destination: "/opt/dockerslim",
			//        Mode: "",
			//        RW: true,
			//    },
			//},
			ExposedPorts: map[docker.Port]struct{}{
				cmdPort: struct{}{},
				evtPort: struct{}{},
			},
			Entrypoint: []string{"/opt/dockerslim/bin/alauncher"},
			Cmd:        fatContainerCmd,
			Labels:     map[string]string{"type": "dockerslim"},
		},
		HostConfig: &docker.HostConfig{
			Binds:           []string{mountInfo},
			PublishAllPorts: true,
			CapAdd:          []string{"SYS_ADMIN"},
			Privileged:      true,
		},
	}

	log.Info("docker-slim: creating instrumented 'fat' container...")
	containerInfo, err := client.CreateContainer(containerOptions)
	failOnError(err)
	log.Infoln("docker-slim: created container =>", containerInfo.ID)

	log.Info("docker-slim: starting 'fat' container...")

	err = client.StartContainer(containerInfo.ID, &docker.HostConfig{
		PublishAllPorts: true,
		CapAdd:          []string{"SYS_ADMIN"},
		Privileged:      true,
	})
	failOnError(err)

	inspContainerInfo, err := client.InspectContainer(containerInfo.ID)
	failWhen(inspContainerInfo.NetworkSettings == nil, "docker-slim: error => no network info")
	log.Debugf("container NetworkSettings.Ports => %#v\n", inspContainerInfo.NetworkSettings.Ports)

	cmdPortBindings := inspContainerInfo.NetworkSettings.Ports[cmdPort]
	evtPortBindings := inspContainerInfo.NetworkSettings.Ports[evtPort]
	dockerHostIP := getDockerHostIP()
	cmdChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIP, cmdPortBindings[0].HostPort)
	evtChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIP, evtPortBindings[0].HostPort)
	log.Debugf("cmdChannelAddr=%v evtChannelAddr=%v\n", cmdChannelAddr, evtChannelAddr)

	var httpProbePorts []string
	for nsPortKey, nsPortData := range inspContainerInfo.NetworkSettings.Ports {
		if (nsPortKey == cmdPort) || (nsPortKey == evtPort) {
			continue
		}

		httpProbePorts = append(httpProbePorts, nsPortData[0].HostPort)
	}

	//TODO: keep checking the monitor state until no new files (and processes) are discovered
	log.Info("docker-slim: watching container monitor...")

	//evtChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-launcher.events.ipc", localVolumePath)
	//cmdChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-launcher.cmds.ipc", localVolumePath)

	evtChannel, err = newEvtChannel(evtChannelAddr)
	failOnError(err)
	cmdChannel, err = newCmdClient(cmdChannelAddr)
	failOnError(err)

	//endTime := time.After(time.Second * 200)
	//work := 0
	//doneWatching:
	//for {
	//	select {
	//	case <-endTime:
	//		log.Info("docker-slim: done with work!")
	//		break doneWatching
	//	case <-time.After(time.Second * 3):
	//		work++
	//		log.Infoln("docker-slim: still watching =>", work)
	//	}
	//}

	if doHttpProbe {
		startHTTPProbe(dockerHostIP, httpProbePorts)
	}

	fmt.Println("docker-slim: press any key when you are done using the container...")
	creader := bufio.NewReader(os.Stdin)
	_, _, _ = creader.ReadLine() //or _,_ = creaderReadString('\n')
	cmdResponse, err := sendCmd(cmdChannel, "monitor.finish")
	_ = cmdResponse
	log.Debugf("'monitor.finish' response => '%v'\n", cmdResponse)
	log.Info("docker-slim: waiting for the container finish its work...")
	//for now there's only one event ("done")
	evt, err := getEvt(evtChannel)
	_ = evt
	log.Debugf("docker-slim: alauncher event => '%v'\n", evt)

	shutdownEvtChannel()
	shutdownCmdChannel()

	log.Info("docker-slim: stopping 'fat' container...")
	err = client.StopContainer(containerInfo.ID, 9)
	warnOnError(err)

	log.Info("docker-slim: removing 'fat' container...")
	removeOption := docker.RemoveContainerOptions{
		ID:            containerInfo.ID,
		RemoveVolumes: true,
		Force:         true,
	}
	err = client.RemoveContainer(removeOption)
	warnOnError(err)

	log.Info("docker-slim: generating AppArmor profile...")
	err = genAppArmorProfile(artifactLocation, appArmorProfileName)
	failOnError(err)

	log.Info("docker-slim: creating 'slim' image...")
	dockerfileLocation := filepath.Join(artifactLocation, "Dockerfile")

	var dfData bytes.Buffer
	dfData.WriteString("FROM scratch\n")
	dfData.WriteString("COPY files /\n")

	dfData.WriteString("WORKDIR ")
	dfData.WriteString(imageMeta.WorkingDir)
	dfData.WriteByte('\n')

	if len(imageMeta.Env) > 0 {
		for _, envInfo := range imageMeta.Env {
			if envParts := strings.Split(envInfo, "="); len(envParts) > 1 {
				dfData.WriteString("ENV ")
				envLine := fmt.Sprintf("%s %s", envParts[0], envParts[1])
				dfData.WriteString(envLine)
				dfData.WriteByte('\n')
			}
		}
	}

	if len(imageMeta.ExposedPorts) > 0 {
		for portInfo := range imageMeta.ExposedPorts {
			dfData.WriteString("EXPOSE ")
			dfData.WriteString(string(portInfo))
			dfData.WriteByte('\n')
		}
	}

	if len(imageMeta.Entrypoint) > 0 {
		var quotedEntryPoint []string
		for idx := range imageMeta.Entrypoint {
			quotedEntryPoint = append(quotedEntryPoint, strconv.Quote(imageMeta.Entrypoint[idx]))
		}
		/*
			"Entrypoint": [
			            "/bin/sh",
			            "-c",
			            "node /opt/my/service/server.js"
			        ],
		*/
		dfData.WriteString("ENTRYPOINT [")
		dfData.WriteString(strings.Join(quotedEntryPoint, ","))
		dfData.WriteByte(']')
		dfData.WriteByte('\n')
	}

	if len(imageMeta.Cmd) > 0 {
		var quotedCmd []string
		for idx := range imageMeta.Entrypoint {
			quotedCmd = append(quotedCmd, strconv.Quote(imageMeta.Cmd[idx]))
		}
		dfData.WriteString("CMD [")
		dfData.WriteString(strings.Join(quotedCmd, ","))
		dfData.WriteByte(']')
		dfData.WriteByte('\n')
	}

	err = ioutil.WriteFile(dockerfileLocation, dfData.Bytes(), 0644)
	failOnError(err)

	buildOptions := docker.BuildImageOptions{
		Name:           imageMeta.RepoName,
		RmTmpContainer: true,
		ContextDir:     artifactLocation,
		Dockerfile:     "Dockerfile",
		OutputStream:   os.Stdout,
	}

	err = client.BuildImage(buildOptions)
	failOnError(err)
	log.Infoln("docker-slim: created new image:", imageMeta.RepoName)

	if doRmFileArtifacts {
		log.Info("docker-slim: removing temporary artifacts...")
		err = os.RemoveAll(artifactLocation) //TODO: remove only the "files" subdirectory
		warnOnError(err)
	}

	fmt.Println("docker-slim: [build] done.")
}
