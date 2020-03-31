// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &InvalidInstruction{
		Info: Info{
			ID:           "ID.20000",
			Name:         "Invalid instruction",
			Description:  "Invalid instruction",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20000",
			MainMessage:  "Invalid instruction in Dockerfile",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelFatal,
				LabelScope: ScopeDockerfile,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type InvalidInstruction struct {
	Info
}

func (c *InvalidInstruction) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, inst := range ctx.Dockerfile.InvalidInstructions {
		if !inst.IsValid {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &Match{
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

	return result, nil
}
