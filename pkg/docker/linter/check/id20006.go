// Package check contains the linter checks
package check

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

func init() {
	check := &StageFromLatest{
		Info: Info{
			ID:           "ID.20006",
			Name:         "Stage from latest tag",
			Description:  "Stage from latest tag",
			DetailsURL:   "https://lint.dockersl.im/check/ID.20006",
			MainMessage:  "Stage from latest tag in Dockerfile",
			MatchMessage: "Stage: index=%d name='%s' start=%d end=%d parent='%s'",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type StageFromLatest struct {
	Info
}

func (c *StageFromLatest) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	for _, stage := range ctx.Dockerfile.Stages {
		if stage.Parent.Name != "" {
			if (stage.Parent.Tag == "" || strings.ToLower(stage.Parent.Tag) == "latest") &&
				stage.Parent.Digest == "" {
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
						stage.EndLine,
						stage.Parent.Name),
				}

				result.Matches = append(result.Matches, match)
			}
		}
	}

	return result, nil
}
