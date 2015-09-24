package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"
)

func failOnError(err error) {
	if err != nil {
		log.Fatalln("docker-slim: ERROR =>", err)
	}
}

func warnOnError(err error) {
	if err != nil {
		log.Println("docker-slim: ERROR =>", err)
	}
}

func failWhen(cond bool, msg string) {
	if cond {
		log.Fatalln("docker-slim: ERROR =>", msg)
	}
}

func myFileDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	failOnError(err)
	return dirName
}

type imageInst struct {
	instCmd      string
	instComment  string
	instType     string
	instTime     int64
	layerImageId string
	imageName    string
	shortTags    []string
	fullTags     []string
}

func genDockerfileFromHistory(apiClient *docker.Client, imageId string) ([]string, error) {
	//NOTE: comment field is missing (TODO: enhance the lib...)
	imageHistory, err := apiClient.ImageHistory(imageId)
	if err != nil {
		return nil, err
	}

	//log.Printf("\n\nIMAGE HISTORY =>\n%#v\n\n",imageHistory)

	var fatImageDockerInstructions []imageInst

	imageLayerCount := len(imageHistory)
	imageLayerStart := imageLayerCount - 1
	if imageLayerCount > 0 {
		for idx := imageLayerStart; idx >= 0; idx-- {
			nopPrefix := "/bin/sh -c #(nop) "
			execPrefix := "/bin/sh -c "
			rawLine := imageHistory[idx].CreatedBy
			var inst string

			if len(rawLine) == 0 {
				inst = "FROM scratch"
			} else if strings.HasPrefix(rawLine, nopPrefix) {
				inst = strings.TrimPrefix(rawLine, nopPrefix)
			} else if strings.HasPrefix(rawLine, execPrefix) {
				inst = "RUN " + strings.TrimPrefix(rawLine, execPrefix)
			} else {
				inst = rawLine
			}

			if strings.HasPrefix(inst, "ENTRYPOINT ") {
				inst = strings.Replace(inst, "&{[", "[", -1)
				inst = strings.Replace(inst, "]}", "]", -1)
			}

			instInfo := imageInst{
				instCmd:      inst,
				instTime:     imageHistory[idx].Created,
				layerImageId: imageHistory[idx].ID,
				instComment:  imageHistory[idx].Comment,
			}

			instType := "intermediate"
			if idx == imageLayerStart {
				instType = "first"
			}

			if len(imageHistory[idx].Tags) > 0 {
				instType = "last"

				if tagInfo := strings.Split(imageHistory[idx].Tags[0], ":"); len(tagInfo) > 1 {
					instInfo.imageName = tagInfo[0]
				}

				instInfo.fullTags = imageHistory[idx].Tags

				for _, fullTag := range instInfo.fullTags {
					if tagInfo := strings.Split(fullTag, ":"); len(tagInfo) > 1 {
						instInfo.shortTags = append(instInfo.shortTags, tagInfo[1])
					}
				}
			}

			instInfo.instType = instType

			fatImageDockerInstructions = append(fatImageDockerInstructions, instInfo)
		}
	}

	var fatImageDockerfileLines []string
	for idx, instInfo := range fatImageDockerInstructions {
		if instInfo.instType == "first" {
			fatImageDockerfileLines = append(fatImageDockerfileLines, "# new image")
		}

		fatImageDockerfileLines = append(fatImageDockerfileLines, instInfo.instCmd)
		if instInfo.instType == "last" {
			commentText := fmt.Sprintf("# end of image: %s (id: %s tags: %s)",
				instInfo.imageName, instInfo.layerImageId, strings.Join(instInfo.shortTags, ","))
			fatImageDockerfileLines = append(fatImageDockerfileLines, commentText)
			fatImageDockerfileLines = append(fatImageDockerfileLines, "")
			if idx < (len(fatImageDockerInstructions) - 1) {
				fatImageDockerfileLines = append(fatImageDockerfileLines, "# new image")
			}
		}

		if instInfo.instComment != "" {
			fatImageDockerfileLines = append(fatImageDockerfileLines, "# "+instInfo.instComment)
		}

		//TODO: use time diff to separate each instruction
	}

	log.Printf("IMAGE INSTRUCTIONS:")
	for _, iiLine := range fatImageDockerfileLines {
		log.Println(iiLine)
	}

	return fatImageDockerfileLines, nil

	//TODO: try adding comments in the docker file to see if the comments
	//show up in the 'history' command

	/*
	   NOTE:
	   Usually "MAINTAINER" is the first instruction,
	   so it can be used to detect a base image.
	*/

	/*
	   TODO:
	   need to have a set of signature for common base images
	   long path: need to discover base images dynamically
	   https://imagelayers.io/?images=alpine:3.1,ubuntu:14.04.1&lock=alpine:3.1

	   https://imagelayers.io/
	   https://github.com/CenturyLinkLabs/imagelayers
	   https://github.com/CenturyLinkLabs/imagelayers-graph
	*/
}

func saveDockerfileData(fatImageDockerfileLocation string, fatImageDockerInstructions []string) error {
	var data bytes.Buffer
	data.WriteString(strings.Join(fatImageDockerInstructions, "\n"))
	return ioutil.WriteFile(fatImageDockerfileLocation, data.Bytes(), 0644)
}

func main() {
	failWhen(len(os.Args) < 2, "docker-slim: error => missing image info")

	imageId := os.Args[1]

	features := map[string]bool{}
	if len(os.Args) > 2 {
		for _, fname := range os.Args[2:] {
			features[fname] = true
		}
	}

	client, _ := docker.NewClientFromEnv()

	log.Println("docker-slim: inspecting \"fat\" image metadata...")
	imageInfo, err := client.InspectImage(imageId)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			log.Fatalf("docker-slim: could not find target image")
		}
		log.Fatalf("docker-slim: InspectImage(%v) error => %v", imageId, err)
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

	slimImageRepo := "slim"
	if len(imageRecord.RepoTags) > 0 {
		if rtInfo := strings.Split(imageRecord.RepoTags[0], ":"); len(rtInfo) > 1 {
			slimImageRepo = fmt.Sprintf("%s.slim", rtInfo[0])
		}
	}

	log.Printf("docker-slim: \"fat\" image size => %v (%v)\n",
		imageInfo.VirtualSize, humanize.Bytes(uint64(imageInfo.VirtualSize)))

	fatImageDockerInstructions, err := genDockerfileFromHistory(client, imageId)
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

	localVolumePath := fmt.Sprintf("%s/container", myFileDir())
	artifactLocation := fmt.Sprintf("%v/artifacts", localVolumePath)

	artifactDir, err := os.Stat(artifactLocation)
	if os.IsNotExist(err) {
		os.MkdirAll(artifactLocation, 0777)
	}

	failWhen(!artifactDir.IsDir(), "artifact location is not a directory")

	log.Println("docker-slim: saving \"fat\" image info...")
	fatImageDockerfileLocation := fmt.Sprintf("%v/Dockerfile.fat", artifactLocation)
	err = saveDockerfileData(fatImageDockerfileLocation, fatImageDockerInstructions)
	failOnError(err)

	if !features["image-info-only"] {
		mountInfo := fmt.Sprintf("%s:/opt/dockerslim", localVolumePath)

		containerOptions := docker.CreateContainerOptions{
			Name: "dockerslimk",
			Config: &docker.Config{
				Image: imageId,
				// NOTE: specifying Mounts here doesn't work :)
				//Mounts: []docker.Mount{{
				//        Source: localVolumePath,
				//        Destination: "/opt/dockerslim",
				//        Mode: "",
				//        RW: true,
				//    },
				//},
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

		log.Println("docker-slim: creating instrumented \"fat\" container...")
		containerInfo, err := client.CreateContainer(containerOptions)
		failOnError(err)
		log.Println("docker-slim: created container =>", containerInfo.ID)

		log.Println("docker-slim: starting \"fat\" container...")

		err = client.StartContainer(containerInfo.ID, &docker.HostConfig{
			PublishAllPorts: true,
			CapAdd:          []string{"SYS_ADMIN"},
			Privileged:      true,
		})
		failOnError(err)

		//TODO: keep checking the monitor state until no new files (and processes) are discovered
		log.Println("docker-slim: watching container monitor...")
		endTime := time.After(time.Second * 130)
		work := 0
	doneWatching:
		for {
			select {
			case <-endTime:
				log.Println("docker-slim: done with work!")
				break doneWatching
			case <-time.After(time.Second * 3):
				work++
				log.Println("docker-slim: still watching =>", work)
			}
		}

		//log.Println("docker-slim: exporting \"fat\" container artifacts...")
		//time.Sleep(5 * time.Second)

		log.Println("docker-slim: stopping \"fat\" container...")
		err = client.StopContainer(containerInfo.ID, 9)
		warnOnError(err)

		log.Println("docker-slim: removing \"fat\" container...")
		removeOption := docker.RemoveContainerOptions{
			ID:            containerInfo.ID,
			RemoveVolumes: true,
			Force:         true,
		}
		err = client.RemoveContainer(removeOption)
		warnOnError(err)

		log.Println("docker-slim: creating \"slim\" image...")

		dockerfileLocation := fmt.Sprintf("%v/Dockerfile", artifactLocation)

		var dfData bytes.Buffer
		dfData.WriteString("FROM scratch\n")
		dfData.WriteString("COPY files /\n")

		dfData.WriteString("WORKDIR ")
		dfData.WriteString(imageMeta.WorkingDir)
		dfData.WriteByte('\n')

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
		log.Println("docker-slim: created new image:", imageMeta.RepoName)

		if features["rm-artifacts"] {
			log.Println("docker-slim: removing temporary artifacts (TODO)...")
			err = os.RemoveAll(artifactLocation)
			warnOnError(err)
		}
	}
}

// eval "$(docker-machine env default)"
// ./dockerslim 6f74095b68c9
// ./dockerslim 6f74095b68c9 rm-artifacts
// ./dockerslim 6f74095b68c9 image-info-only
