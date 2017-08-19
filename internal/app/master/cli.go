package app

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
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
		cli.BoolTFlag{
			Name:  "tls",
			Usage: "use TLS",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "verify TLS",
		},
		cli.StringFlag{
			Name:  "tls-cert-path",
			Value: "",
			Usage: "path to TLS cert files",
		},
		cli.StringFlag{
			Name:  "host",
			Value: "",
			Usage: "Docker host address",
		},
		cli.StringFlag{
			Name:  "state-path",
			Value: "",
			Usage: "DockerSlim state base path",
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

	doHTTPProbeFlag := cli.BoolFlag{
		Name:   "http-probe, p",
		Usage:  "Enables HTTP probe",
		EnvVar: "DSLIM_HTTP_PROBE",
	}

	doHTTPProbeCmdFlag := cli.StringSliceFlag{
		Name:   "http-probe-cmd",
		Value:  &cli.StringSlice{},
		Usage:  "User defined HTTP probes",
		EnvVar: "DSLIM_HTTP_PROBE_CMD",
	}

	doHTTPProbeCmdFileFlag := cli.StringFlag{
		Name:   "http-probe-cmd-file",
		Value:  "",
		Usage:  "File with user defined HTTP probes",
		EnvVar: "DSLIM_HTTP_PROBE_CMD_FILE",
	}

	doShowContainerLogsFlag := cli.BoolFlag{
		Name:   "show-clogs",
		Usage:  "Show container logs",
		EnvVar: "DSLIM_SHOW_CLOGS",
	}

	doUseEntrypointFlag := cli.StringFlag{
		Name:   "entrypoint",
		Value:  "",
		Usage:  "Override ENTRYPOINT analyzing image",
		EnvVar: "DSLIM_ENTRYPOINT",
	}

	doUseCmdFlag := cli.StringFlag{
		Name:   "cmd",
		Value:  "",
		Usage:  "Override CMD analyzing image",
		EnvVar: "DSLIM_TARGET_CMD",
	}

	doUseWorkdirFlag := cli.StringFlag{
		Name:   "workdir",
		Value:  "",
		Usage:  "Override WORKDIR analyzing image",
		EnvVar: "DSLIM_TARGET_WORKDIR",
	}

	doUseEnvFlag := cli.StringSliceFlag{
		Name:   "env",
		Value:  &cli.StringSlice{},
		Usage:  "Override ENV analyzing image",
		EnvVar: "DSLIM_TARGET_ENV",
	}

	doUseExposeFlag := cli.StringSliceFlag{
		Name:   "expose",
		Value:  &cli.StringSlice{},
		Usage:  "Use additional EXPOSE instructions analyzing image",
		EnvVar: "DSLIM_TARGET_EXPOSE",
	}

	doExcludeMountsFlag := cli.BoolTFlag{
		Name:   "exclude-mounts",
		Usage:  "Exclude mounted volumes from image",
		EnvVar: "DSLIM_EXCLUDE_MOUNTS",
	}

	doExcludePathFlag := cli.StringSliceFlag{
		Name:   "exclude-path",
		Value:  &cli.StringSlice{},
		Usage:  "Exclude path from image",
		EnvVar: "DSLIM_EXCLUDE_PATH",
	}

	doIncludePathFlag := cli.StringSliceFlag{
		Name:   "include-path",
		Value:  &cli.StringSlice{},
		Usage:  "Include path from image",
		EnvVar: "DSLIM_INCLUDE_PATH",
	}

	doUseMountFlag := cli.StringSliceFlag{
		Name:   "mount",
		Value:  &cli.StringSlice{},
		Usage:  "Mount volume analyzing image",
		EnvVar: "DSLIM_MOUNT",
	}

	doConfinueAfterFlag := cli.StringFlag{
		Name:   "continue-after",
		Value:  "enter",
		Usage:  "Select continue mode: enter | signal | probe | timeout or numberInSeconds",
		EnvVar: "DSLIM_CONTINUE_AFTER",
	}

	app.Commands = []cli.Command{
		{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "Shows docker-slim and docker version information",
			Action: func(ctx *cli.Context) error {
				clientConfig := getDockerClientConfig(ctx)
				commands.OnVersion(clientConfig)
				return nil
			},
		},
		{
			Name:    "info",
			Aliases: []string{"i"},
			Usage:   "Collects fat image information and reverse engineers its Dockerfile",
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("[info] missing image ID/name...\n\n")
					cli.ShowCommandHelp(ctx, "info")
					return nil
				}

				statePath := ctx.GlobalString("state-path")

				imageRef := ctx.Args().First()
				clientConfig := getDockerClientConfig(ctx)

				commands.OnInfo(statePath, clientConfig, imageRef)
				return nil
			},
		},
		{
			Name:    "build",
			Aliases: []string{"b"},
			Usage:   "Collects fat image information and builds a slim image from it",
			Flags: []cli.Flag{
				doHTTPProbeFlag,
				doHTTPProbeCmdFlag,
				doHTTPProbeCmdFileFlag,
				doShowContainerLogsFlag,
				cli.BoolFlag{
					Name:   "remove-file-artifacts, r",
					Usage:  "remove file artifacts when command is done",
					EnvVar: "DSLIM_RM_FILE_ARTIFACTS",
				},
				cli.StringFlag{
					Name:   "tag",
					Value:  "",
					Usage:  "Custom tag for the generated image",
					EnvVar: "DSLIM_TARGET_TAG",
				},
				cli.StringFlag{
					Name:   "image-overrides",
					Value:  "",
					Usage:  "Use overrides in generated image",
					EnvVar: "DSLIM_TARGET_OVERRIDES",
				},
				doUseEntrypointFlag,
				doUseCmdFlag,
				doUseWorkdirFlag,
				doUseEnvFlag,
				doUseExposeFlag,
				doExcludeMountsFlag,
				doExcludePathFlag,
				doIncludePathFlag,
				doUseMountFlag,
				doConfinueAfterFlag,
			},
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("[build] missing image ID/name...\n\n")
					cli.ShowCommandHelp(ctx, "build")
					return nil
				}

				statePath := ctx.GlobalString("state-path")

				imageRef := ctx.Args().First()
				clientConfig := getDockerClientConfig(ctx)
				doRmFileArtifacts := ctx.Bool("remove-file-artifacts")

				doHTTPProbe := ctx.Bool("http-probe")

				httpProbeCmds, err := getHTTPProbes(ctx)
				if err != nil {
					fmt.Printf("[build] invalid HTTP probes: %v\n", err)
					return err
				}

				if len(httpProbeCmds) > 0 {
					doHTTPProbe = true
				}

				doShowContainerLogs := ctx.Bool("show-clogs")
				doTag := ctx.String("tag")

				doImageOverrides := ctx.String("image-overrides")
				overrides, err := getContainerOverrides(ctx)
				if err != nil {
					fmt.Printf("[build] invalid container overrides: %v\n", err)
					return err
				}

				volumeMounts, err := parseVolumeMounts(ctx.StringSlice("mount"))
				if err != nil {
					fmt.Printf("[build] invalid volume mounts: %v\n", err)
					return err
				}

				excludePaths := parsePaths(ctx.StringSlice("exclude-path"))
				includePaths := parsePaths(ctx.StringSlice("include-path"))

				doExcludeMounts := ctx.BoolT("exclude-mounts")
				if doExcludeMounts {
					for mpath := range volumeMounts {
						excludePaths[mpath] = true
					}
				}

				confinueAfter, err := getContinueAfter(ctx)
				if err != nil {
					fmt.Printf("[build] invalid continue-after mode: %v\n", err)
					return err
				}

				for ipath := range includePaths {
					if excludePaths[ipath] {
						fmt.Printf("[build] include and exclude path conflict: %v\n", err)
						return nil
					}
				}

				commands.OnBuild(ctx.GlobalBool("debug"),
					statePath,
					clientConfig,
					imageRef, doTag,
					doHTTPProbe, httpProbeCmds,
					doRmFileArtifacts, doShowContainerLogs,
					parseImageOverrides(doImageOverrides),
					overrides,
					volumeMounts, excludePaths, includePaths,
					confinueAfter)

				return nil
			},
		},
		{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Collects fat image information and generates a fat container report",
			Flags: []cli.Flag{
				doHTTPProbeFlag,
				doHTTPProbeCmdFlag,
				doHTTPProbeCmdFileFlag,
				doShowContainerLogsFlag,
				doUseEntrypointFlag,
				doUseCmdFlag,
				doUseWorkdirFlag,
				doUseEnvFlag,
				doUseExposeFlag,
				doExcludeMountsFlag,
				doExcludePathFlag,
				doIncludePathFlag,
				doUseMountFlag,
				doConfinueAfterFlag,
			},
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("[profile] missing image ID/name...\n\n")
					cli.ShowCommandHelp(ctx, "profile")
					return nil
				}

				statePath := ctx.GlobalString("state-path")

				imageRef := ctx.Args().First()
				clientConfig := getDockerClientConfig(ctx)
				doHTTPProbe := ctx.Bool("http-probe")

				httpProbeCmds, err := getHTTPProbes(ctx)
				if err != nil {
					fmt.Printf("[profile] invalid HTTP probes: %v\n", err)
					return err
				}

				if len(httpProbeCmds) > 0 {
					doHTTPProbe = true
				}

				doShowContainerLogs := ctx.Bool("show-clogs")
				overrides, err := getContainerOverrides(ctx)
				if err != nil {
					fmt.Printf("[profile] invalid container overrides: %v", err)
					return err
				}

				volumeMounts, err := parseVolumeMounts(ctx.StringSlice("mount"))
				if err != nil {
					fmt.Printf("[profile] invalid volume mounts: %v\n", err)
					return err
				}

				excludePaths := parsePaths(ctx.StringSlice("exclude-path"))
				includePaths := parsePaths(ctx.StringSlice("include-path"))

				doExcludeMounts := ctx.Bool("exclude-mounts")
				if doExcludeMounts {
					for mpath := range volumeMounts {
						excludePaths[mpath] = true
					}
				}

				confinueAfter, err := getContinueAfter(ctx)
				if err != nil {
					fmt.Printf("[profile] invalid continue-after mode: %v\n", err)
					return err
				}

				for ipath := range includePaths {
					if excludePaths[ipath] {
						fmt.Printf("[profile] include and exclude path conflict: %v\n", err)
						return nil
					}
				}

				commands.OnProfile(ctx.GlobalBool("debug"),
					statePath,
					clientConfig,
					imageRef,
					doHTTPProbe, httpProbeCmds,
					doShowContainerLogs, overrides,
					volumeMounts, excludePaths, includePaths,
					confinueAfter)

				return nil
			},
		},
	}
}

func getContinueAfter(ctx *cli.Context) (*config.ContinueAfter, error) {
	info := &config.ContinueAfter{
		Mode: "enter",
	}

	doConfinueAfter := ctx.String("continue-after")
	switch doConfinueAfter {
	case "enter":
		info.Mode = "enter"
	case "signal":
		info.Mode = "signal"
		info.ContinueChan = appContinueChan
	case "probe":
		info.Mode = "probe"
	case "timeout":
		info.Mode = "timeout"
		info.Timeout = 60
	default:
		if waitTime, err := strconv.Atoi(doConfinueAfter); err == nil && waitTime > 0 {
			info.Mode = "timeout"
			info.Timeout = time.Duration(waitTime)
		}
	}

	return info, nil
}

func getContainerOverrides(ctx *cli.Context) (*config.ContainerOverrides, error) {
	doUseEntrypoint := ctx.String("entrypoint")
	doUseCmd := ctx.String("cmd")
	doUseExpose := ctx.StringSlice("expose")

	overrides := &config.ContainerOverrides{
		Workdir: ctx.String("workdir"),
		Env:     ctx.StringSlice("env"),
	}

	var err error
	if len(doUseExpose) > 0 {
		overrides.ExposedPorts, err = parseDockerExposeOpt(doUseExpose)
		if err != nil {
			fmt.Printf("invalid expose options..\n\n")
			return nil, err
		}
	}

	overrides.Entrypoint, err = parseExec(doUseEntrypoint)
	if err != nil {
		fmt.Printf("invalid entrypoint option..\n\n")
		return nil, err
	}

	overrides.ClearEntrypoint = isOneSpace(doUseEntrypoint)

	overrides.Cmd, err = parseExec(doUseCmd)
	if err != nil {
		fmt.Printf("invalid cmd option..\n\n")
		return nil, err
	}

	overrides.ClearCmd = isOneSpace(doUseCmd)

	return overrides, nil
}

func getHTTPProbes(ctx *cli.Context) ([]config.HTTPProbeCmd, error) {
	httpProbeCmds, err := parseHTTPProbes(ctx.StringSlice("http-probe-cmd"))
	if err != nil {
		return nil, err
	}

	moreHTTPProbeCmds, err := parseHTTPProbesFile(ctx.String("http-probe-cmd-file"))
	if err != nil {
		return nil, err
	}

	if moreHTTPProbeCmds != nil {
		httpProbeCmds = append(httpProbeCmds, moreHTTPProbeCmds...)
	}

	return httpProbeCmds, nil
}

func getDockerClientConfig(ctx *cli.Context) *config.DockerClient {
	config := &config.DockerClient{
		UseTLS:      ctx.GlobalBool("tls"),
		VerifyTLS:   ctx.GlobalBool("tls-verify"),
		TLSCertPath: ctx.GlobalString("tls-cert-path"),
		Host:        ctx.GlobalString("host"),
		Env:         map[string]string{},
	}

	getEnv := func(name string) {
		if value, exists := os.LookupEnv(name); exists {
			config.Env[name] = value
		}
	}

	getEnv("DOCKER_HOST")
	getEnv("DOCKER_TLS_VERIFY")
	getEnv("DOCKER_CERT_PATH")

	return config
}

func runCli() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
