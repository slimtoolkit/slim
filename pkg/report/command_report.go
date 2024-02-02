package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/reverse"
	"github.com/slimtoolkit/slim/pkg/docker/dockerimage"
	"github.com/slimtoolkit/slim/pkg/docker/linter/check"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/version"
)

// DefaultFilename is the default name for the command report
const DefaultFilename = "slim.report.json"

const tmpPath = "/tmp"

// Command is the common command report data
type Command struct {
	reportLocation string
	Version        string     `json:"version"`
	Engine         string     `json:"engine"`
	Containerized  bool       `json:"containerized"`
	HostDistro     DistroInfo `json:"host_distro"`
	//Docker         string  `json:"docker,omitempty"`
	Type  command.Type  `json:"type"`
	State command.State `json:"state"`
	Error string        `json:"error,omitempty"`
}

// ImageIdentity includes the container image identity fields
type ImageIdentity struct {
	ID          string   `json:"id"`
	Tags        []string `json:"tags,omitempty"`
	Names       []string `json:"names,omitempty"`
	Digests     []string `json:"digests,omitempty"`
	FullDigests []string `json:"full_digests,omitempty"`
}

// ImageMetadata provides basic image metadata
type ImageMetadata struct {
	Identity              ImageIdentity     `json:"identity"`
	Size                  int64             `json:"size"`
	SizeHuman             string            `json:"size_human"`
	CreateTime            string            `json:"create_time"`
	Author                string            `json:"author,omitempty"`
	Maintainers           []string          `json:"maintainers,omitempty"`
	DockerVersion         string            `json:"docker_version"`
	Architecture          string            `json:"architecture"`
	User                  string            `json:"user,omitempty"`
	ExposedPorts          []string          `json:"exposed_ports,omitempty"`
	OS                    string            `json:"os,omitempty"`
	Volumes               []string          `json:"volumes,omitempty"`
	Labels                map[string]string `json:"labels,omitempty"`
	EnvVars               []string          `json:"env_vars,omitempty"`
	WorkDir               string            `json:"workdir,omitempty"`
	InheritedInstructions []string          `json:"inherited_instructions,omitempty"`

	//TODO:
	//Should be in ImageReport dockerimage.ImageReport
	//because it's additional info discovered during analysis
	//BUT also need to find a way to make it available
	//for the 'build' command (at least, distro)
	Distro         *DistroInfo        `json:"distro,omitempty"`
	Buildpack      *BuildpackInfo     `json:"buildpack,omitempty"`
	ContainerEntry ContainerEntryInfo `json:"container_entry"`

	//Base image info
	BaseImageDigest string `json:"base_image_digest,omitempty"`
	BaseImageName   string `json:"base_image_name,omitempty"`
}

type ContainerEntryInfo struct {
	Entrypoint  []string             `json:"entrypoint,omitempty"`
	Cmd         []string             `json:"cmd,omitempty"`
	ExePath     string               `json:"exe_path"`
	FullExePath *ContainerFileInfo   `json:"full_exe_path,omitempty"`
	ExeArgs     []string             `json:"exe_args,omitempty"`
	ArgFiles    []*ContainerFileInfo `json:"arg_files,omitempty"`
}

type ContainerFileInfo struct {
	Name  string `json:"name"`
	Layer int    `json:"layer"`
}

type BuildpackInfo struct {
	Stack     string `json:"stack"`
	Vendor    string `json:"vendor,omitempty"`
	Buildpack string `json:"buildpack,omitempty"`
}

// SystemMetadata provides basic system metadata
type SystemMetadata struct {
	Type    string     `json:"type"`
	Release string     `json:"release"`
	Distro  DistroInfo `json:"distro"`
}

// Output Version for 'build'
const OVBuildCommand = "1.1"

// BuildCommand is the 'build' command report data
type BuildCommand struct {
	Command
	TargetReference        string               `json:"target_reference"`
	System                 SystemMetadata       `json:"system"`
	SourceImage            ImageMetadata        `json:"source_image"`
	MinifiedImageSize      int64                `json:"minified_image_size"`
	MinifiedImageSizeHuman string               `json:"minified_image_size_human"`
	MinifiedImage          string               `json:"minified_image"`
	MinifiedImageID        string               `json:"minified_image_id"`
	MinifiedImageDigest    string               `json:"minified_image_digest"`
	MinifiedImageHasData   bool                 `json:"minified_image_has_data"`
	MinifiedBy             float64              `json:"minified_by"`
	ArtifactLocation       string               `json:"artifact_location"`
	ContainerReportName    string               `json:"container_report_name"`
	SeccompProfileName     string               `json:"seccomp_profile_name"`
	AppArmorProfileName    string               `json:"apparmor_profile_name"`
	ImageStack             []*reverse.ImageInfo `json:"image_stack"`
	ImageCreated           bool                 `json:"image_created"`
	ImageBuildEngine       string               `json:"image_build_engine"`
}

// Output Version for 'profile'
const OVProfileCommand = "1.0"

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

// Output Version for 'xray'
const OVXrayCommand = "1.2.3"

// XrayCommand is the 'xray' command report data
type XrayCommand struct {
	Command
	TargetReference      string                            `json:"target_reference"`
	SourceImage          ImageMetadata                     `json:"source_image"`
	ArtifactLocation     string                            `json:"artifact_location"`
	ImageReport          *dockerimage.ImageReport          `json:"image_report,omitempty"`
	ImageStack           []*reverse.ImageInfo              `json:"image_stack"`
	ImageLayers          []*dockerimage.LayerReport        `json:"image_layers"`
	ImageArchiveLocation string                            `json:"image_archive_location"`
	RawImageManifest     *dockerimage.DockerManifestObject `json:"raw_image_manifest,omitempty"`
	RawImageConfig       *dockerimage.ConfigObject         `json:"raw_image_config,omitempty"`
}

// Output Version for 'lint'
const OVLintCommand = "1.0"

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

// Output Version for 'images'
const OVImagesCommand = "1.0"

// ImagesCommand is the 'images' command report data
type ImagesCommand struct {
	Command
}

// Output Version for 'containerize'
const OVContainerizeCommand = "1.0"

// ContainerizeCommand is the 'containerize' command report data
type ContainerizeCommand struct {
	Command
}

// Output Version for 'convert'
const OVConvertCommand = "1.0"

// ConvertCommand is the 'convert' command report data
type ConvertCommand struct {
	Command
}

// Output Version for 'merge'
const OVMergeCommand = "1.0"

// MergeCommand is the 'merge' command report data
type MergeCommand struct {
	Command
	FirstImage           string `json:"first_image"`
	LastImage            string `json:"last_image"`
	UseLastImageMetadata bool   `json:"use_last_image_metadata"`
}

// Output Version for 'edit'
const OVEditCommand = "1.0"

// EditCommand is the 'edit' command report data
type EditCommand struct {
	Command
}

// Output Version for 'debug'
const OVDebugCommand = "1.0"

// DebugCommand is the 'debug' command report data
type DebugCommand struct {
	Command
}

// Output Version for 'probe'
const OVProbeCommand = "1.0"

// ProbeCommand is the 'probe' command report data
type ProbeCommand struct {
	Command
}

// Output Version for 'server'
const OVServerCommand = "1.0"

// ServerCommand is the 'server' command report data
type ServerCommand struct {
	Command
}

// Output Version for 'run'
const OVRunCommand = "1.0"

// RunCommand is the 'run' command report data
type RunCommand struct {
	Command
	TargetReference string `json:"target_reference"`
}

// Output Version for 'registry'
const OVRegistryCommand = "1.0"

// RegistryCommand is the 'registry' command report data
type RegistryCommand struct {
	Command
	TargetReference string `json:"target_reference"`
}

// Output Version for 'vulnerability'
const OVVulnerabilityCommand = "1.0"

// VulnerabilityCommand is the 'vulnerability' command report data
type VulnerabilityCommand struct {
	Command
	Operation string `json:"operation"`
}

func (cmd *Command) init(containerized bool) {
	cmd.Containerized = containerized
	cmd.Engine = version.Current()

	hinfo := system.GetSystemInfo()
	cmd.HostDistro = DistroInfo{
		Name:        hinfo.Distro.Name,
		Version:     hinfo.Distro.Version,
		DisplayName: hinfo.Distro.DisplayName,
	}
}

// NewBuildCommand creates a new 'build' command report
func NewBuildCommand(reportLocation string, containerized bool) *BuildCommand {
	cmd := &BuildCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVBuildCommand, //build command 'results' version (report and artifacts)
			Type:           command.Build,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewProfileCommand creates a new 'profile' command report
func NewProfileCommand(reportLocation string, containerized bool) *ProfileCommand {
	cmd := &ProfileCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVProfileCommand, //profile command 'results' version (report and artifacts)
			Type:           command.Profile,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewXrayCommand creates a new 'xray' command report
func NewXrayCommand(reportLocation string, containerized bool) *XrayCommand {
	cmd := &XrayCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVXrayCommand, //xray command 'results' version (report and artifacts)
			Type:           command.Xray,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewLintCommand creates a new 'lint' command report
func NewLintCommand(reportLocation string, containerized bool) *LintCommand {
	cmd := &LintCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVLintCommand, //lint command 'results' version (report and artifacts)
			Type:           command.Lint,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewImagesCommand creates a new 'images' command report
func NewImagesCommand(reportLocation string, containerized bool) *ImagesCommand {
	cmd := &ImagesCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVImagesCommand, //images command 'results' version (report and artifacts)
			Type:           command.Images,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewContainerizeCommand creates a new 'containerize' command report
func NewContainerizeCommand(reportLocation string, containerized bool) *ContainerizeCommand {
	cmd := &ContainerizeCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVContainerizeCommand, //containerize command 'results' version (report and artifacts)
			Type:           command.Containerize,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewConvertCommand creates a new 'convert' command report
func NewConvertCommand(reportLocation string, containerized bool) *ConvertCommand {
	cmd := &ConvertCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVConvertCommand, //convert command 'results' version (report and artifacts)
			Type:           command.Convert,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewMergeCommand creates a new 'edit' command report
func NewMergeCommand(reportLocation string, containerized bool) *MergeCommand {
	cmd := &MergeCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVMergeCommand, //edit command 'results' version (report and artifacts)
			Type:           command.Merge,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewEditCommand creates a new 'edit' command report
func NewEditCommand(reportLocation string, containerized bool) *EditCommand {
	cmd := &EditCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVEditCommand, //edit command 'results' version (report and artifacts)
			Type:           command.Edit,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewDebugCommand creates a new 'debug' command report
func NewDebugCommand(reportLocation string, containerized bool) *DebugCommand {
	cmd := &DebugCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVDebugCommand, //debug command 'results' version (report and artifacts)
			Type:           command.Debug,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewProbeCommand creates a new 'probe' command report
func NewProbeCommand(reportLocation string, containerized bool) *ProbeCommand {
	cmd := &ProbeCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVProbeCommand, //probe command 'results' version (report and artifacts)
			Type:           command.Probe,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewServerCommand creates a new 'server' command report
func NewServerCommand(reportLocation string, containerized bool) *ServerCommand {
	cmd := &ServerCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVServerCommand, //server command 'results' version (report and artifacts)
			Type:           command.Server,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewRunCommand creates a new 'run' command report
func NewRunCommand(reportLocation string, containerized bool) *RunCommand {
	cmd := &RunCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVRunCommand, //run command 'results' version (report and artifacts)
			Type:           command.Run,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewRegistryCommand creates a new 'registry' command report
func NewRegistryCommand(reportLocation string, containerized bool) *RegistryCommand {
	cmd := &RegistryCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVRegistryCommand, //registry command 'results' version (report and artifacts)
			Type:           command.Registry,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
}

// NewVulnerabilityCommand creates a new 'registry' command report
func NewVulnerabilityCommand(reportLocation string, containerized bool) *VulnerabilityCommand {
	cmd := &VulnerabilityCommand{
		Command: Command{
			reportLocation: reportLocation,
			Version:        OVVulnerabilityCommand,
			Type:           command.Vulnerability,
			State:          command.StateUnknown,
		},
	}

	cmd.Command.init(containerized)
	return cmd
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

		err = os.WriteFile(p.reportLocation, reportData.Bytes(), 0644)
		if err != nil && os.IsPermission(err) {
			if pinfo, tmpErr := os.Stat(tmpPath); tmpErr == nil && pinfo.IsDir() {
				p.reportLocation = filepath.Join(tmpPath, DefaultFilename)
				log.Debugf("report.saveInfo - overriding command report file path to %v", p.reportLocation)
				err = os.WriteFile(p.reportLocation, reportData.Bytes(), 0644)
			} else {
				fmt.Printf("report.Command.saveInfo: not saving report file - '%s'\n", p.reportLocation)
				return false
			}
		}

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
