// Package check contains the linter checks
package check

import (
	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/spec"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerignore"
	"github.com/docker-slim/docker-slim/pkg/docker/instruction"
)

type Context struct {
	DockerfilePath  string
	Dockerfile      *spec.Dockerfile
	BuildContextDir string
	Dockerignore    *dockerignore.Matcher
}

type Options struct {
}

type Info struct {
	ID           string
	Name         string
	Description  string
	MainMessage  string //can be a template with a format string
	MatchMessage string //can be a template with a format string
	DetailsURL   string
	Labels       map[string]string
}

const (
	LabelLevel       = "level"
	LabelScope       = "scope"
	LabelInstruction = "instruction"
	LabelApp         = "app"
	LabelShell       = "shell"
)

const (
	LevelAny   = "any"
	LevelFatal = "fatal" //parse or other errors that will result in image build failures
	LevelError = "error"
	LevelWarn  = "warn"
	LevelInfo  = "info"
	LevelStyle = "style"
)

const (
	ScopeAll          = "all"
	ScopeDockerfile   = "dockerfile"
	ScopeStage        = "stage"
	ScopeInstruction  = "instruction"
	ScopeDockerignore = "dockerignore"
	ScopeData         = "data"
	ScopeApp          = "app"
	ScopeShell        = "shell"
)

//Possible labels:
//"level" -> "info", "warn", "error", "style"
//"scope" -> "app", "shell", "instruction", "stage", "dockerfile", "all", "dockerignore", "data"
//"instruction" -> "list,of,instructions" (negative with !instruction)
//"app" -> "list,of,app names"
//"shell" -> "general or specific shell name"

func (i *Info) Get() *Info {
	return i
}

type Result struct {
	Source     *Info
	Hit        bool
	Message    string
	Matches    []*Match
	DetailsURL string
}

type Match struct {
	Stage       *spec.BuildStage
	Instruction *instruction.Field
	Message     string
}

type Runner interface {
	Get() *Info
	Run(opts *Options, ctx *Context) (*Result, error)
}

//regex-based (with rules)
//type PolicyCheck struct {
//	Info
//}

var AllChecks = []Runner{}
