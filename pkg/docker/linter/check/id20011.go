// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &MultipleCmdInstructions{
		Info: Info{
			ID:           "ID.20011",
			Name:         "Multiple CMD instructions",
			Description:  "Multiple CMD instructions",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20011",
			MainMessage:  "Multiple CMD instructions in stage",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type MultipleCmdInstructions struct {
	Info
}

func (c *MultipleCmdInstructions) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Cmd]; ok {
			if len(instructions) > 1 {
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

	return result, nil
}
