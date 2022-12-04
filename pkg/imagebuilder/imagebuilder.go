package imagebuilder

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/docker-slim/docker-slim/pkg/docker/instruction"
)

type SimpleBuildOptions struct {
	From         string
	Entrypoint   []string
	Cmd          []string
	WorkDir      string
	User         string
	Volumes      map[string]struct{}
	EnvVars      []string
	ExposedPorts map[string]struct{}
	Labels       map[string]string
	Tags         []string
	Layers       []LayerDataInfo
	//todo:  add 'Healthcheck'
	Architecture string
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

type SimpleBuildEngine interface {
	Name() string
	Build(options SimpleBuildOptions) error
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
			options.User = parts[1]
		case instruction.Volume:
			//options.Volumes map[string]struct{}
		case instruction.Workdir:
			options.WorkDir = parts[1]
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
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return SimpleBuildOptionsFromDockerfileData(string(data), ignoreExeInstructions)
}
