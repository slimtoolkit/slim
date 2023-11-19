// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &MalformedInstExecForm{
		Info: Info{
			ID:           "ID.20013",
			Name:         "Malformed instruction in exec/JSON form",
			Description:  "Malformed instruction in exec/JSON form",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20013",
			MainMessage:  "Malformed instruction in exec/JSON form",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type MalformedInstExecForm struct {
	Info
}

func (c *MalformedInstExecForm) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		for _, name := range instruction.SupportsJSONForm() {
			if instructions, ok := stage.CurrentInstructionsByType[name]; ok {
				for _, inst := range instructions {
					if inst.IsJSONForm {
						continue
					}

					if strings.HasPrefix(inst.ArgsRaw, "[") ||
						strings.HasSuffix(inst.ArgsRaw, "]") {
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
									inst.Name,
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
	}

	return result, nil
}
