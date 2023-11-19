// Package check contains the linter checks
package check

import (
	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/spec"
	"github.com/slimtoolkit/slim/pkg/docker/dockerignore"
	"github.com/slimtoolkit/slim/pkg/docker/instruction"
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
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"-"`
	MainMessage  string            `json:"-"` //can be a template with a format string
	MatchMessage string            `json:"-"` //can be a template with a format string
	DetailsURL   string            `json:"-"`
	Labels       map[string]string `json:"-"`
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
	Source     *Info    `json:"source"`
	Hit        bool     `json:"-"`
	Message    string   `json:"message,omitempty"`
	Matches    []*Match `json:"matches,omitempty"`
	DetailsURL string   `json:"-"`
}

type Match struct {
	Stage       *spec.BuildStage   `json:"stage,omitempty"`
	Instruction *instruction.Field `json:"instruction,omitempty"`
	Message     string             `json:"message,omitempty"`
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
