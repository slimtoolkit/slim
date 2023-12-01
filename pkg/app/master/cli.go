package app

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/commands"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/appbom"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/build"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/containerize"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/convert"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/debug"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/dockerclipm"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/edit"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/help"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/install"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/lint"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/merge"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/probe"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/profile"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/registry"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/run"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/server"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/update"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/version"
	"github.com/slimtoolkit/slim/pkg/app/master/commands/xray"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/system"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

// Main/driver app CLI constants
const (
	AppName  = "slim"
	AppUsage = "inspect, optimize and debug your containers!"
)

func registerCommands() {
	//registering commands explicitly instead of relying on init()
	//also get to control the order of the commands in the interactive prompt

	xray.RegisterCommand()
	lint.RegisterCommand()
	build.RegisterCommand()
	merge.RegisterCommand()
	registry.RegisterCommand()
	profile.RegisterCommand()
	version.RegisterCommand()
	appbom.RegisterCommand()
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

	doShowCommunityInfo := true
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

		//NOTE: not displaying the community info here to reduce noise
		//tmp hack
		//if !strings.Contains(strings.Join(os.Args, " "), " docker-cli-plugin-metadata") {
		//   app.ShowCommunityInfo(gparams.ConsoleOutput)
		//}
		return nil
	}

	cliApp.After = func(ctx *cli.Context) error {
		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}
		if gcvalues.QuietCLIMode {
			return nil
		}

		//tmp hack
		if !strings.Contains(strings.Join(os.Args, " "), " docker-cli-plugin-metadata") {
			if doShowCommunityInfo {
				app.ShowCommunityInfo(ctx.String(commands.FlagConsoleFormat))
			}
		}
		return nil
	}

	cliApp.Action = func(ctx *cli.Context) error {
		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			return err
		}

		//disable community info in interactive mode (too noisy)
		doShowCommunityInfo = false
		ia := commands.NewInteractiveApp(cliApp, gcvalues)
		ia.Run()
		return nil
	}

	cliApp.Commands = commands.Get()
	return cliApp
}
