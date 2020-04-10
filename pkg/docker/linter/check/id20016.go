// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/docker/instruction"
)

func init() {
	check := &NoEnvArgs{
		Info: Info{
			ID:           "ID.20016",
			Name:         "No ENV args",
			Description:  "No ENV args",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20016",
			MainMessage:  "No ENV args in stage",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelFatal,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type NoEnvArgs struct {
	Info
}

func (c *NoEnvArgs) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Env]; ok {
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
