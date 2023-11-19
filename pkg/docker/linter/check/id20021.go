// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

func init() {
	check := &SeparateRemove{
		Info: Info{
			ID:           "ID.20021",
			Name:         "Separate rm Command",
			Description:  "Separate rm command only hides data",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20021",
			MainMessage:  "Separate rm command only hides data in the image (also creates a new layer)",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type SeparateRemove struct {
	Info
}

func (c *SeparateRemove) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if instructions, ok := stage.CurrentInstructionsByType[instruction.Run]; ok {
			for _, inst := range instructions {
				if len(inst.ArgsRaw) == 0 {
					continue
				}

				var args []string
				if inst.IsJSONForm {
					args = inst.Args
				} else {
					var err error
					args, err = shlex.Split(inst.ArgsRaw)
					if err != nil {
						log.Fatal(err)
					}
				}

				if len(args) > 1 {
					if strings.ToLower(args[0]) == "rm" {
						//can also check/track if it's the only RUN instruction
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
								inst.StageIndex),
						}

						result.Matches = append(result.Matches, match)
					}

				}
			}
		}
	}

	return result, nil
}
