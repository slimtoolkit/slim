// Package spec describes the Dockerfile data model.
package spec

import (
	"github.com/docker-slim/docker-slim/pkg/docker/instruction"
)

type ParentImage struct {
	Name           string
	Tag            string
	Digest         string
	BuildArgAll    string
	BuildArgName   string
	BuildArgTag    string
	BuildArgDigest string
	HasEmptyName   bool
	HasEmptyTag    bool
	HasEmptyDigest bool
	ParentStage    *BuildStage
}

type BuildStage struct {
	Index     int
	StartLine int
	EndLine   int
	Name      string
	Parent    ParentImage

	AllInstructions           []*instruction.Field
	CurrentInstructions       []*instruction.Field
	OnBuildInstructions       []*instruction.Field
	UnknownInstructions       []*instruction.Field
	InvalidInstructions       []*instruction.Field //not including unknown instructions
	CurrentInstructionsByType map[string][]*instruction.Field
	FromInstruction           *instruction.Field
	ArgInstructions           []*instruction.Field
	EnvInstructions           []*instruction.Field

	EnvVars            map[string]string
	BuildArgs          map[string]string
	FromArgs           map[string]string //"FROM" ARGs used by the stage
	UnknownFromArgs    map[string]struct{}
	IsUsed             bool
	StageReferences    map[string]*BuildStage
	ExternalReferences map[string]struct{}
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
	FromArgs              map[string]string //all "FROM" ARGs
	Stages                []*BuildStage
	StagesByName          map[string]*BuildStage
	LastStage             *BuildStage
	StagelessInstructions []*instruction.Field
	ArgInstructions       []*instruction.Field
	AllInstructions       []*instruction.Field
	UnknownInstructions   []*instruction.Field
	InvalidInstructions   []*instruction.Field //not including unknown instructions
	Warnings              []string
}

func NewDockerfile() *Dockerfile {
	return &Dockerfile{
		FromArgs:     map[string]string{},
		StagesByName: map[string]*BuildStage{},
	}
}
