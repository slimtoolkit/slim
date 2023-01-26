// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &EmptyStage{
		Info: Info{
			ID:           "ID.20003",
			Name:         "Empty stage",
			Description:  "Empty stage",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20003",
			MainMessage:  "Empty stage in Dockerfile",
			MatchMessage: "Stage: index=%d name='%s' start=%d end=%d",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type EmptyStage struct {
	Info
}

func (c *EmptyStage) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if len(stage.AllInstructions) == 1 {
			if !result.Hit {
				result.Hit = true
				result.Message = c.MainMessage
			}

			match := &Match{
				Stage: stage,
				Message: fmt.Sprintf(c.MatchMessage,
					stage.Index,
					stage.Name,
					stage.StartLine,
					stage.EndLine),
			}

			result.Matches = append(result.Matches, match)
		}
	}

	return result, nil
}
