package app

import (
	"fmt"
	"os"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/build"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/containerize"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/convert"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/edit"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/help"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/lint"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/probe"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/profile"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/run"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/server"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/update"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/version"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/xray"
	"github.com/docker-slim/docker-slim/pkg/system"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// DockerSlim app CLI constants
const (
	AppName  = "docker-slim"
	AppUsage = "optimize and secure your Docker containers!"
)

func registerCommands() {
	//registering commands explicitly instead of relying on init()
	//also get to control the order of the commands in the interactive prompt

	xray.RegisterCommand()
	lint.RegisterCommand()
	build.RegisterCommand()
	profile.RegisterCommand()
	version.RegisterCommand()
	help.RegisterCommand()
	update.RegisterCommand()
	edit.RegisterCommand()
	probe.RegisterCommand()
	convert.RegisterCommand()
	run.RegisterCommand()
	server.RegisterCommand()
	containerize.RegisterCommand()
}

func newCLI() *cli.App {
	registerCommands()

	app := cli.NewApp()
	app.Version = v.Current()
	app.Name = AppName
	app.Usage = AppUsage
	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Printf("unknown command - %v \n\n", command)
		cli.ShowAppHelp(ctx)
	}

	app.Flags = commands.GlobalFlags()

	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool(commands.FlagNoColor) {
			commands.NoColor()
		}

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
	return app
}
