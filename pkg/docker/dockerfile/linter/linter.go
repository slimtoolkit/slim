// Package linter implements a Dockerfile linter
package linter

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/parser"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerfile/spec"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerignore"
	"github.com/docker-slim/docker-slim/pkg/docker/instruction"
)

var (
	ErrBadParams = errors.New("bad params")
)

//TODO:
//* support incremental, partial and instruction level linting
//* support linting from string

type Options struct {
	DockerfilePath   string
	Dockerfile       *spec.Dockerfile
	BuildContextDir  string
	SkipDockerIgnore bool //to disable .dockerignore parsing
	Dockerignore     *dockerignore.Matcher
	Selector         CheckSelector
	Config           map[string]*CheckOptions
}

type CheckContext struct {
	DockerfilePath  string
	Dockerfile      *spec.Dockerfile
	BuildContextDir string
	Dockerignore    *dockerignore.Matcher
}

type CheckSelector struct {
	IncludeCheckTags map[string]string
	IncludeCheckIDs  map[string]string
	ExcludeCheckIDs  map[string]string
	ExcludeCheckTags map[string]string
}

type CheckOptions struct {
}

const (
	StatusUnknown  = "unknown"
	StatusRunning  = "running"
	StatusComplete = "complete"
	StatusTimedOut = "timeout"
	StatusFailed   = "failed"
)

//TODO: add report name, report time, etc
type Report struct {
	Status          string
	BuildContextDir string
	Dockerfile      *spec.Dockerfile
	Dockerignore    *dockerignore.Matcher
	Hits            map[string]*CheckResult
	NoHits          map[string]*CheckResult
	Errors          map[string]error
}

func NewReport() *Report {
	return &Report{
		Status: StatusUnknown,
		Hits:   map[string]*CheckResult{},
		NoHits: map[string]*CheckResult{},
		Errors: map[string]error{},
	}
}

type Check struct {
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

func (c *Check) Info() *Check {
	return c
}

type CheckResult struct {
	Source     *Check
	Hit        bool
	Message    string
	Matches    []*CheckMatch
	DetailsURL string
}

type CheckMatch struct {
	Stage       *spec.BuildStage
	Instruction *instruction.Field
	Message     string
}

type Checker interface {
	Info() *Check
	Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error)
}

type CheckState struct {
	Check   Checker
	Options *CheckOptions
	Context *CheckContext
	Result  *CheckResult
	Error   error
}

type NoDockerignoreCheck struct {
	Check
}

func (c *NoDockerignoreCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	if ctx.Dockerignore == nil {
		return result, nil
	}

	if !ctx.Dockerignore.Exists {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}

type EmptyDockerignoreCheck struct {
	Check
}

func (c *EmptyDockerignoreCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	if ctx.Dockerignore == nil {
		return result, nil
	}

	if ctx.Dockerignore.Exists && len(ctx.Dockerignore.Patterns) == 0 {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}

type UnknownInstructionCheck struct {
	Check
}

func (c *UnknownInstructionCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	if len(ctx.Dockerfile.UnknownInstructions) > 0 {
		result.Hit = true
		result.Message = c.MainMessage

		for _, inst := range ctx.Dockerfile.UnknownInstructions {
			match := &CheckMatch{
				Instruction: inst,
				Message: fmt.Sprintf(c.MatchMessage,
					inst.StartLine,
					inst.EndLine,
					inst.Name,
					inst.GlobalIndex,
					inst.StageID,
					inst.StageIndex),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}

type StagelessInstructionCheck struct {
	Check
}

func (c *StagelessInstructionCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	if len(ctx.Dockerfile.StagelessInstructions) > 0 {
		result.Hit = true
		result.Message = c.MainMessage

		for _, inst := range ctx.Dockerfile.StagelessInstructions {
			match := &CheckMatch{
				Instruction: inst,
				Message: fmt.Sprintf(c.MatchMessage,
					inst.StartLine,
					inst.EndLine,
					inst.Name,
					inst.GlobalIndex),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}

type NoStagesCheck struct {
	Check
}

func (c *NoStagesCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	if len(ctx.Dockerfile.Stages) == 0 {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}

type NoStageArgsCheck struct {
	Check
}

func (c *NoStageArgsCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if stage.FromInstruction == nil {
			continue
		}

		if len(stage.FromInstruction.Args) == 0 {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &CheckMatch{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					stage.Index,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}

type InvalidStageArgsCheck struct {
	Check
}

func (c *InvalidStageArgsCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if stage.FromInstruction == nil {
			continue
		}

		if !stage.FromInstruction.IsValid {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &CheckMatch{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					"invalid from instruction",
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}

		if len(stage.FromInstruction.Args) == 2 ||
			len(stage.FromInstruction.Args) > 3 {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &CheckMatch{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					"incorrect number of arguments",
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}

		if len(stage.FromInstruction.Args) == 3 &&
			strings.ToLower(stage.FromInstruction.Args[1]) != "as" {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &CheckMatch{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					"malformed arguments",
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	for name, stageByName := range ctx.Dockerfile.StagesByName {
		for _, stageByIdx := range ctx.Dockerfile.Stages {
			if stageByIdx.Name != "" {
				if stageByIdx.Name == name && stageByIdx != stageByName {
					if !result.Hit {
						result.Hit = true
						result.Message = c.MainMessage
					}

					match := &CheckMatch{
						Stage: stageByIdx,
						Message: fmt.Sprintf(c.MatchMessage,
							"duplicate name",
							stageByIdx.Index,
							stageByIdx.Name,
							stageByIdx.StartLine,
							stageByIdx.EndLine),
					}

					result.Matches = append(result.Matches, match)
				}
			}
		}
	}

	return result, nil
}

type StageFromLatestCheck struct {
	Check
}

func (c *StageFromLatestCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if stage.Parent.Name != "" {
			if (stage.Parent.Tag == "" || strings.ToLower(stage.Parent.Tag) == "latest") &&
				stage.Parent.Digest == "" {
				if !result.Hit {
					result.Hit = true
					result.Message = c.MainMessage
				}

				match := &CheckMatch{
					Stage: stage,
					Message: fmt.Sprintf(c.MatchMessage,
						stage.Index,
						stage.Name,
						stage.StartLine,
						stage.EndLine,
						stage.Parent.Name),
				}

				result.Matches = append(result.Matches, match)
			}
		}
	}

	return result, nil
}

type EmptyStageCheck struct {
	Check
}

func (c *EmptyStageCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if len(stage.AllInstructions) == 1 {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &CheckMatch{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}

//regex-based (with rules)
type PolicyCheck struct {
	Check
}

type EmptyDockerfileCheck struct {
	Check
}

func (c *EmptyDockerfileCheck) Run(opts *CheckOptions, ctx *CheckContext) (*CheckResult, error) {
	result := &CheckResult{
		Source: &c.Check,
	}

	if len(ctx.Dockerfile.AllInstructions) == 0 {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}

var allChecks = []Checker{
	&NoDockerignoreCheck{
		Check: Check{
			ID:          "ID.10001",
			Name:        "Missing .dockerignore",
			Description: "Long description",
			DetailsURL:  "https://lint.dockersl.im/check/ID.10001",
			MainMessage: "No .dockerignore",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeDockerignore,
			},
		},
	},
	&EmptyDockerignoreCheck{
		Check: Check{
			ID:          "ID.10002",
			Name:        "Empty .dockerignore",
			Description: "Long description",
			DetailsURL:  "https://lint.dockersl.im/check/ID.10002",
			MainMessage: "No exclude patterns in .dockerignore",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeDockerignore,
			},
		},
	},

	&EmptyDockerfileCheck{
		Check: Check{
			ID:          "ID.20001",
			Name:        "Empty Dockerfile",
			Description: "Long description",
			DetailsURL:  "https://lint.dockersl.im/check/ID.20001",
			MainMessage: "No instructions in Dockerfile",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeDockerfile,
			},
		},
	},
	&NoStagesCheck{
		Check: Check{
			ID:          "ID.20002",
			Name:        "No stages",
			Description: "Long description",
			DetailsURL:  "https://lint.dockersl.im/check/ID.20002",
			MainMessage: "No stages in Dockerfile",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	},
	&EmptyStageCheck{
		Check: Check{
			ID:           "ID.20003",
			Name:         "Empty stage",
			Description:  "Long description",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20003",
			MainMessage:  "Empty stage in Dockerfile",
			MatchMessage: "Stage: index=%d name='%s' start=%d end=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	},
	&NoStageArgsCheck{
		Check: Check{
			ID:           "ID.20004",
			Name:         "No stage arguments",
			Description:  "Long description",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20004",
			MainMessage:  "Stage without arguments in Dockerfile",
			MatchMessage: "Stage: index=%d start=%d end=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	},
	&InvalidStageArgsCheck{
		Check: Check{
			ID:           "ID.20005",
			Name:         "Invalid stage arguments",
			Description:  "Long description",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20005",
			MainMessage:  "Stage with invalid arguments in Dockerfile",
			MatchMessage: "Stage: reason='%s' index=%d name='%s' start=%d end=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	},
	&StageFromLatestCheck{
		Check: Check{
			ID:           "ID.20006",
			Name:         "Stage from latest tag",
			Description:  "Long description",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20006",
			MainMessage:  "Stage from latest tag in Dockerfile",
			MatchMessage: "Stage: index=%d name='%s' start=%d end=%d parent='%s'",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	},
	&UnknownInstructionCheck{
		Check: Check{
			ID:           "ID.20007",
			Name:         "Unknown instruction",
			Description:  "Long description",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20007",
			MainMessage:  "Unknown instruction in Dockerfile",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelInfo,
				LabelScope: ScopeInstruction,
			},
		},
	},
	&StagelessInstructionCheck{
		Check: Check{
			ID:           "ID.20008",
			Name:         "Stageless instruction",
			Description:  "Long description",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20008",
			MainMessage:  "Non-arg instruction outside of a Dockerfile stage",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeDockerfile,
			},
		},
	},
}

func Execute(options Options) (*Report, error) {
	df := options.Dockerfile
	if df == nil {
		if options.DockerfilePath == "" {
			return nil, ErrBadParams
		}

		var err error
		df, err = parser.FromFile(options.DockerfilePath)
		if err != nil {
			return nil, err
		}
	}

	di := options.Dockerignore
	if di == nil && !options.SkipDockerIgnore {
		var err error
		di, err = dockerignore.Load(options.BuildContextDir)
		if err != nil {
			return nil, err
		}
	}

	report := NewReport()
	report.BuildContextDir = options.BuildContextDir
	report.Dockerfile = df
	report.Dockerignore = di

	var selectedChecks []Checker
	for _, check := range allChecks {
		info := check.Info()
		if len(options.Selector.ExcludeCheckIDs) == 0 {
			selectedChecks = append(selectedChecks, check)
		} else if _, ok := options.Selector.ExcludeCheckIDs[info.ID]; !ok {
			selectedChecks = append(selectedChecks, check)
		}
	}

	if len(selectedChecks) == 0 {
		report.Status = StatusComplete
		return report, nil
	}

	report.Status = StatusRunning

	checkContext := &CheckContext{
		DockerfilePath:  options.DockerfilePath,
		Dockerfile:      df,
		BuildContextDir: options.BuildContextDir,
		Dockerignore:    di,
	}

	stateCh := make(chan *CheckState, len(selectedChecks))
	var workers sync.WaitGroup
	workers.Add(len(selectedChecks))

	for _, check := range selectedChecks {
		go func(c Checker) {
			defer workers.Done()
			info := c.Info()
			var checkOptions *CheckOptions
			if len(options.Config) > 0 {
				checkOptions = options.Config[info.ID]
			}

			state := &CheckState{
				Check:   c,
				Options: checkOptions,
				Context: checkContext,
			}

			result, err := c.Run(checkOptions, checkContext)

			state.Result = result
			state.Error = err
			stateCh <- state
		}(check)
	}

	go func() {
		workers.Wait()
		close(stateCh)
	}()

	timeout := time.After(120 * time.Second)
done:
	for {
		select {
		case checkState, ok := <-stateCh:
			if !ok {
				report.Status = StatusComplete
				break done
			}
			if checkState != nil {
				info := checkState.Check.Info()
				if checkState.Error != nil {
					report.Errors[info.ID] = checkState.Error
				} else {
					if checkState.Result.Hit {
						report.Hits[info.ID] = checkState.Result
					} else {
						report.NoHits[info.ID] = checkState.Result
					}
				}
			}
		case <-timeout:
			report.Status = StatusTimedOut
			break done
		}
	}

	return report, nil
}
