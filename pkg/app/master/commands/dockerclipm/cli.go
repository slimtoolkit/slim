package dockerclipm

import (
	"encoding/json"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/version"
)

const (
	Name  = "docker-cli-plugin-metadata"
	Usage = "Plugin metadata for the docker cli"
)

type pluginMetadata struct {
	SchemaVersion    string
	Vendor           string
	Version          string
	ShortDescription string
	URL              string
}

var CLI = &cli.Command{
	Category: "internal.metadata",
	Name:     Name,
	Usage:    Usage,
	Action: func(ctx *cli.Context) error {
		metadata := pluginMetadata{
			SchemaVersion:    "0.1.0",
			Vendor:           "DockerSlim",
			Version:          version.Current(),
			ShortDescription: "DockerSlim commands (build=minify, xray=static analyze, profile=dynamic analyze, lint=validate, more)",
			URL:              "https://dockersl.im",
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "    ")
		encoder.Encode(metadata)
		return nil
	},
}
