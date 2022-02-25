package app

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/build"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/containerize"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/convert"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/debug"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/dockerclipm"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/edit"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/help"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/install"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/lint"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/probe"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/profile"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/registry"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/run"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/server"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/update"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/version"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands/xray"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"
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
	registry.RegisterCommand()
	profile.RegisterCommand()
	version.RegisterCommand()
	help.RegisterCommand()
	update.RegisterCommand()
	install.RegisterCommand()
	edit.RegisterCommand()
	probe.RegisterCommand()
	convert.RegisterCommand()
	run.RegisterCommand()
	server.RegisterCommand()
	debug.RegisterCommand()
	containerize.RegisterCommand()
	dockerclipm.RegisterCommand()
}

func newCLI() *cli.App {
	registerCommands()

	cliApp := cli.NewApp()
	cliApp.Version = v.Current()
	cliApp.Name = AppName
	cliApp.Usage = AppUsage
	cliApp.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Printf("unknown command - %v \n\n", command)
		cli.ShowAppHelp(ctx)
	}

	cliApp.Flags = commands.GlobalFlags()

	cliApp.Before = func(ctx *cli.Context) error {
		gparams, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			log.Errorf("commands.GlobalFlagValues error - %v", err)
			return err
		}

		appParams, err := config.NewAppOptionsFromFile(fsutil.ResolveImageStateBasePath(gparams.StatePath))
		if err != nil {
			log.Errorf("config.NewAppOptionsFromFile error - %v", err)
			return err
		}

		gparams = commands.UpdateGlobalFlagValues(appParams, gparams)

		ctx.Context = commands.CLIContextSave(ctx.Context, commands.GlobalParams, gparams)
		ctx.Context = commands.CLIContextSave(ctx.Context, commands.AppParams, appParams)

		if gparams.NoColor {
			app.NoColor()
		}

		if gparams.Debug {
			log.SetLevel(log.DebugLevel)
		} else {
			if gparams.Verbose {
				log.SetLevel(log.InfoLevel)
			} else {
				logLevel := log.WarnLevel
				switch gparams.LogLevel {
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
					log.Fatalf("unknown log-level %q", gparams.LogLevel)
				}

				log.SetLevel(logLevel)
			}
		}

		if gparams.Log != "" {
			f, err := os.Create(gparams.Log)
			if err != nil {
				return err
			}
			log.SetOutput(f)
		}

		switch gparams.LogFormat {
		case "text":
			log.SetFormatter(&log.TextFormatter{DisableColors: true})
		case "json":
			log.SetFormatter(new(log.JSONFormatter))
		default:
			log.Fatalf("unknown log-format %q", gparams.LogFormat)
		}

		log.Debugf("sysinfo => %#v", system.GetSystemInfo())

		//tmp hack
		if !strings.Contains(strings.Join(os.Args, " "), " docker-cli-plugin-metadata") {
			app.ShowCommunityInfo()
		}
		return nil
	}

	cliApp.After = func(ctx *cli.Context) error {
		//tmp hack
		if !strings.Contains(strings.Join(os.Args, " "), " docker-cli-plugin-metadata") {
			app.ShowCommunityInfo()
		}
		return nil
	}

	cliApp.Action = func(ctx *cli.Context) error {
		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		ia := commands.NewInteractiveApp(cliApp, gcvalues)
		ia.Run()
		return nil
	}

	cliApp.Commands = commands.CLI
	return cliApp
}
