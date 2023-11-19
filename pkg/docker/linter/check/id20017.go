// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &LastUserRoot{
		Info: Info{
			ID:           "ID.20017",
			Name:         "Last USER instruction with root",
			Description:  "Last USER instruction with root",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20017",
			MainMessage:  "Last USER instruction with root",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelInfo,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type LastUserRoot struct {
	Info
}

func (c *LastUserRoot) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	lastStageIdx := len(ctx.Dockerfile.Stages) - 1
	if lastStageIdx > -1 {
		stage := ctx.Dockerfile.Stages[lastStageIdx]

		if instructions, ok := stage.CurrentInstructionsByType[instruction.User]; ok {
			lastUserIdx := len(instructions) - 1
			if lastUserIdx > -1 {
				inst := instructions[lastUserIdx]
				argsRaw := strings.ToLower(inst.ArgsRaw)
				if argsRaw == "0" ||
					argsRaw == "root" ||
					strings.HasPrefix(argsRaw, "0:") ||
					strings.HasPrefix(argsRaw, "root:") {

					result.Hit = true
					result.Message = c.MainMessage

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
		//if no explicit USER instruction (parent's USER is inherited)
	}

	return result, nil
}
