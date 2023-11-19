// Package check contains the linter checks
package check

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/docker/instruction"
)

const (
	maxLayerCount = 100
)

func init() {
	check := &TooManyLayers{
		Info: Info{
			ID:          "ID.20020",
			Name:        "Too Many Layers",
			Description: "Too many image layers",
			DetailsURL:  "https://lint.dockersl.im/check/ID.20020",
			MainMessage: "Too many image layers in stage: count=%d",
			Labels: map[string]string{
				LabelLevel: LevelWarn,
				LabelScope: ScopeStage,
			},
		},
	}

	AllChecks = append(AllChecks, check)
}

type TooManyLayers struct {
	Info
}

func (c *TooManyLayers) Run(opts *Options, ctx *Context) (*Result, error) {
	log.Debugf("linter.check[%s:'%s']", c.ID, c.Name)
	result := &Result{
		Source: &c.Info,
	}

	//NOTES:
	//A regular Dockerfile lint check won't be very accurate because:
	//* the check doesn't know how many layers exist in the base image
	//* it doesn't know the storage driver and its exact limit
	//* not all RUN instructions generate new layers (e.g., "RUN python -V")
	//* not all WORKDIR instructions generate new layers (new layer only when dir doesn't exist)
	for _, stage := range ctx.Dockerfile.Stages {
		layerCount := 0
		for _, inst := range stage.CurrentInstructions {
			switch inst.Name {
			case instruction.Run,
				instruction.Workdir,
				instruction.Copy,
				instruction.Add:
				layerCount++
			}
		}

		if layerCount > maxLayerCount {
			result.Hit = true
			result.Message = fmt.Sprintf(c.MainMessage, layerCount)
		}
	}

	return result, nil
}
