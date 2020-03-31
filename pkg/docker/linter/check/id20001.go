// Package check contains the linter checks
package check

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	check := &EmptyDockerfile{
		Info: Info{
			ID:          "ID.20001",
			Name:        "Empty Dockerfile",
			Description: "Empty Dockerfile",
			DetailsURL:  "https://lint.dockersl.im/check/ID.20001",
			MainMessage: "No instructions in Dockerfile",
			Labels: map[string]string{
				LabelLevel: LevelError,
				LabelScope: ScopeDockerfile,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type EmptyDockerfile struct {
	Info
}

func (c *EmptyDockerfile) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	if len(ctx.Dockerfile.AllInstructions) == 0 {
		result.Hit = true
		result.Message = c.MainMessage
	}

	return result, nil
}
