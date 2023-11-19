// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &EntrypointCmdShellForm{
		Info: Info{
			ID:           "ID.20012",
			Name:         "ENTRYPOINT or CMD in shell form",
			Description:  "ENTRYPOINT or CMD should use the exec/JSON form",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20012",
			MainMessage:  "Instruction in shell form",
			MatchMessage: "Instruction: start=%d end=%d name='%s' global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeStage,
			},
		},
		Names: []string{
			instruction.Entrypoint,
			instruction.Cmd,
		},
	}

	AllChecks = append(AllChecks, check)
}

type EntrypointCmdShellForm struct {
	Info
	Names []string
}

func (c *EntrypointCmdShellForm) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		for _, name := range c.Names {
			if instructions, ok := stage.CurrentInstructionsByType[name]; ok {
				for _, inst := range instructions {
					if !inst.IsJSONForm {
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
