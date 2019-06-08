package dockerfile

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
)

type Layer struct {
	Name string
	Tags []string
}

// Info represents the reverse engineered Dockerfile info
type Info struct {
	Lines        []string
	AllUsers     []string
	ExeUser      string
	ExposedPorts []string
	Layers       []Layer
}

type imageInst struct {
	instCmd      string
	instComment  string
	instType     string
	instTime     int64
	layerImageID string
	imageName    string
	shortTags    []string
	fullTags     []string
}

// ReverseDockerfileFromHistory recreates Dockerfile information from container image history
func ReverseDockerfileFromHistory(apiClient *docker.Client, imageID string) (*Info, error) {
	//NOTE: comment field is missing (TODO: enhance the lib...)
	imageHistory, err := apiClient.ImageHistory(imageID)
	if err != nil {
		return nil, err
	}

	var out Info

	log.Debugf("\n\nIMAGE HISTORY =>\n%#v\n\n", imageHistory)

	var fatImageDockerInstructions []imageInst

	imageLayerCount := len(imageHistory)
	imageLayerStart := imageLayerCount - 1
	if imageLayerCount > 0 {
		for idx := imageLayerStart; idx >= 0; idx-- {
			nopPrefix := "/bin/sh -c #(nop) "
			execPrefix := "/bin/sh -c "
			rawLine := imageHistory[idx].CreatedBy
			var inst string

			switch {
			case len(rawLine) == 0:
				inst = "FROM scratch"
			case strings.HasPrefix(rawLine, nopPrefix):
				inst = strings.TrimPrefix(rawLine, nopPrefix)
			case strings.HasPrefix(rawLine, execPrefix):
				runData := strings.TrimPrefix(rawLine, execPrefix)
				if strings.Contains(runData, "&&") {
					parts := strings.Split(runData, "&&")
					for i := range parts {
						partPrefix := ""
						if i != 0 {
							partPrefix = "\t"
						}
						parts[i] = partPrefix + strings.TrimSpace(parts[i])
					}
					runDataFormatted := strings.Join(parts, " && \\\n")
					inst = "RUN " + runDataFormatted
				} else {
					inst = "RUN " + runData
				}
			default:
				inst = rawLine
			}
			//NOTE: Dockerfile instructions can be any case, but the instructions from history are always uppercase
			cleanInst := strings.TrimSpace(inst)

			if strings.HasPrefix(cleanInst, "ENTRYPOINT ") {
				inst = strings.Replace(inst, "&{[", "[", -1)
				inst = strings.Replace(inst, "]}", "]", -1)
				//TODO: make whitespace separated array comma separated
			}

			if strings.HasPrefix(cleanInst, "USER ") {
				parts := strings.SplitN(cleanInst, " ", 2)
				if len(parts) == 2 {
					userName := strings.TrimSpace(parts[1])

					out.AllUsers = append(out.AllUsers, userName)
					out.ExeUser = userName
				} else {
					log.Infof("ReverseDockerfileFromHistory - unexpected number of user parts - %v", len(parts))
				}
			}

			if strings.HasPrefix(cleanInst, "EXPOSE ") {
				parts := strings.SplitN(cleanInst, " ", 2)
				if len(parts) == 2 {
					portInfo := strings.TrimSpace(parts[1])

					out.ExposedPorts = append(out.ExposedPorts, portInfo)
				} else {
					log.Infof("ReverseDockerfileFromHistory - unexpected number of expose parts - %v", len(parts))
				}
			}

			instInfo := imageInst{
				instCmd:      inst,
				instTime:     imageHistory[idx].Created,
				layerImageID: imageHistory[idx].ID,
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

				out.Layers = append(out.Layers, Layer{Name: instInfo.imageName, Tags: instInfo.shortTags})
			}

			instInfo.instType = instType

			fatImageDockerInstructions = append(fatImageDockerInstructions, instInfo)
		}
	}

	for idx, instInfo := range fatImageDockerInstructions {
		if instInfo.instType == "first" {
			out.Lines = append(out.Lines, "# new image")
		}

		out.Lines = append(out.Lines, instInfo.instCmd)
		if instInfo.instType == "last" {
			commentText := fmt.Sprintf("# end of image: %s (id: %s tags: %s)",
				instInfo.imageName, instInfo.layerImageID, strings.Join(instInfo.shortTags, ","))
			out.Lines = append(out.Lines, commentText)
			out.Lines = append(out.Lines, "")
			if idx < (len(fatImageDockerInstructions) - 1) {
				out.Lines = append(out.Lines, "# new image")
			}
		}

		if instInfo.instComment != "" {
			out.Lines = append(out.Lines, "# "+instInfo.instComment)
		}

		//TODO: use time diff to separate each instruction
	}

	log.Debugf("IMAGE INSTRUCTIONS:")
	for _, iiLine := range out.Lines {
		log.Debug(iiLine)
	}

	return &out, nil

	//TODO: try adding comments in the docker file to see if the comments
	//show up in the 'history' command

	/*
	   NOTE:
	   Usually "MAINTAINER" is the first instruction,
	   so it can be used to detect a base image.
	   NOTE2:
	   "MAINTAINER" is now depricated
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

// SaveDockerfileData saves the Dockerfile information to a file
func SaveDockerfileData(fatImageDockerfileLocation string, fatImageDockerfileLines []string) error {
	var data bytes.Buffer
	data.WriteString(strings.Join(fatImageDockerfileLines, "\n"))
	return ioutil.WriteFile(fatImageDockerfileLocation, data.Bytes(), 0644)
}

// GenerateFromInfo builds and saves a Dockerfile file object
func GenerateFromInfo(location string,
	workingDir string,
	env []string,
	user string,
	exposedPorts map[docker.Port]struct{},
	entrypoint []string,
	cmd []string,
	hasData bool) error {

	dockerfileLocation := filepath.Join(location, "Dockerfile")

	var dfData bytes.Buffer
	dfData.WriteString("FROM scratch\n")

	if hasData {
		dfData.WriteString("COPY files /\n")
	}

	if workingDir != "" {
		dfData.WriteString("WORKDIR ")
		dfData.WriteString(workingDir)
		dfData.WriteByte('\n')
	}

	if len(env) > 0 {
		for _, envInfo := range env {
			if envParts := strings.Split(envInfo, "="); len(envParts) > 1 {
				dfData.WriteString("ENV ")
				envLine := fmt.Sprintf("%s \"%s\"", envParts[0], envParts[1])
				dfData.WriteString(envLine)
				dfData.WriteByte('\n')
			}
		}
	}

	if user != "" {
		dfData.WriteString("USER ")
		dfData.WriteString(user)
		dfData.WriteByte('\n')
	}

	if len(exposedPorts) > 0 {
		for portInfo := range exposedPorts {
			dfData.WriteString("EXPOSE ")
			dfData.WriteString(string(portInfo))
			dfData.WriteByte('\n')
		}
	}

	if len(entrypoint) > 0 {
		//TODO: need to make sure the generated ENTRYPOINT is compatible with the original behavior
		var quotedEntryPoint []string
		for idx := range entrypoint {
			quotedEntryPoint = append(quotedEntryPoint, strconv.Quote(entrypoint[idx]))
		}

		dfData.WriteString("ENTRYPOINT [")
		dfData.WriteString(strings.Join(quotedEntryPoint, ","))
		dfData.WriteByte(']')
		dfData.WriteByte('\n')
	}

	if len(cmd) > 0 {
		//TODO: need to make sure the generated CMD is compatible with the original behavior
		var quotedCmd []string
		for idx := range cmd {
			quotedCmd = append(quotedCmd, strconv.Quote(cmd[idx]))
		}
		dfData.WriteString("CMD [")
		dfData.WriteString(strings.Join(quotedCmd, ","))
		dfData.WriteByte(']')
		dfData.WriteByte('\n')
	}

	return ioutil.WriteFile(dockerfileLocation, dfData.Bytes(), 0644)
}

//
// https://docs.docker.com/engine/reference/builder/
//
