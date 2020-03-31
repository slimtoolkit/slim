// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &UnknownInstruction{
		Info: Info{
			ID:           "ID.20007",
			Name:         "Unknown instruction",
			Description:  "Unknown instruction",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20007",
			MainMessage:  "Unknown instruction in Dockerfile",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelInfo,
				LabelScope: ScopeInstruction,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type UnknownInstruction struct {
	Info
}

func (c *UnknownInstruction) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if len(ctx.Dockerfile.UnknownInstructions) > 0 {
		result.Hit = true
		result.Message = c.MainMessage

		for _, inst := range ctx.Dockerfile.UnknownInstructions {
			match := &Match{
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
