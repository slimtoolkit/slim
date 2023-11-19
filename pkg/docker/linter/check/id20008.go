// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &DeprecatedInstruction{
		Info: Info{
			ID:           "ID.20008",
			Name:         "Deprecated instruction",
			Description:  "Deprecated instruction",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20008",
			MainMessage:  "Deprecated instruction",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelInfo,
				LabelScope: ScopeDockerfile,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type DeprecatedInstruction struct {
	Info
}

func (c *DeprecatedInstruction) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if instructions, ok := ctx.Dockerfile.InstructionsByType[instruction.Maintainer]; ok {
		result.Hit = true
		result.Message = c.MainMessage

		for _, inst := range instructions {
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
