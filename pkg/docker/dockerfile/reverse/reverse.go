package reverse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
)

var (
	ErrBadInstPrefix = errors.New("bad instruction prefix")
)

// Dockerfile represents the reverse engineered Dockerfile info
type Dockerfile struct {
	Lines           []string
	Maintainers     []string
	AllUsers        []string
	ExeUser         string
	ExposedPorts    []string
	ImageStack      []*ImageInfo
	AllInstructions []*InstructionInfo
	HasOnbuild      bool
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
	Type string `json:"type"`
	Time string `json:"time"`
	//Time              time.Time `json:"time"`
	IsLastInstruction   bool     `json:"is_last_instruction,omitempty"`
	IsNop               bool     `json:"is_nop"`
	IsExecForm          bool     `json:"is_exec_form,omitempty"` //is exec/json format (a valid field for RUN, ENTRYPOINT, CMD)
	LocalImageExists    bool     `json:"local_image_exists"`
	IntermediateImageID string   `json:"intermediate_image_id,omitempty"`
	LayerIndex          int      `json:"layer_index"` //-1 for an empty layer
	LayerID             string   `json:"layer_id,omitempty"`
	LayerFSDiffID       string   `json:"layer_fsdiff_id,omitempty"`
	Size                int64    `json:"size"`
	SizeHuman           string   `json:"size_human,omitempty"`
	Params              string   `json:"params,omitempty"`
	CommandSnippet      string   `json:"command_snippet"`
	CommandAll          string   `json:"command_all"`
	SystemCommands      []string `json:"system_commands,omitempty"`
	Comment             string   `json:"comment,omitempty"`
	Author              string   `json:"author,omitempty"`
	EmptyLayer          bool     `json:"empty_layer,omitempty"`
	instPosition        string
	imageFullName       string
	RawTags             []string `json:"raw_tags,omitempty"`
	Target              string   `json:"target,omitempty"`      //for ADD and COPY
	SourceType          string   `json:"source_type,omitempty"` //for ADD and COPY
}

//The 'History' API doesn't expose the 'author' in the records it returns
//The 'author' field is useful in detecting if it's a Dockerfile instruction
//or if it's created with something else.
//One option is to combine the 'History' API data with the history data
//from the image config JSON embedded in the image.
//Another option is to rely on '#(nop)'.

const (
	defaultRunInstShell = "/bin/sh"
	notRunInstPrefix    = "/bin/sh -c #(nop) "
	runInstShellPrefix  = "/bin/sh -c " //without any ARG params
	runInstArgsPrefix   = "|"
)

const (
	//MAINTAINER:
	instPrefixMaintainer = "MAINTAINER "
	//ENTRYPOINT:
	instTypeEntrypoint   = "ENTRYPOINT"
	instPrefixEntrypoint = "ENTRYPOINT "
	//CMD:
	instTypeCmd   = "CMD"
	instPrefixCmd = "CMD "
	//USER:
	instTypeUser   = "USER"
	instPrefixUser = "USER "
	//EXPOSE:
	instTypeExpose   = "EXPOSE"
	instPrefixExpose = "EXPOSE "
	//WORKDIR:
	instTypeWorkdir   = "WORKDIR"
	instPrefixWorkdir = "WORKDIR "
	//HEALTHCHECK:
	instTypeHealthcheck   = "HEALTHCHECK"
	instPrefixHealthcheck = "HEALTHCHECK "
	//ONBUILD:
	instTypeOnbuild = "ONBUILD"
	//RUN:
	instTypeRun   = "RUN"
	instPrefixRun = "RUN "
	//ADD:
	instTypeAdd = "ADD"
	//COPY:
	instTypeCopy = "COPY"
)

// DockerfileFromHistory recreates Dockerfile information from container image history
func DockerfileFromHistory(apiClient *docker.Client, imageID string) (*Dockerfile, error) {
	imageHistory, err := apiClient.ImageHistory(imageID)
	if err != nil {
		return nil, err
	}

	var out Dockerfile

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

			rawLine := imageHistory[idx].CreatedBy
			var inst string

			if strings.Contains(rawLine, "#(nop)") {
				isNop = true
			}

			isExecForm := false

			switch {
			case len(rawLine) == 0:
				inst = ""
			case strings.HasPrefix(rawLine, notRunInstPrefix):
				//Instructions that are not RUN
				inst = strings.TrimPrefix(rawLine, notRunInstPrefix)
			case strings.HasPrefix(rawLine, runInstShellPrefix):
				//RUN instruction in shell form
				runData := strings.TrimPrefix(rawLine, runInstShellPrefix)
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
					inst = instPrefixRun + runDataFormatted
				} else {
					inst = instPrefixRun + runData
				}
			default:
				//TODO: need to refactor
				processed := false
				rawInst := rawLine
				if strings.HasPrefix(rawInst, runInstArgsPrefix) {
					parts := strings.SplitN(rawInst, " ", 2)
					if len(parts) == 2 {
						withArgs := strings.TrimSpace(parts[1])
						argNumStr := parts[0][1:]
						argNum, err := strconv.Atoi(argNumStr)
						if err == nil {
							if withArgsArray, err := shlex.Split(withArgs); err == nil {
								if len(withArgsArray) > argNum {
									rawInstParts := withArgsArray[argNum:]
									processed = true
									if len(rawInstParts) > 2 &&
										rawInstParts[0] == defaultRunInstShell &&
										rawInstParts[1] == "-c" {
										isExecForm = false
										rawInstParts = rawInstParts[2:]

										inst = fmt.Sprintf("RUN %s", strings.Join(rawInstParts, " "))
										inst = strings.TrimSpace(inst)
									} else {
										isExecForm = true

										var outJson bytes.Buffer
										encoder := json.NewEncoder(&outJson)
										encoder.SetEscapeHTML(false)
										err := encoder.Encode(rawInstParts)
										if err == nil {
											inst = fmt.Sprintf("RUN %s", outJson.String())
										}
									}
								} else {
									log.Infof("ReverseDockerfileFromHistory - RUN with ARGs - malformed - %v (%v)", rawInst, err)
								}
							} else {
								log.Infof("ReverseDockerfileFromHistory - RUN with ARGs - malformed - %v (%v)", rawInst, err)
							}

						} else {
							log.Infof("ReverseDockerfileFromHistory - RUN with ARGs - malformed number of ARGs - %v (%v)", rawInst, err)
						}
					} else {
						log.Infof("ReverseDockerfileFromHistory - RUN with ARGs - unexpected number of parts - %v", rawInst)
					}
				}

				if !processed {
					//default to RUN instruction in exec form
					isExecForm = true
					inst = instPrefixRun + rawInst
					if outArray, err := shlex.Split(rawInst); err == nil {
						var outJson bytes.Buffer
						encoder := json.NewEncoder(&outJson)
						encoder.SetEscapeHTML(false)
						err := encoder.Encode(outArray)
						if err == nil {
							inst = fmt.Sprintf("RUN %s", outJson.String())
						}
					}
				}
			}

			//NOTE: Dockerfile instructions can be any case, but the instructions from history are always uppercase
			cleanInst := strings.TrimSpace(inst)

			if strings.HasPrefix(cleanInst, instPrefixEntrypoint) {
				cleanInst = strings.Replace(cleanInst, "&{[", "[", -1)
				cleanInst = strings.Replace(cleanInst, "]}", "]", -1)

				entrypointShellFormPrefix := `ENTRYPOINT ["/bin/sh" "-c" "`
				if strings.HasPrefix(cleanInst, entrypointShellFormPrefix) {
					instData := strings.TrimPrefix(cleanInst, entrypointShellFormPrefix)
					instData = strings.TrimSuffix(instData, `"]`)
					cleanInst = instPrefixEntrypoint + instData
				} else {
					isExecForm = true

					instData := strings.TrimPrefix(cleanInst, instPrefixEntrypoint)
					instData = fixJSONArray(instData)
					cleanInst = instPrefixEntrypoint + instData
				}
			}

			if strings.HasPrefix(cleanInst, instPrefixCmd) {
				cmdShellFormPrefix := `CMD ["/bin/sh" "-c" "`
				if strings.HasPrefix(cleanInst, cmdShellFormPrefix) {
					instData := strings.TrimPrefix(cleanInst, cmdShellFormPrefix)
					instData = strings.TrimSuffix(instData, `"]`)
					cleanInst = instPrefixCmd + instData
				} else {
					isExecForm = true

					instData := strings.TrimPrefix(cleanInst, instPrefixCmd)
					instData = fixJSONArray(instData)
					cleanInst = instPrefixCmd + instData
				}
			}

			if strings.HasPrefix(cleanInst, instPrefixMaintainer) {
				parts := strings.SplitN(cleanInst, " ", 2)
				if len(parts) == 2 {
					maintainer := strings.TrimSpace(parts[1])

					out.Maintainers = append(out.Maintainers, maintainer)
				} else {
					log.Infof("ReverseDockerfileFromHistory - MAINTAINER - unexpected number of user parts - %v", len(parts))
				}
			}

			if strings.HasPrefix(cleanInst, instPrefixUser) {
				parts := strings.SplitN(cleanInst, " ", 2)
				if len(parts) == 2 {
					userName := strings.TrimSpace(parts[1])

					out.AllUsers = append(out.AllUsers, userName)
					out.ExeUser = userName
				} else {
					log.Infof("ReverseDockerfileFromHistory - unexpected number of user parts - %v", len(parts))
				}
			}

			if strings.HasPrefix(cleanInst, instPrefixExpose) {
				parts := strings.SplitN(cleanInst, " ", 2)
				if len(parts) == 2 {
					portInfo := strings.TrimSpace(parts[1])

					out.ExposedPorts = append(out.ExposedPorts, portInfo)
				} else {
					log.Infof("ReverseDockerfileFromHistory - unexpected number of expose parts - %v", len(parts))
				}
			}

			instInfo := InstructionInfo{
				IsNop:      isNop,
				IsExecForm: isExecForm,
				CommandAll: cleanInst,
				Time:       time.Unix(imageHistory[idx].Created, 0).UTC().Format(time.RFC3339),
				//Time:    time.Unix(imageHistory[idx].Created, 0),
				Comment: imageHistory[idx].Comment,
				RawTags: imageHistory[idx].Tags,
				Size:    imageHistory[idx].Size,
			}

			instParts := strings.SplitN(cleanInst, " ", 2)
			if len(instParts) == 2 {
				instInfo.Type = instParts[0]
			}

			if instInfo.Type == instTypeOnbuild {
				out.HasOnbuild = true
			}

			if instInfo.CommandAll == "" {
				instInfo.Type = "NONE"
				instInfo.CommandAll = "#no instruction info"
			}

			if instInfo.Type == instTypeRun {
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
			} else {
				if len(instParts) == 2 {
					instInfo.Params = instParts[1]
				}
			}

			if instInfo.Type == instTypeWorkdir {
				instInfo.SystemCommands = append(instInfo.SystemCommands, fmt.Sprintf("mkdir -p %s", instParts[1]))
			}

			switch instInfo.Type {
			case instTypeAdd, instTypeCopy:
				pparts := strings.SplitN(instInfo.Params, ":", 2)
				if len(pparts) == 2 {
					instInfo.SourceType = pparts[0]
					tparts := strings.SplitN(pparts[1], " in ", 2)
					if len(tparts) == 2 {
						instInfo.Target = tparts[1]

						instInfo.CommandAll = fmt.Sprintf("%s %s:%s %s",
							instInfo.Type,
							instInfo.SourceType,
							tparts[0],
							instInfo.Target)
					}
				}
			}

			if instInfo.Type == instTypeHealthcheck {

				healthInst, _, err := deserialiseHealtheckInstruction(instInfo.Params)
				if err != nil {
					log.Errorf("ReverseDockerfileFromHistory - HEALTHCHECK - deserialiseHealtheckInstruction - %v", err)
				}

				instInfo.CommandAll = healthInst
			}

			if len(instInfo.CommandAll) > 44 {
				instInfo.CommandSnippet = fmt.Sprintf("%s...", instInfo.CommandAll[0:44])
			} else {
				instInfo.CommandSnippet = instInfo.CommandAll
			}

			if instInfo.Size > 0 {
				instInfo.SizeHuman = humanize.Bytes(uint64(instInfo.Size))
			}

			if imageHistory[idx].ID != "<missing>" {
				instInfo.LocalImageExists = true
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

			out.AllInstructions = append(out.AllInstructions, &instInfo)

			instPosition := "intermediate"
			if idx == imageLayerStart {
				instPosition = "first" //first instruction in the list
			}

			if idx == 0 || (len(imageHistory[idx].Tags) > 0) {
				instPosition = "last" //last in an image

				currentImageInfo.ID = imageHistory[idx].ID
				prevImageID = currentImageInfo.ID

				if instInfo.IntermediateImageID == currentImageInfo.ID {
					instInfo.IntermediateImageID = ""
					instInfo.IsLastInstruction = true
				}

				currentImageInfo.CreateTime = instInfo.Time
				currentImageInfo.RawTags = imageHistory[idx].Tags

				if len(imageHistory[idx].Tags) > 0 {
					instInfo.imageFullName = imageHistory[idx].Tags[0]
					currentImageInfo.FullName = imageHistory[idx].Tags[0]

					if tagInfo := strings.Split(imageHistory[idx].Tags[0], ":"); len(tagInfo) > 1 {
						currentImageInfo.RepoName = tagInfo[0]
						currentImageInfo.VersionTag = tagInfo[1]
					}
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

	//Always adding "FROM scratch" as the first line
	//GOAL: to have a reversed Dockerfile that can be used to build a new image
	out.Lines = append(out.Lines, "FROM scratch")
	for idx, instInfo := range fatImageDockerInstructions {
		if instInfo.instPosition == "first" {
			out.Lines = append(out.Lines, "# new image")
		}

		out.Lines = append(out.Lines, instInfo.CommandAll)
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

func fixJSONArray(in string) string {
	data := in
	if data[0] == '[' {
		data = data[1 : len(data)-1]
	}
	outArray, err := shlex.Split(data)
	if err != nil {
		return in
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(outArray)
	if err != nil {
		return in
	}

	return out.String()
}

func deserialiseHealtheckInstruction(data string) (string, *docker.HealthConfig, error) {
	//Example:
	// HEALTHCHECK &{["CMD" "/healthcheck" "8080"] "5s" "10s" "0s" '\x03'}
	// HEALTHCHECK --interval=5s --timeout=10s --retries=3 CMD [ "/healthcheck", "8080" ]
	//Note: CMD can be specified with both formats (shell and json)
	cleanInst := strings.TrimSpace(data)
	if !strings.HasPrefix(cleanInst, instPrefixHealthcheck) {
		return "", nil, ErrBadInstPrefix
	}

	cleanInst = strings.Replace(cleanInst, "&{[", "", -1)

	//Splits the string into two parts - first part pointer to array of string and rest of the string with } in end.
	instParts := strings.SplitN(cleanInst, "]", 2)
	// Cleans HEALTHCHECK part and splits the first part further
	parts := strings.SplitN(instParts[0], " ", 2)
	// joins the first part of the string
	instPart1 := strings.Join(parts[1:], " ")
	// removes quotes from the first part of the string
	instPart1 = strings.ReplaceAll(instPart1, "\"", "")

	// cleans it to assign it to the pointer config.Test
	config := docker.HealthConfig{
		Test: strings.Split(instPart1, " "),
	}

	// removes the } from the second part of the string
	instPart2 := strings.Replace(instParts[1], "}", "", -1)
	// removes extra spaces from string
	instPart2 = strings.TrimSpace(instPart2)

	paramParts := strings.SplitN(instPart2, " ", 4)
	for i, param := range paramParts {
		paramParts[i] = strings.Trim(param, "\"'")
	}

	var err error
	config.Interval, err = time.ParseDuration(paramParts[0])
	if err != nil {
		fmt.Printf("[%s] config.Interval err = %v\n", paramParts[0], err)
	}

	config.Timeout, err = time.ParseDuration(paramParts[1])
	if err != nil {
		fmt.Printf("[%s] config.Timeout err = %v\n", paramParts[1], err)
	}

	config.StartPeriod, err = time.ParseDuration(paramParts[2])
	if err != nil {
		fmt.Printf("[%s] config.StartPeriod err = %v\n", paramParts[2], err)
	}

	paramParts[3] = strings.TrimPrefix(paramParts[3], `\x`)
	retries, err := strconv.ParseInt(paramParts[3], 16, 64)
	if err != nil {
		fmt.Printf("[%s] config.Retries err = %v\n", paramParts[3], err)
	} else {
		config.Retries = int(retries)
	}

	var testType string
	if len(config.Test) > 0 {
		testType = config.Test[0]
	}

	var strTest string
	switch testType {
	case "NONE":
		strTest = "NONE"
	case "CMD":
		if len(config.Test) == 1 {
			strTest = "CMD []"
		} else {
			strTest = fmt.Sprintf(`CMD ["%s"]`, strings.Join(config.Test[1:], `", "`))
		}
	case "CMD-SHELL":
		strTest = fmt.Sprintf("CMD %s", strings.Join(config.Test[1:], " "))
	}

	defaultTimeout := false
	defaultInterval := false
	defaultRetries := false
	defaultStartPeriod := false

	if config.Timeout == 0 {
		defaultTimeout = true
		config.Timeout = 30 * time.Second
	}
	if config.Interval == 0 {
		defaultInterval = true
		config.Interval = 30 * time.Second
	}
	if config.Retries == 0 {
		defaultRetries = true
		config.Retries = 3
	}
	if config.StartPeriod == 0 {
		defaultStartPeriod = true
	}

	type HealthCheckFlag struct {
		flagFmtStr string
		isDefault  bool
		value      interface{}
	}

	healthInst := "HEALTHCHECK"
	for _, flag := range []HealthCheckFlag{
		{flagFmtStr: "--interval=%v", isDefault: defaultInterval, value: config.Interval},
		{flagFmtStr: "--timeout=%v", isDefault: defaultTimeout, value: config.Timeout},
		{flagFmtStr: "--start-period=%v", isDefault: defaultStartPeriod, value: config.StartPeriod},
		{flagFmtStr: "--retries=%x", isDefault: defaultRetries, value: config.Retries},
	} {
		if !flag.isDefault {
			healthInst = healthInst + " " + fmt.Sprintf(flag.flagFmtStr, flag.value)
		}
	}

	healthInst += " " + strTest
	if strTest == "NONE" {
		healthInst = "HEALTHCHECK NONE"
	}

	return healthInst, &config, nil
}

//
// https://docs.docker.com/engine/reference/builder/
//
