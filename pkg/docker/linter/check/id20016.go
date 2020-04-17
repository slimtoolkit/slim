// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &NoEnvArgs{
		Info: Info{
			ID:           "ID.20016",
			Name:         "No instruction args",
			Description:  "No instruction args",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20016",
			MainMessage:  "No instruction args in stage",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
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
		for _, inst := range stage.CurrentInstructions {
			if len(inst.ArgsRaw) == 0 {
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
						inst.Name,
						inst.GlobalIndex,
						inst.StageID,
						inst.StageIndex),
				}

				result.Matches = append(result.Matches, match)
			}
		}
	}

	return result, nil
}
