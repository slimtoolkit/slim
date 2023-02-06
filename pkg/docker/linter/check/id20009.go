// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &StagelessInstruction{
		Info: Info{
			ID:           "ID.20009",
			Name:         "Stageless instruction",
			Description:  "Stageless instruction",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20009",
			MainMessage:  "Non-arg instruction outside of a Dockerfile stage",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeDockerfile,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type StagelessInstruction struct {
	Info
}

func (c *StagelessInstruction) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if len(ctx.Dockerfile.StagelessInstructions) > 0 {
		result.Hit = true
		result.Message = c.MainMessage

		for _, inst := range ctx.Dockerfile.StagelessInstructions {
			match := &Match{
				Instruction: inst,
				Message: fmt.Sprintf(c.MatchMessage,
					inst.StartLine,
					inst.EndLine,
					inst.Name,
					inst.GlobalIndex),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}
