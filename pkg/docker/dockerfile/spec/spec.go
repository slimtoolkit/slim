// Package spec describes the Dockerfile data model.
package spec

import (
	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

type ParentImage struct {
	Name           string      `json:"name,omitempty"`
	Tag            string      `json:"tag,omitempty"`
	Digest         string      `json:"digest,omitempty"`
	BuildArgAll    string      `json:"-"`
	BuildArgName   string      `json:"-"`
	BuildArgTag    string      `json:"-"`
	BuildArgDigest string      `json:"-"`
	HasEmptyName   bool        `json:"-"`
	HasEmptyTag    bool        `json:"-"`
	HasEmptyDigest bool        `json:"-"`
	ParentStage    *BuildStage `json:"-"`
}

type BuildStage struct {
	Index     int         `json:"index"`
	StartLine int         `json:"start_line"`
	EndLine   int         `json:"end_line"`
	Name      string      `json:"name,omitempty"`
	Parent    ParentImage `json:"parent"`

	AllInstructions           []*instruction.Field            `json:"-"`
	CurrentInstructions       []*instruction.Field            `json:"-"`
	OnBuildInstructions       []*instruction.Field            `json:"-"`
	UnknownInstructions       []*instruction.Field            `json:"-"`
	InvalidInstructions       []*instruction.Field            `json:"-"` //not including unknown instructions
	CurrentInstructionsByType map[string][]*instruction.Field `json:"-"`
	FromInstruction           *instruction.Field              `json:"-"`
	ArgInstructions           []*instruction.Field            `json:"-"`
	EnvInstructions           []*instruction.Field            `json:"-"`

	EnvVars            map[string]string      `json:"-"`
	BuildArgs          map[string]string      `json:"-"`
	FromArgs           map[string]string      `json:"-"` //"FROM" ARGs used by the stage
	UnknownFromArgs    map[string]struct{}    `json:"-"`
	IsUsed             bool                   `json:"-"`
	StageReferences    map[string]*BuildStage `json:"-"`
	ExternalReferences map[string]struct{}    `json:"-"`
}

func NewBuildStage() *BuildStage {
	return &BuildStage{
		CurrentInstructionsByType: map[string][]*instruction.Field{},
		EnvVars:                   map[string]string{},
		BuildArgs:                 map[string]string{},
		FromArgs:                  map[string]string{},
		UnknownFromArgs:           map[string]struct{}{},
		StageReferences:           map[string]*BuildStage{},
		ExternalReferences:        map[string]struct{}{},
	}
}

type Dockerfile struct {
	Name                  string
	Location              string
	Lines                 []string
	FromArgs              map[string]string //all "FROM" ARGs
	Stages                []*BuildStage
	StagesByName          map[string]*BuildStage
	LastStage             *BuildStage
	StagelessInstructions []*instruction.Field
	ArgInstructions       []*instruction.Field
	AllInstructions       []*instruction.Field
	InstructionsByType    map[string][]*instruction.Field
	UnknownInstructions   []*instruction.Field
	InvalidInstructions   []*instruction.Field //not including unknown instructions
	Warnings              []string
}

func NewDockerfile() *Dockerfile {
	return &Dockerfile{
		FromArgs:           map[string]string{},
		StagesByName:       map[string]*BuildStage{},
		InstructionsByType: map[string][]*instruction.Field{},
	}
}
