package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/reverse"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerimage"
	"github.com/docker-slim/docker-slim/pkg/docker/linter/check"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
)

// Command is the common command report data
type Command struct {
	reportLocation string
	Type           command.Type  `json:"type"`
	State          command.State `json:"state"`
	Error          string        `json:"error,omitempty"`
}

// ImageMetadata provides basic image metadata
type ImageMetadata struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Size          int64    `json:"size"`
	SizeHuman     string   `json:"size_human"`
	CreateTime    string   `json:"create_time"`
	AllNames      []string `json:"all_names"`
	Author        string   `json:"author,omitempty"`
	DockerVersion string   `json:"docker_version"`
	Architecture  string   `json:"architecture"`
	User          string   `json:"user,omitempty"`
	ExposedPorts  []string `json:"exposed_ports,omitempty"`
}

// SystemMetadata provides basic system metadata
type SystemMetadata struct {
	Type    string `json:"type"`
	Release string `json:"release"`
	OS      string `json:"os"`
}

// BuildCommand is the 'build' command report data
type BuildCommand struct {
	Command
	TargetReference        string               `json:"target_reference"`
	System                 SystemMetadata       `json:"system"`
	SourceImage            ImageMetadata        `json:"source_image"`
	MinifiedImageSize      int64                `json:"minified_image_size"`
	MinifiedImageSizeHuman string               `json:"minified_image_size_human"`
	MinifiedImage          string               `json:"minified_image"`
	MinifiedImageHasData   bool                 `json:"minified_image_has_data"`
	MinifiedBy             float64              `json:"minified_by"`
	ArtifactLocation       string               `json:"artifact_location"`
	ContainerReportName    string               `json:"container_report_name"`
	SeccompProfileName     string               `json:"seccomp_profile_name"`
	AppArmorProfileName    string               `json:"apparmor_profile_name"`
	ImageStack             []*reverse.ImageInfo `json:"image_stack"`
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

// XrayCommand is the 'xray' command report data
type XrayCommand struct {
	Command
	TargetReference      string                      `json:"target_reference"`
	SourceImage          ImageMetadata               `json:"source_image"`
	ArtifactLocation     string                      `json:"artifact_location"`
	ImageStack           []*reverse.ImageInfo        `json:"image_stack"`
	ImageLayers          []*dockerimage.LayerReport  `json:"image_layers"`
	ImageArchiveLocation string                      `json:"image_archive_location"`
	RawImageManifest     *dockerimage.ManifestObject `json:"raw_image_manifest,omitempty"`
	RawImageConfig       *dockerimage.ConfigObject   `json:"raw_image_config,omitempty"`
}

// LintCommand is the 'lint' command report data
type LintCommand struct {
	Command
	TargetType      string                   `json:"target_type"`
	TargetReference string                   `json:"target_reference"`
	BuildContextDir string                   `json:"build_context_dir,omitempty"`
	HitsCount       int                      `json:"hits_count"`
	NoHitsCount     int                      `json:"nohits_count"`
	ErrorsCount     int                      `json:"errors_count"`
	Hits            map[string]*check.Result `json:"hits,omitempty"`   //map[CHECK_ID]CHECK_RESULT
	Errors          map[string]error         `json:"errors,omitempty"` //map[CHECK_ID]ERROR_INFO
}

// ContainerizeCommand is the 'containerize' command report data
type ContainerizeCommand struct {
	Command
}

// ConvertCommand is the 'convert' command report data
type ConvertCommand struct {
	Command
}

// EditCommand is the 'edit' command report data
type EditCommand struct {
	Command
}

// NewBuildCommand creates a new 'build' command report
func NewBuildCommand(reportLocation string) *BuildCommand {
	return &BuildCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Build,
			State:          command.StateUnknown,
		},
	}
}

// NewProfileCommand creates a new 'profile' command report
func NewProfileCommand(reportLocation string) *ProfileCommand {
	return &ProfileCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Profile,
			State:          command.StateUnknown,
		},
	}
}

// NewXrayCommand creates a new 'xray' command report
func NewXrayCommand(reportLocation string) *XrayCommand {
	return &XrayCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Xray,
			State:          command.StateUnknown,
		},
	}
}

// NewLintCommand creates a new 'lint' command report
func NewLintCommand(reportLocation string) *LintCommand {
	return &LintCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Lint,
			State:          command.StateUnknown,
		},
	}
}

// NewContainerizeCommand creates a new 'containerize' command report
func NewContainerizeCommand(reportLocation string) *ContainerizeCommand {
	return &ContainerizeCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Containerize,
			State:          command.StateUnknown,
		},
	}
}

// NewConvertCommand creates a new 'convert' command report
func NewConvertCommand(reportLocation string) *ConvertCommand {
	return &ConvertCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Convert,
			State:          command.StateUnknown,
		},
	}
}

// NewEditCommand creates a new 'edit' command report
func NewEditCommand(reportLocation string) *EditCommand {
	return &EditCommand{
		Command: Command{
			reportLocation: reportLocation,
			Type:           command.Edit,
			State:          command.StateUnknown,
		},
	}
}

func (p *Command) ReportLocation() string {
	return p.reportLocation
}

func (p *Command) saveInfo(info interface{}) bool {
	if p.reportLocation != "" {
		dirName := filepath.Dir(p.reportLocation)
		baseName := filepath.Base(p.reportLocation)

		if baseName == "." {
			fmt.Printf("no build command report location: %v\n", p.reportLocation)
			return false
		}

		if dirName != "." {
			_, err := os.Stat(dirName)
			if os.IsNotExist(err) {
				os.MkdirAll(dirName, 0777)
				_, err = os.Stat(dirName)
				errutil.FailOn(err)
			}
		}

		var reportData bytes.Buffer
		encoder := json.NewEncoder(&reportData)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		err := encoder.Encode(info)
		errutil.FailOn(err)

		err = ioutil.WriteFile(p.reportLocation, reportData.Bytes(), 0644)
		errutil.FailOn(err)
		return true
	}

	return false
}

// Save saves the report data to the configured location
func (p *Command) Save() bool {
	return p.saveInfo(p)
}

// Save saves the Build command report data to the configured location
func (p *BuildCommand) Save() bool {
	return p.saveInfo(p)
}

// Save saves the Profile command report data to the configured location
func (p *ProfileCommand) Save() bool {
	return p.saveInfo(p)
}

// Save saves the Xray command report data to the configured location
func (p *XrayCommand) Save() bool {
	return p.saveInfo(p)
}

// Save saves the Lint command report data to the configured location
func (p *LintCommand) Save() bool {
	return p.saveInfo(p)
}
