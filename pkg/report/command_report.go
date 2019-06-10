package report

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/pkg/util/errutil"
)

// Command state constants
const (
	CmdStateUnknown   = "unknown"
	CmdStateError     = "error"
	CmdStateStarted   = "started"
	CmdStateCompleted = "completed"
	CmdStateExited    = "exited"
	CmdStateDone      = "done"
)

// Command type constants
const (
	CmdTypeBuild   CmdType = "build"
	CmdTypeProfile CmdType = "profile"
	CmdTypeInfo    CmdType = "info"
)

// CmdType is the command name data type
type CmdType string

// Command is the common command report data
type Command struct {
	reportLocation string
	Type           CmdType `json:"type"`
	State          string  `json:"state"`
	Error          string  `json:"error,omitempty"`
}

// BuildCommand is the 'build' command report data
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

// ProfileCommand is the 'profile' command report data
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

// InfoCommand is the 'info' command report data
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

// NewBuildCommand creates a new 'build' command report
func NewBuildCommand(reportLocation string) *BuildCommand {
	return &BuildCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           CmdTypeBuild,
			State:          CmdStateUnknown,
		},
	}
}

// NewProfileCommand creates a new 'profile' command report
func NewProfileCommand(reportLocation string) *ProfileCommand {
	return &ProfileCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           CmdTypeProfile,
			State:          CmdStateUnknown,
		},
	}
}

// NewInfoCommand creates a new 'info' command report
func NewInfoCommand(reportLocation string) *InfoCommand {
	return &InfoCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           CmdTypeInfo,
			State:          CmdStateUnknown,
		},
	}
}

// Save saves the report data to the configured location
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
				errutil.FailOn(err)
			}
		}

		reportData, err := json.MarshalIndent(p, "", "  ")
		errutil.FailOn(err)

		err = ioutil.WriteFile(p.reportLocation, reportData, 0644)
		errutil.FailOn(err)
	}
}
