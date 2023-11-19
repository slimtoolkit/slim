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
	check := &BadContainerCommands{
		Info: Info{
			ID:           "ID.20022",
			Name:         "Bad Container Commands",
			Description:  "Bad container commands",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20022",
			MainMessage:  "Bad container commands",
			MatchMessage: "Instruction: start=%d end=%d global_index=%d stage_id=%d stage_index=%d cmd=%s",
			Labels: map[string]string{
				LabelLevel: LevelInfo,
				LabelScope: ScopeStage,
			},
		},
		Names: []string{
			"vim",
			"shutdown",
			"mount",
			"kill",
			"top",
			"free",
			"service",
			"ssh",
			"ps",
		},
	}

	AllChecks = append(AllChecks, check)
}

type BadContainerCommands struct {
	Info
	Names []string
}

func (check *BadContainerCommands) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", check.ID, check.Name)
	result := &Result{
		Source: &check.Info,
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
					for _, cmdName := range check.Names {
						//TODO:
						//need to make it work with multiple commands in instruction
						//need better shell parsing first
						if strings.ToLower(args[0]) == cmdName {
							if !result.Hit {
								result.Hit = true
								result.Message = check.MainMessage
							}

							match := &Match{
								Stage:       stage,
								Instruction: inst,
								Message: fmt.Sprintf(check.MatchMessage,
									inst.StartLine,
									inst.EndLine,
									inst.GlobalIndex,
									inst.StageID,
									inst.StageIndex,
									cmdName),
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
