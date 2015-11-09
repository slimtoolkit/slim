package main

import (
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

const (
	VERSION = "0.6"
	USAGE   = "lean and mean Docker containers :-)"
)

var app *cli.App

func init() {
	app = cli.NewApp()
	app.Version = VERSION
	app.Name = "docker-slim"
	app.Usage = USAGE
	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Println("unknown command -", command, "\n")
		cli.ShowAppHelp(ctx)
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug logs",
		},
		cli.StringFlag{
			Name:  "log",
			Usage: "log file to store logs",
		},
		cli.StringFlag{
			Name:  "log-format",
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool("debug") {
			log.SetLevel(log.DebugLevel)
		}
		if path := ctx.GlobalString("log"); path != "" {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			log.SetOutput(f)
		}
		switch ctx.GlobalString("log-format") {
		case "text":
		case "json":
			log.SetFormatter(new(log.JSONFormatter))
		default:
			log.Fatalf("unknown log-format %q", ctx.GlobalString("log-format"))
		}
		return nil
	}

	doHttpProbeFlag := cli.BoolFlag{
		Name:   "http-probe, p",
		Usage:  "enables HTTP probe",
		EnvVar: "DSLIM_HTTP_PROBE",
	}

	app.Commands = []cli.Command{
		{
			Name:    "info",
			Aliases: []string{"i"},
			Usage:   "Collects fat image information and reverse engineers its Dockerfile",
			Action: func(ctx *cli.Context) {
				if len(ctx.Args()) < 1 {
					fmt.Println("[info] missing image ID/name...\n")
					cli.ShowCommandHelp(ctx, "info")
					return
				}

				imageRef := ctx.Args().First()
				onInfoCommand(imageRef)
			},
		},
		{
			Name:    "build",
			Aliases: []string{"b"},
			Usage:   "Collects fat image information and builds a slim image from it",
			Flags: []cli.Flag{
				doHttpProbeFlag,
				cli.BoolFlag{
					Name:   "remove-file-artifacts, r",
					Usage:  "remove file artifacts when command is done",
					EnvVar: "DSLIM_RM_FILE_ARTIFACTS",
				},
			},
			Action: func(ctx *cli.Context) {
				if len(ctx.Args()) < 1 {
					fmt.Println("[build] missing image ID/name...\n")
					cli.ShowCommandHelp(ctx, "build")
					return
				}

				imageRef := ctx.Args().First()
				doHttpProbe := ctx.Bool("http-probe")
				doRmFileArtifacts := ctx.Bool("remove-file-artifacts")
				onBuildCommand(imageRef, doHttpProbe, doRmFileArtifacts)
			},
		},
		{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Collects fat image information and generates a fat container report",
			Flags: []cli.Flag{
				doHttpProbeFlag,
			},
			Action: func(ctx *cli.Context) {
				if len(ctx.Args()) < 1 {
					fmt.Println("[profile] missing image ID/name...\n")
					cli.ShowCommandHelp(ctx, "profile")
					return
				}

				imageRef := ctx.Args().First()
				onProfileCommand(imageRef)
			},
		},
	}
}

func runCli() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
