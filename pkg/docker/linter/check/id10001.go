// Package check contains the linter checks
package check

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	check := &NoDockerignore{
		Info: Info{
			ID:          "ID.10001",
			Name:        "Missing .dockerignore",
			Description: "Missing .dockerignore",
			DetailsURL:  "https://lint.dockersl.im/check/ID.10001",
			MainMessage: "No .dockerignore",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeDockerignore,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type NoDockerignore struct {
	Info
}

func (c *NoDockerignore) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if ctx.Dockerignore == nil {
		return result, nil
	}

	if !ctx.Dockerignore.Exists {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}
