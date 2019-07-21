package dockerfile

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudimmunity/go-dockerclientx"
	"github.com/dustin/go-humanize"

	v "github.com/docker-slim/docker-slim/pkg/version"
)

// Info represents the reverse engineered Dockerfile info
type Info struct {
	Lines        []string
	AllUsers     []string
	ExeUser      string
	ExposedPorts []string
	ImageStack   []*ImageInfo
}

type ImageInfo struct {
	IsTopImage   bool               `json:"is_top_image"`
	ID           string             `json:"id"`
	FullName     string             `json:"full_name"`
	RepoName     string             `json:"repo_name"`
	VersionTag   string             `json:"version_tag"`
	RawTags      []string           `json:"raw_tags,omitempty"`
	CreateTime   string             `json:"create_time"`
	NewSize      int64              `json:"new_size"`
	NewSizeHuman string             `json:"new_size_human"`
	BaseImageID  string             `json:"base_image_id,omitempty"`
	Instructions []*InstructionInfo `json:"instructions"`
}

type InstructionInfo struct {
	Type                string `json:"type"`
	Time                string `json:"time"`
	IsNop               bool   `json:"is_nop"`
	IsLocal             bool   `json:"is_local"`
	IntermediateImageID string `json:"intermediate_image_id,omitempty"`
	Size                int64  `json:"size"`
	SizeHuman           string `json:"size_human,omitempty"`
	CommandSnippet      string `json:"command_snippet"`
	command             string
	SystemCommands      []string `json:"system_commands,omitempty"`
	Comment             string   `json:"comment,omitempty"`
	instPosition        string
	imageFullName       string
	RawTags             []string `json:"raw_tags,omitempty"`
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

	var fatImageDockerInstructions []InstructionInfo
	var currentImageInfo *ImageInfo
	var prevImageID string

	imageLayerCount := len(imageHistory)
	imageLayerStart := imageLayerCount - 1
	startNewImage := true
	if imageLayerCount > 0 {
		for idx := imageLayerStart; idx >= 0; idx-- {
			isNop := false

			nopPrefix := "/bin/sh -c #(nop) "
			execPrefix := "/bin/sh -c "
			rawLine := imageHistory[idx].CreatedBy
			var inst string

			if strings.Contains(rawLine, "(nop)") {
				isNop = true
			}

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

			instInfo := InstructionInfo{
				IsNop:   isNop,
				command: cleanInst,
				Time:    time.Unix(imageHistory[idx].Created, 0).UTC().Format(time.RFC3339),
				Comment: imageHistory[idx].Comment,
				RawTags: imageHistory[idx].Tags,
				Size:    imageHistory[idx].Size,
			}

			instParts := strings.SplitN(cleanInst, " ", 2)
			if len(instParts) == 2 {
				instInfo.Type = instParts[0]
			}

			if instInfo.Type == "RUN" {
				var cmdParts []string
				cmds := strings.Replace(instParts[1], "\\", "", -1)
				if strings.Contains(cmds, "&&") {
					cmdParts = strings.Split(cmds, "&&")
				} else {
					cmdParts = strings.Split(cmds, ";")
				}

				for _, cmd := range cmdParts {
					cmd = strings.TrimSpace(cmd)
					cmd = strings.Replace(cmd, "\t", "", -1)
					cmd = strings.Replace(cmd, "\n", "", -1)
					instInfo.SystemCommands = append(instInfo.SystemCommands, cmd)
				}
			}

			if instInfo.Type == "WORKDIR" {
				instInfo.SystemCommands = append(instInfo.SystemCommands, fmt.Sprintf("mkdir -p %s", instParts[1]))
			}

			if len(instInfo.command) > 44 {
				instInfo.CommandSnippet = fmt.Sprintf("%s...", instInfo.command[0:44])
			} else {
				instInfo.CommandSnippet = instInfo.command
			}

			if instInfo.Size > 0 {
				instInfo.SizeHuman = humanize.Bytes(uint64(instInfo.Size))
			}

			if imageHistory[idx].ID != "<missing>" {
				instInfo.IsLocal = true
				instInfo.IntermediateImageID = imageHistory[idx].ID
			}

			if startNewImage {
				startNewImage = false
				currentImageInfo = &ImageInfo{
					BaseImageID: prevImageID,
					NewSize:     0,
				}
			}

			currentImageInfo.NewSize += imageHistory[idx].Size
			currentImageInfo.Instructions = append(currentImageInfo.Instructions, &instInfo)

			instPosition := "intermediate"
			if idx == imageLayerStart {
				instPosition = "first" //first instruction in the list
			}

			if len(imageHistory[idx].Tags) > 0 {
				instPosition = "last" //last in an image

				currentImageInfo.ID = imageHistory[idx].ID
				prevImageID = currentImageInfo.ID

				currentImageInfo.CreateTime = instInfo.Time
				currentImageInfo.RawTags = imageHistory[idx].Tags

				instInfo.imageFullName = imageHistory[idx].Tags[0]
				currentImageInfo.FullName = imageHistory[idx].Tags[0]

				if tagInfo := strings.Split(imageHistory[idx].Tags[0], ":"); len(tagInfo) > 1 {
					currentImageInfo.RepoName = tagInfo[0]
					currentImageInfo.VersionTag = tagInfo[1]
				}

				currentImageInfo.NewSizeHuman = humanize.Bytes(uint64(currentImageInfo.NewSize))

				out.ImageStack = append(out.ImageStack, currentImageInfo)
				startNewImage = true
			}

			instInfo.instPosition = instPosition

			fatImageDockerInstructions = append(fatImageDockerInstructions, instInfo)
		}

		if currentImageInfo != nil {
			currentImageInfo.IsTopImage = true
		}
	}

	for idx, instInfo := range fatImageDockerInstructions {
		if instInfo.instPosition == "first" {
			out.Lines = append(out.Lines, "# new image")
		}

		out.Lines = append(out.Lines, instInfo.command)
		if instInfo.instPosition == "last" {
			commentText := fmt.Sprintf("# end of image: %s (id: %s tags: %s)",
				instInfo.imageFullName, instInfo.IntermediateImageID, strings.Join(instInfo.RawTags, ","))

			out.Lines = append(out.Lines, commentText)
			out.Lines = append(out.Lines, "")
			if idx < (len(fatImageDockerInstructions) - 1) {
				out.Lines = append(out.Lines, "# new image")
			}
		}

		if instInfo.Comment != "" {
			out.Lines = append(out.Lines, "# "+instInfo.Comment)
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
	volumes map[string]struct{},
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

	dsInfoLabel := fmt.Sprintf("LABEL docker-slim.version=\"%s\"\n", v.Current())
	dfData.WriteString(dsInfoLabel)

	if len(volumes) > 0 {
		var volumeList []string
		for volumeName := range volumes {
			volumeList = append(volumeList, strconv.Quote(volumeName))
		}

		volumeInst := fmt.Sprintf("VOLUME [%s]", strings.Join(volumeList, ","))
		dfData.WriteString(volumeInst)
		dfData.WriteByte('\n')
	}

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
