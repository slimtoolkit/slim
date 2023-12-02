package help

import (
	"github.com/urfave/cli/v2"
)

const (
	Name  = "help"
	Usage = "Show help info"
	Alias = "h"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		cli.ShowAppHelp(ctx)
		return nil
	},
}
