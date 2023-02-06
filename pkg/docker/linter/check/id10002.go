// Package check contains the linter checks
package check

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	check := &EmptyDockerignore{
		Info: Info{
			ID:          "ID.10002",
			Name:        "Empty .dockerignore",
			Description: "Empty .dockerignore",
			DetailsURL:  "https://lint.dockersl.im/check/ID.10002",
			MainMessage: "No exclude patterns in .dockerignore",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeDockerignore,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type EmptyDockerignore struct {
	Info
}

func (c *EmptyDockerignore) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if ctx.Dockerignore == nil {
		return result, nil
	}

	if ctx.Dockerignore.Exists && len(ctx.Dockerignore.Patterns) == 0 {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}
