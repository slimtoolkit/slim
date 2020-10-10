package app

import (
	"fmt"
	"os"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// DockerSlim app CLI constants
const (
	AppName  = "docker-slim"
	AppUsage = "optimize and secure your Docker containers!"
)

var app *cli.App

func init() {
	app = cli.NewApp()
	app.Version = version.Current()
	app.Name = AppName
	app.Usage = AppUsage
	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Printf("unknown command - %v \n\n", command)
		cli.ShowAppHelp(ctx)
	}

	app.Flags = commands.GlobalFlags()

	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool(commands.FlagDebug) {
			log.SetLevel(log.DebugLevel)
		} else {
			if ctx.GlobalBool(commands.FlagVerbose) {
				log.SetLevel(log.InfoLevel)
			} else {
				logLevel := log.WarnLevel
				logLevelName := ctx.GlobalString(commands.FlagLogLevel)
				switch logLevelName {
				case "trace":
					logLevel = log.TraceLevel
				case "debug":
					logLevel = log.DebugLevel
				case "info":
					logLevel = log.InfoLevel
				case "warn":
					logLevel = log.WarnLevel
				case "error":
					logLevel = log.ErrorLevel
				case "fatal":
					logLevel = log.FatalLevel
				case "panic":
					logLevel = log.PanicLevel
				default:
					log.Fatalf("unknown log-level %q", logLevelName)
				}

				log.SetLevel(logLevel)
			}
		}

		if path := ctx.GlobalString(commands.FlagLog); path != "" {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			log.SetOutput(f)
		}

		logFormat := ctx.GlobalString(commands.FlagLogFormat)
		switch logFormat {
		case "text":
			log.SetFormatter(&log.TextFormatter{DisableColors: true})
		case "json":
			log.SetFormatter(new(log.JSONFormatter))
		default:
			log.Fatalf("unknown log-format %q", logFormat)
		}

		log.Debugf("sysinfo => %#v", system.GetSystemInfo())

		return nil
	}

	app.Action = func(ctx *cli.Context) error {
		gcvalues, err := commands.GlobalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		ia := commands.NewInteractiveApp(app, gcvalues)
		ia.Run()
		return nil
	}

	app.Commands = commands.CLI
}

func runCli() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
