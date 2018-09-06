package report

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/pkg/utils/errutils"
)

const (
	CmdStateUnknown   = "unknown"
	CmdStateError     = "error"
	CmdStateStarted   = "started"
	CmdStateCompleted = "completed"
	CmdStateExited    = "exited"
	CmdStateDone      = "done"
)

const (
	CmdTypeBuild   CmdType = "build"
	CmdTypeProfile CmdType = "profile"
	CmdTypeInfo    CmdType = "info"
)

type CmdType string

type Command struct {
	reportLocation string
	Type           CmdType `json:"type"`
	State          string  `json:"state"`
	Error          string  `json:"error,omitempty"`
}

type BuildCommand struct {
	Command
	OriginalImage          string  `json:"original_image"`
	OriginalImageSize      int64   `json:"original_image_size"`
	OriginalImageSizeHuman string  `json:"original_image_size_human"`
	MinifiedImageSize      int64   `json:"minified_image_size"`
	MinifiedImageSizeHuman string  `json:"minified_image_size_human"`
	MinifiedImage          string  `json:"minified_image"`
	MinifiedImageHasData   bool    `json:"minified_image_has_data"`
	MinifiedBy             float64 `json:"minified_by"`
	ArtifactLocation       string  `json:"artifact_location"`
	ContainerReportName    string  `json:"container_report_name"`
	SeccompProfileName     string  `json:"seccomp_profile_name"`
	AppArmorProfileName    string  `json:"apparmor_profile_name"`
}

type ProfileCommand struct {
	Command
	OriginalImage          string  `json:"original_image"`
	OriginalImageSize      int64   `json:"original_image_size"`
	OriginalImageSizeHuman string  `json:"original_image_size_human"`
	MinifiedImageSize      int64   `json:"minified_image_size"`
	MinifiedImageSizeHuman string  `json:"minified_image_size_human"`
	MinifiedImage          string  `json:"minified_image"`
	MinifiedImageHasData   bool    `json:"minified_image_has_data"`
	MinifiedBy             float64 `json:"minified_by"`
	ArtifactLocation       string  `json:"artifact_location"`
	ContainerReportName    string  `json:"container_report_name"`
	SeccompProfileName     string  `json:"seccomp_profile_name"`
	AppArmorProfileName    string  `json:"apparmor_profile_name"`
}

type InfoCommand struct {
	Command
	OriginalImage          string  `json:"original_image"`
	OriginalImageSize      int64   `json:"original_image_size"`
	OriginalImageSizeHuman string  `json:"original_image_size_human"`
	MinifiedImageSize      int64   `json:"minified_image_size"`
	MinifiedImageSizeHuman string  `json:"minified_image_size_human"`
	MinifiedImage          string  `json:"minified_image"`
	MinifiedImageHasData   bool    `json:"minified_image_has_data"`
	MinifiedBy             float64 `json:"minified_by"`
	ArtifactLocation       string  `json:"artifact_location"`
	ContainerReportName    string  `json:"container_report_name"`
	SeccompProfileName     string  `json:"seccomp_profile_name"`
	AppArmorProfileName    string  `json:"apparmor_profile_name"`
}

func NewBuildCommand(reportLocation string) *BuildCommand {
	return &BuildCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           CmdTypeBuild,
			State:          CmdStateUnknown,
		},
	}
}

func NewProfileCommand(reportLocation string) *ProfileCommand {
	return &ProfileCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           CmdTypeProfile,
			State:          CmdStateUnknown,
		},
	}
}

func NewInfoCommand(reportLocation string) *InfoCommand {
	return &InfoCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           CmdTypeInfo,
			State:          CmdStateUnknown,
		},
	}
}

func (p *Command) Save() {
	if p.reportLocation != "" {
		dirName := filepath.Dir(p.reportLocation)
		baseName := filepath.Base(p.reportLocation)

		if baseName == "." {
			fmt.Printf("no build command report location: %v\n", p.reportLocation)
			return
		}

		if dirName != "." {
			_, err := os.Stat(dirName)
			if os.IsNotExist(err) {
				os.MkdirAll(dirName, 0777)
				_, err = os.Stat(dirName)
				errutils.FailOn(err)
			}
		}

		reportData, err := json.MarshalIndent(p, "", "  ")
		errutils.FailOn(err)

		err = ioutil.WriteFile(p.reportLocation, reportData, 0644)
		errutils.FailOn(err)
	}
}
