// Package linter implements a Dockerfile linter
package linter

import (
	"errors"
	"sync"
	"time"

	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/parser"
	"github.com/slimtoolkit/slim/pkg/docker/dockerfile/spec"
	"github.com/slimtoolkit/slim/pkg/docker/dockerignore"
	"github.com/slimtoolkit/slim/pkg/docker/linter/check"

	log "github.com/sirupsen/logrus"
)

var (
	ErrBadParams = errors.New("bad params")
)

const (
	DockerfileTargetType = "dockerfile"
	ImageTargetType      = "image"
)

//TODO:
//* support incremental, partial and instruction level linting
//* support linting from string

type Options struct {
	DockerfilePath   string
	Dockerfile       *spec.Dockerfile
	SkipBuildContext bool
	BuildContextDir  string
	SkipDockerignore bool //to disable .dockerignore parsing
	Dockerignore     *dockerignore.Matcher
	Selector         CheckSelector
	Config           map[string]*check.Options
}

type CheckContext struct {
	DockerfilePath  string
	Dockerfile      *spec.Dockerfile
	BuildContextDir string
	Dockerignore    *dockerignore.Matcher
}

type CheckSelector struct {
	IncludeCheckLabels map[string]string
	IncludeCheckIDs    map[string]struct{}
	ExcludeCheckLabels map[string]string
	ExcludeCheckIDs    map[string]struct{}
}

const (
	StatusUnknown  = "unknown"
	StatusRunning  = "running"
	StatusComplete = "complete"
	StatusTimedOut = "timeout"
	StatusFailed   = "failed"
)

// TODO: add report name, report time, etc
type Report struct {
	Status          string
	BuildContextDir string
	Dockerfile      *spec.Dockerfile
	Dockerignore    *dockerignore.Matcher
	Hits            map[string]*check.Result
	NoHits          map[string]*check.Result
	Errors          map[string]error
}

func NewReport() *Report {
	return &Report{
		Status: StatusUnknown,
		Hits:   map[string]*check.Result{},
		NoHits: map[string]*check.Result{},
		Errors: map[string]error{},
	}
}

type CheckState struct {
	Check   check.Runner
	Options *check.Options
	Context *check.Context
	Result  *check.Result
	Error   error
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

	var buildContextDir string
	if !options.SkipBuildContext {
		if df != nil {
			buildContextDir = df.Location
		}

		if options.BuildContextDir != "" {
			buildContextDir = options.BuildContextDir
		}
	}

	di := options.Dockerignore
	if di == nil && !options.SkipDockerignore {
		var err error
		di, err = dockerignore.Load(buildContextDir)
		if err != nil {
			return nil, err
		}
	}

	report := NewReport()
	report.BuildContextDir = options.BuildContextDir
	report.Dockerfile = df
	report.Dockerignore = di

	var selectedChecks []check.Runner
	for _, check := range check.AllChecks {
		info := check.Get()

		if len(options.Selector.IncludeCheckIDs) > 0 {
			if _, ok := options.Selector.IncludeCheckIDs[info.ID]; ok {
				selectedChecks = append(selectedChecks, check)
				log.Debugf("linter.Execute: selected check - id=%v (IncludeCheckIDs)", info.ID)
			}
			continue
		}

		if len(options.Selector.IncludeCheckLabels) > 0 {
			for k, v := range info.Labels {
				if inval := options.Selector.IncludeCheckLabels[k]; inval == v {
					if len(options.Selector.ExcludeCheckIDs) == 0 {
						selectedChecks = append(selectedChecks, check)
						log.Debugf("linter.Execute: selected check - id=%v label=%v:%v (IncludeCheckLabels)", info.ID, k, v)
					} else if _, ok := options.Selector.ExcludeCheckIDs[info.ID]; !ok {
						selectedChecks = append(selectedChecks, check)
						log.Debugf("linter.Execute: selected check - id=%v label=%v:%v (IncludeCheckLabels/ExcludeCheckIDs)", info.ID, k, v)
					}

					continue
				}
			}

			continue
		}

		if len(options.Selector.ExcludeCheckLabels) > 0 {
			for k, v := range info.Labels {
				if inval := options.Selector.ExcludeCheckLabels[k]; inval == v {
					continue
				}
			}
		}

		if len(options.Selector.ExcludeCheckIDs) > 0 {
			if _, ok := options.Selector.ExcludeCheckIDs[info.ID]; ok {
				continue
			}
		}

		selectedChecks = append(selectedChecks, check)
		log.Debugf("linter.Execute: selected check - id=%v", info.ID)
	}

	if len(selectedChecks) == 0 {
		report.Status = StatusComplete
		return report, nil
	}

	report.Status = StatusRunning

	checkContext := &check.Context{
		DockerfilePath:  options.DockerfilePath,
		Dockerfile:      df,
		BuildContextDir: options.BuildContextDir,
		Dockerignore:    di,
	}

	stateCh := make(chan *CheckState, len(selectedChecks))
	var workers sync.WaitGroup
	workers.Add(len(selectedChecks))

	for _, c := range selectedChecks {
		go func(c check.Runner) {
			defer workers.Done()
			info := c.Get()
			var checkOptions *check.Options
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
		}(c)
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
				info := checkState.Check.Get()
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

func ListChecks() []*check.Info {
	var list []*check.Info
	for _, check := range check.AllChecks {
		info := check.Get()
		list = append(list, info)
	}

	return list
}
