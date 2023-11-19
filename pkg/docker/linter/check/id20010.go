// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &MultipleEntrypointInstructions{
		Info: Info{
			ID:           "ID.20010",
			Name:         "Multiple ENTRYPOINT instructions",
			Description:  "Multiple ENTRYPOINT instructions",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20010",
			MainMessage:  "Multiple ENTRYPOINT instructions in stage",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type MultipleEntrypointInstructions struct {
	Info
}

func (c *MultipleEntrypointInstructions) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Entrypoint]; ok {
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
