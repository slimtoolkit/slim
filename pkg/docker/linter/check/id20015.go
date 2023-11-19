// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &RelativeWorkdir{
		Info: Info{
			ID:           "ID.20015",
			Name:         "Relative WORKDIR path",
			Description:  "Relative WORKDIR path",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20015",
			MainMessage:  "Relative WORKDIR path in stage",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type RelativeWorkdir struct {
	Info
}

func (c *RelativeWorkdir) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Workdir]; ok {
			for _, inst := range instructions {
				if len(inst.ArgsRaw) > 0 {
					workdirPath := inst.ArgsRaw
					if strings.Contains(workdirPath, "$") {
						workdirPath = expandEnvVars(workdirPath, stage.EnvVars)
					}

					if strings.HasPrefix(workdirPath, "/") {
						continue
					}

					if !result.Hit {
						result.Hit = true
						result.Message = c.MainMessage
					}

					for _, inst := range instructions {
						match := &Match{
							Stage:       stage,
							Instruction: inst,
							Message: fmt.Sprintf(c.MatchMessage,
								inst.StartLine,
								inst.EndLine,
								inst.GlobalIndex,
								inst.StageID,
								inst.StageIndex),
						}

						result.Matches = append(result.Matches, match)
					}
				}
			}
		}
	}

	return result, nil
}

func expandEnvVars(data string, vars map[string]string) string {
	//todo: do it the right way
	if strings.HasPrefix(data, "$") {
		name := data[1:]
		if val, ok := vars[name]; ok {
			return val
		}
	}

	return data
}
