// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &UnnecessaryLayer{
		Info: Info{
			ID:           "ID.20019",
			Name:         "Unnecessary Layer",
			Description:  "RUN instruction will result in unnecessary layer (combine command with previous RUN instruciton)",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20019",
			MainMessage:  "RUN instruction will result in unnecessary layer",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d prev_start=%d prev_end=%d prev_global_index=%d prev_stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type UnnecessaryLayer struct {
	Info
}

func (c *UnnecessaryLayer) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {

		var prevInst *instruction.Field
		for _, inst := range stage.CurrentInstructions {

			if inst.Name == instruction.Run &&
				prevInst != nil &&
				prevInst.Name == instruction.Run {
				//very primitive unnecessary layer that only checks the previous RUN instruction
				//should have a separate check with more advanced unnecessary layer detection
				if !result.Hit {
					result.Hit = true
					result.Message = c.MainMessage
				}

				match := &Match{
					Stage:       stage,
					Instruction: inst,
					Message: fmt.Sprintf(c.MatchMessage,
						inst.StartLine,
						inst.EndLine,
						inst.GlobalIndex,
						inst.StageID,
						inst.StageIndex,
						prevInst.StartLine,
						prevInst.EndLine,
						prevInst.GlobalIndex,
						prevInst.StageIndex),
				}

				result.Matches = append(result.Matches, match)
			}

			prevInst = inst
		}
	}

	return result, nil
}
