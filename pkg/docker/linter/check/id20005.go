// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &InvalidStageArgs{
		Info: Info{
			ID:           "ID.20005",
			Name:         "Invalid stage arguments",
			Description:  "Invalid stage arguments",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20005",
			MainMessage:  "Stage with invalid arguments in Dockerfile",
			MatchMessage: "Stage: reason='%s' index=%d name='%s' start=%d end=%d",
			Labels: map[string]string{
				LabelLevel: LevelFatal,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type InvalidStageArgs struct {
	Info
}

func (c *InvalidStageArgs) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if stage.FromInstruction == nil {
			continue
		}

		if !stage.FromInstruction.IsValid {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &Match{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					"invalid from instruction",
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}

		//FROM args are always parsed
		if len(stage.FromInstruction.Args) == 2 ||
			len(stage.FromInstruction.Args) > 3 {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &Match{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					"incorrect number of arguments",
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}

		if len(stage.FromInstruction.Args) == 3 &&
			strings.ToLower(stage.FromInstruction.Args[1]) != "as" {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &Match{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					"malformed arguments",
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	for name, stageByName := range ctx.Dockerfile.StagesByName {
		for _, stageByIdx := range ctx.Dockerfile.Stages {
			if stageByIdx.Name != "" {
				if stageByIdx.Name == name && stageByIdx != stageByName {
					if !result.Hit {
						result.Hit = true
						result.Message = c.MainMessage
					}

					match := &Match{
						Stage: stageByIdx,
						Message: fmt.Sprintf(c.MatchMessage,
							"duplicate name",
							stageByIdx.Index,
							stageByIdx.Name,
							stageByIdx.StartLine,
							stageByIdx.EndLine),
					}

					result.Matches = append(result.Matches, match)
				}
			}
		}
	}

	return result, nil
}
