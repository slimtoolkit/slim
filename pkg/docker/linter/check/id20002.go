// Package check contains the linter checks
package check

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	check := &NoStages{
		Info: Info{
			ID:          "ID.20002",
			Name:        "No stages",
			Description: "No stages",
			DetailsURL:  "https://lint.dockersl.im/check/ID.20002",
			MainMessage: "No stages in Dockerfile",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type NoStages struct {
	Info
}

func (c *NoStages) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if len(ctx.Dockerfile.Stages) == 0 {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}
