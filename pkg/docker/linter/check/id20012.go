// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/docker/instruction"
)

func init() {
	check := &NoWorkdirPath{
		Info: Info{
			ID:           "ID.20012",
			Name:         "No WORKDIR path",
			Description:  "No WORKDIR path",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20012",
			MainMessage:  "No WORKDIR path in stage",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type NoWorkdirPath struct {
	Info
}

func (c *NoWorkdirPath) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("check.NoWorkdirPath.Run[%s]", c.ID)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Workdir]; ok {
			for _, inst := range instructions {
				if len(inst.Args) == 0 {
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
