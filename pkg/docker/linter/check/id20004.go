// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &NoStageArgs{
		Info: Info{
			ID:           "ID.20004",
			Name:         "No stage arguments",
			Description:  "No stage arguments",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20004",
			MainMessage:  "Stage without arguments in Dockerfile",
			MatchMessage: "Stage: index=%d start=%d end=%d",
			Labels: map[string]string{
				LabelLevel: LevelFatal,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type NoStageArgs struct {
	Info
}

func (c *NoStageArgs) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if stage.FromInstruction == nil {
			continue
		}

		if len(stage.FromInstruction.ArgsRaw) == 0 {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &Match{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					stage.Index,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}
