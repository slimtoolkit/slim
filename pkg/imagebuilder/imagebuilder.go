package imagebuilder

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

// ImageConfig describes the container image configurations (aka ConfigFile or V1Image/Image in other libraries)
// Fields (ordered according to spec):
// * https://github.com/opencontainers/image-spec/blob/main/config.md#properties
// * https://github.com/moby/moby/blob/e1c92184f08153456ecbf5e302a851afd6f28e1c/image/image.go#LL40C6-L40C13
// Note: related to pkg/docker/dockerimage/V1ConfigObject|ConfigObject
// TODO: refactor into one set of common structs later
type ImageConfig struct {
	Created      time.Time `json:"created,omitempty"`
	Author       string    `json:"author,omitempty"`
	Architecture string    `json:"architecture"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"os.version,omitempty"`
	OSFeatures   []string  `json:"os.features,omitempty"`
	Variant      string    `json:"variant,omitempty"`
	Config       RunConfig `json:"config"`
	RootFS       *RootFS   `json:"rootfs"`            //not used building images
	History      []History `json:"history,omitempty"` //not used building images
	//Extra fields
	Container     string `json:"container,omitempty"`
	DockerVersion string `json:"docker_version,omitempty"`
	//More extra fields
	ID      string `json:"id,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids,omitempty"`
}

type History struct {
	Created    string `json:"created,omitempty"`
	Author     string `json:"author,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
	Comment    string `json:"comment,omitempty"`
	EmptyLayer bool   `json:"empty_layer,omitempty"`
}

// RunConfig describes the runtime config parameters for container instances (aka Config in other libraries)
// Fields (ordered according to spec): Memory, MemorySwap, CpuShares aren't necessary
// * https://github.com/opencontainers/image-spec/blob/main/config.md#properties (Config field)
// * https://github.com/moby/moby/blob/master/api/types/container/config.go#L70
// Note: related to pkg/docker/dockerimage/ContainerConfig
// TODO: refactor into one set of common structs later
type RunConfig struct {
	User         string              `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	ArgsEscaped  bool                `json:"ArgsEscaped,omitempty"`
	Healthcheck  *HealthConfig       `json:"Healthcheck,omitempty"`
	//Extra fields
	AttachStderr    bool     `json:"AttachStderr,omitempty"`
	AttachStdin     bool     `json:"AttachStdin,omitempty"`
	AttachStdout    bool     `json:"AttachStdout,omitempty"`
	Domainname      string   `json:"Domainname,omitempty"`
	Hostname        string   `json:"Hostname,omitempty"`
	Image           string   `json:"Image,omitempty"`
	OnBuild         []string `json:"OnBuild,omitempty"`
	OpenStdin       bool     `json:"OpenStdin,omitempty"`
	StdinOnce       bool     `json:"StdinOnce,omitempty"`
	Tty             bool     `json:"Tty,omitempty"`
	NetworkDisabled bool     `json:"NetworkDisabled,omitempty"`
	MacAddress      string   `json:"MacAddress,omitempty"`
	StopTimeout     *int     `json:"StopTimeout,omitempty"`
	Shell           []string `json:"Shell,omitempty"`
}

type HealthConfig struct {
	Test        []string      `json:",omitempty"`
	Interval    time.Duration `json:",omitempty"`
	Timeout     time.Duration `json:",omitempty"`
	StartPeriod time.Duration `json:",omitempty"`
	Retries     int           `json:",omitempty"`
}

type SimpleBuildOptions struct {
	From        string
	Tags        []string
	Layers      []LayerDataInfo
	ImageConfig ImageConfig

	/*
	   //todo:  add 'Healthcheck'
	   Entrypoint   []string
	   Cmd          []string
	   WorkDir      string
	   User         string
	   StopSignal   string
	   OnBuild      []string
	   Volumes      map[string]struct{}
	   EnvVars      []string
	   ExposedPorts map[string]struct{}
	   Labels       map[string]string
	   Architecture string
	*/
}

type LayerSourceType string

const (
	TarSource LayerSourceType = "lst.tar"
	DirSource LayerSourceType = "lst.dir"
)

type LayerDataInfo struct {
	Type   LayerSourceType
	Source string
	Params *DataParams
	//TODO: add other common layer metadata...
}

type DataParams struct {
	TargetPath string
	//TODO: add useful fields (e.g., to filter directory files or to use specific file perms, etc)
}

type ImageResult struct {
	ID        string   `json:"id,omitempty"`
	Digest    string   `json:"digest,omitempty"`
	Name      string   `json:"name,omitempty"`
	OtherTags []string `json:"other_tags,omitempty"`
}

type SimpleBuildEngine interface {
	Name() string
	Build(options SimpleBuildOptions) (*ImageResult, error)
}

func SimpleBuildOptionsFromDockerfileData(data string, ignoreExeInstructions bool) (*SimpleBuildOptions, error) {
	var options SimpleBuildOptions
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		instName := strings.ToLower(parts[0])
		switch instName {
		case instruction.Entrypoint:
			//options.Entrypoint []string
		case instruction.Cmd:
			//options.Cmd []string
		case instruction.Env:
			//options.EnvVars []string
		case instruction.Expose:
			//options.ExposedPorts map[string]struct{}
		case instruction.Label:
			//options.Labels map[string]string
		case instruction.User:
			//options.User = parts[1]
			options.ImageConfig.Config.User = parts[1]
		case instruction.Volume:
			//options.Volumes map[string]struct{}
		case instruction.Workdir:
			//options.WorkDir = parts[1]
			options.ImageConfig.Config.WorkingDir = parts[1]
		case instruction.Add:
			//support tar files (ignore other things, at leas, for now)
			//options.Layers []LayerDataInfo
		case instruction.Copy:
			//options.Layers []LayerDataInfo
		case instruction.Maintainer:
			//TBD
		case instruction.Healthcheck:
			//TBD
		case instruction.From:
			//options.From string
		case instruction.Arg:
			//TODO
		case instruction.Run:
			if !ignoreExeInstructions {
				return nil, fmt.Errorf("RUN instructions are not supported")
			}
		case instruction.Onbuild:
			//IGNORE
		case instruction.Shell:
			//IGNORE
		case instruction.StopSignal:
			//IGNORE
		}
	}
	return &options, nil
}

func SimpleBuildOptionsFromDockerfile(path string, ignoreExeInstructions bool) (*SimpleBuildOptions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return SimpleBuildOptionsFromDockerfileData(string(data), ignoreExeInstructions)
}

func SimpleBuildOptionsFromImageConfig(data *ImageConfig) (*SimpleBuildOptions, error) {
	return &SimpleBuildOptions{ImageConfig: *data}, nil
}
