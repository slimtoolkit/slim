package app

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

// DockerSlim app CLI constants
const (
	AppName  = "docker-slim"
	AppUsage = "optimize and secure your Docker containers!"
)

// DockerSlim app command names
const (
	CmdVersion = "version"
	CmdUpdate  = "update"
	CmdInfo    = "info"
	CmdBuild   = "build"
	CmdProfile = "profile"
)

// DockerSlim app flag names
const (
	FlagCheckVersion        = "check-version"
	FlagDebug               = "debug"
	FlagCommandReport       = "report"
	FlagVerbose             = "verbose"
	FlagLogLevel            = "log-level"
	FlagLog                 = "log"
	FlagLogFormat           = "log-format"
	FlagUseTLS              = "tls"
	FlagVerifyTLS           = "tls-verify"
	FlagTLSCertPath         = "tls-cert-path"
	FlagHost                = "host"
	FlagStatePath           = "state-path"
	FlagHTTPProbe           = "http-probe"
	FlagHTTPProbeCmd        = "http-probe-cmd"
	FlagHTTPProbeCmdFile    = "http-probe-cmd-file"
	FlagHTTPProbeRetryCount = "http-probe-retry-count"
	FlagHTTPProbeRetryWait  = "http-probe-retry-wait"
	FlagHTTPProbePorts      = "http-probe-ports"
	FlagHTTPProbeFull       = "http-probe-full"
	FlagShowContainerLogs   = "show-clogs"
	FlagShowBuildLogs       = "show-blogs"
	FlagEntrypoint          = "entrypoint"
	FlagCmd                 = "cmd"
	FlagWorkdir             = "workdir"
	FlagEnv                 = "env"
	FlagExpose              = "expose"
	FlagNewEntrypoint       = "new-entrypoint"
	FlagNewCmd              = "new-cmd"
	FlagNewExpose           = "new-expose"
	FlagNewWorkdir          = "new-workdir"
	FlagNewEnv              = "new-env"
	FlagExludeMounts        = "exclude-mounts"
	FlagExcludePath         = "exclude-path"
	FlagIncludePath         = "include-path"
	FlagIncludePathFile     = "include-path-file"
	FlagIncludeBin          = "include-bin"
	FlagIncludeExe          = "include-exe"
	FlagIncludeShell        = "include-shell"
	FlagMount               = "mount"
	FlagContinueAfter       = "continue-after"
	FlagNetwork             = "network"
	FlagLink                = "link"
	FlagHostname            = "hostname"
	FlagEtcHostsMap         = "etc-hosts-map"
	FlagContainerDNS        = "container-dns"
	FlagContainerDNSSearch  = "container-dns-search"
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
		cli.StringFlag{
			Name:  FlagCommandReport,
			Usage: "command report location",
		},
		cli.BoolTFlag{
			Name:   FlagCheckVersion,
			Usage:  "check if the current version is outdated",
			EnvVar: "DSLIM_CHECK_VERSION",
		},
		cli.BoolFlag{
			Name:  FlagDebug,
			Usage: "enable debug logs",
		},
		cli.BoolFlag{
			Name:  FlagVerbose,
			Usage: "enable info logs",
		},
		cli.StringFlag{
			Name:  FlagLogLevel,
			Value: "warn",
			Usage: "set the logging level ('debug', 'info', 'warn' (default), 'error', 'fatal', 'panic')",
		},
		cli.StringFlag{
			Name:  FlagLog,
			Usage: "log file to store logs",
		},
		cli.StringFlag{
			Name:  FlagLogFormat,
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
		cli.BoolTFlag{
			Name:  FlagUseTLS,
			Usage: "use TLS",
		},
		cli.BoolTFlag{
			Name:  FlagVerifyTLS,
			Usage: "verify TLS",
		},
		cli.StringFlag{
			Name:  FlagTLSCertPath,
			Value: "",
			Usage: "path to TLS cert files",
		},
		cli.StringFlag{
			Name:  FlagHost,
			Value: "",
			Usage: "Docker host address",
		},
		cli.StringFlag{
			Name:  FlagStatePath,
			Value: "",
			Usage: "DockerSlim state base path",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool(FlagDebug) {
			log.SetLevel(log.DebugLevel)
		} else {
			if ctx.GlobalBool(FlagVerbose) {
				log.SetLevel(log.InfoLevel)
			} else {
				logLevel := log.WarnLevel
				logLevelName := ctx.GlobalString(FlagLogLevel)
				switch logLevelName {
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

		if path := ctx.GlobalString(FlagLog); path != "" {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			log.SetOutput(f)
		}

		logFormat := ctx.GlobalString(FlagLogFormat)
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

	//true by default
	doHTTPProbeFlag := cli.BoolTFlag{
		Name:   FlagHTTPProbe,
		Usage:  "Enables HTTP probe",
		EnvVar: "DSLIM_HTTP_PROBE",
	}

	doHTTPProbeCmdFlag := cli.StringSliceFlag{
		Name:   FlagHTTPProbeCmd,
		Value:  &cli.StringSlice{},
		Usage:  "User defined HTTP probes",
		EnvVar: "DSLIM_HTTP_PROBE_CMD",
	}

	doHTTPProbeCmdFileFlag := cli.StringFlag{
		Name:   FlagHTTPProbeCmdFile,
		Value:  "",
		Usage:  "File with user defined HTTP probes",
		EnvVar: "DSLIM_HTTP_PROBE_CMD_FILE",
	}

	doHTTPProbeRetryCountFlag := cli.IntFlag{
		Name:   FlagHTTPProbeRetryCount,
		Value:  5,
		Usage:  "Number of retries for each HTTP probe",
		EnvVar: "DSLIM_HTTP_PROBE_RETRY_COUNT",
	}

	doHTTPProbeRetryWaitFlag := cli.IntFlag{
		Name:   FlagHTTPProbeRetryWait,
		Value:  8,
		Usage:  "Number of seconds to wait before retrying HTTP probe (doubles when target is not ready)",
		EnvVar: "DSLIM_HTTP_PROBE_RETRY_WAIT",
	}

	doHTTPProbePortsFlag := cli.StringFlag{
		Name:   FlagHTTPProbePorts,
		Value:  "",
		Usage:  "Explicit list of ports to probe (in the order you want them to be probed)",
		EnvVar: "DSLIM_HTTP_PROBE_PORTS",
	}

	doHTTPProbeFullFlag := cli.BoolFlag{
		Name:   FlagHTTPProbeFull,
		Usage:  "Do full HTTP probe for all selected ports (if false, finish after first successful scan)",
		EnvVar: "DSLIM_HTTP_PROBE_FULL",
	}

	doShowContainerLogsFlag := cli.BoolFlag{
		Name:   FlagShowContainerLogs,
		Usage:  "Show container logs",
		EnvVar: "DSLIM_SHOW_CLOGS",
	}

	doShowBuildLogsFlag := cli.BoolFlag{
		Name:   FlagShowBuildLogs,
		Usage:  "Show build logs",
		EnvVar: "DSLIM_SHOW_BLOGS",
	}

	doUseNewEntrypointFlag := cli.StringFlag{
		Name:   FlagNewEntrypoint,
		Value:  "",
		Usage:  "New ENTRYPOINT instruction for the minified image",
		EnvVar: "DSLIM_NEW_ENTRYPOINT",
	}

	doUseNewCmdFlag := cli.StringFlag{
		Name:   FlagNewCmd,
		Value:  "",
		Usage:  "New CMD instruction for the minified image",
		EnvVar: "DSLIM_NEW_CMD",
	}

	doUseNewExposeFlag := cli.StringSliceFlag{
		Name:   FlagNewExpose,
		Value:  &cli.StringSlice{},
		Usage:  "New EXPOSE instructions for the minified image",
		EnvVar: "DSLIM_NEW_EXPOSE",
	}

	doUseNewWorkdirFlag := cli.StringFlag{
		Name:   FlagNewWorkdir,
		Value:  "",
		Usage:  "New WORKDIR instruction for the minified image",
		EnvVar: "DSLIM_NEW_WORKDIR",
	}

	doUseNewEnvFlag := cli.StringSliceFlag{
		Name:   FlagNewEnv,
		Value:  &cli.StringSlice{},
		Usage:  "New ENV instructions for the minified image",
		EnvVar: "DSLIM_NEW_ENV",
	}

	doUseEntrypointFlag := cli.StringFlag{
		Name:   FlagEntrypoint,
		Value:  "",
		Usage:  "Override ENTRYPOINT analyzing image",
		EnvVar: "DSLIM_ENTRYPOINT",
	}

	doUseCmdFlag := cli.StringFlag{
		Name:   FlagCmd,
		Value:  "",
		Usage:  "Override CMD analyzing image",
		EnvVar: "DSLIM_TARGET_CMD",
	}

	doUseWorkdirFlag := cli.StringFlag{
		Name:   FlagWorkdir,
		Value:  "",
		Usage:  "Override WORKDIR analyzing image",
		EnvVar: "DSLIM_TARGET_WORKDIR",
	}

	doUseEnvFlag := cli.StringSliceFlag{
		Name:   FlagEnv,
		Value:  &cli.StringSlice{},
		Usage:  "Override ENV analyzing image",
		EnvVar: "DSLIM_TARGET_ENV",
	}

	doUseLinkFlag := cli.StringSliceFlag{
		Name:   FlagLink,
		Value:  &cli.StringSlice{},
		Usage:  "Add link to another container analyzing image",
		EnvVar: "DSLIM_TARGET_LINK",
	}

	doUseEtcHostsMapFlag := cli.StringSliceFlag{
		Name:   FlagEtcHostsMap,
		Value:  &cli.StringSlice{},
		Usage:  "Add a host to IP mapping to /etc/hosts analyzing image",
		EnvVar: "DSLIM_TARGET_ETC_HOSTS_MAP",
	}

	doUseContainerDNSFlag := cli.StringSliceFlag{
		Name:   FlagContainerDNS,
		Value:  &cli.StringSlice{},
		Usage:  "Add a dns server analyzing image",
		EnvVar: "DSLIM_TARGET_DNS",
	}

	doUseContainerDNSSearchFlag := cli.StringSliceFlag{
		Name:   FlagContainerDNSSearch,
		Value:  &cli.StringSlice{},
		Usage:  "Add a dns search domain for unqualified hostnames analyzing image",
		EnvVar: "DSLIM_TARGET_DNS_SEARCH",
	}

	doUseHostnameFlag := cli.StringFlag{
		Name:   FlagHostname,
		Value:  "",
		Usage:  "Override default container hostname analyzing image",
		EnvVar: "DSLIM_TARGET_HOSTNAME",
	}

	doUseNetworkFlag := cli.StringFlag{
		Name:   FlagNetwork,
		Value:  "",
		Usage:  "Override default container network settings analyzing image",
		EnvVar: "DSLIM_TARGET_NET",
	}

	doUseExposeFlag := cli.StringSliceFlag{
		Name:   FlagExpose,
		Value:  &cli.StringSlice{},
		Usage:  "Use additional EXPOSE instructions analyzing image",
		EnvVar: "DSLIM_TARGET_EXPOSE",
	}

	//true by default
	doExcludeMountsFlag := cli.BoolTFlag{
		Name:   FlagExludeMounts,
		Usage:  "Exclude mounted volumes from image",
		EnvVar: "DSLIM_EXCLUDE_MOUNTS",
	}

	doExcludePathFlag := cli.StringSliceFlag{
		Name:   FlagExcludePath,
		Value:  &cli.StringSlice{},
		Usage:  "Exclude path from image",
		EnvVar: "DSLIM_EXCLUDE_PATH",
	}

	doIncludePathFlag := cli.StringSliceFlag{
		Name:   FlagIncludePath,
		Value:  &cli.StringSlice{},
		Usage:  "Include path from image",
		EnvVar: "DSLIM_INCLUDE_PATH",
	}

	doIncludePathFileFlag := cli.StringFlag{
		Name:   FlagIncludePathFile,
		Value:  "",
		Usage:  "File with paths to include from image",
		EnvVar: "DSLIM_INCLUDE_PATH_FILE",
	}

	doIncludeBinFlag := cli.StringSliceFlag{
		Name:   FlagIncludeBin,
		Value:  &cli.StringSlice{},
		Usage:  "Include binary from image (executable or shared object using its absolute path)",
		EnvVar: "DSLIM_INCLUDE_BIN",
	}

	doIncludeExeFlag := cli.StringSliceFlag{
		Name:   FlagIncludeExe,
		Value:  &cli.StringSlice{},
		Usage:  "Include executable from image (by executable name)",
		EnvVar: "DSLIM_INCLUDE_EXE",
	}

	doIncludeShellFlag := cli.BoolFlag{
		Name:   FlagIncludeShell,
		Usage:  "Include basic shell functionality",
		EnvVar: "DSLIM_INCLUDE_SHELL",
	}

	doUseMountFlag := cli.StringSliceFlag{
		Name:   FlagMount,
		Value:  &cli.StringSlice{},
		Usage:  "Mount volume analyzing image",
		EnvVar: "DSLIM_MOUNT",
	}

	doConfinueAfterFlag := cli.StringFlag{
		Name:   FlagContinueAfter,
		Value:  "enter",
		Usage:  "Select continue mode: enter | signal | probe | timeout or numberInSeconds",
		EnvVar: "DSLIM_CONTINUE_AFTER",
	}

	//enable 'show-progress' by default only on Mac OS X
	var doShowProgressFlag cli.Flag
	switch runtime.GOOS {
	case "darwin":
		doShowProgressFlag = cli.BoolTFlag{
			Name:   "show-progress",
			Usage:  "show progress when the release package is downloaded (default: true)",
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	default:
		doShowProgressFlag = cli.BoolFlag{
			Name:   "show-progress",
			Usage:  "show progress when the release package is downloaded (default: false)",
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	}

	app.Commands = []cli.Command{
		{
			Name:    CmdVersion,
			Aliases: []string{"v"},
			Usage:   "Shows docker-slim and docker version information",
			Action: func(ctx *cli.Context) error {
				clientConfig := getDockerClientConfig(ctx)
				commands.OnVersion(clientConfig)
				return nil
			},
		},
		{
			Name:    CmdUpdate,
			Aliases: []string{"u"},
			Usage:   "Update docker-slim",
			Flags: []cli.Flag{
				doShowProgressFlag,
			},
			Action: func(ctx *cli.Context) error {
				doDebug := ctx.GlobalBool(FlagDebug)
				statePath := ctx.GlobalString(FlagStatePath)
				doShowProgress := ctx.Bool("show-progress")

				commands.OnUpdate(doDebug, statePath, doShowProgress)
				return nil
			},
		},
		{
			Name:    CmdInfo,
			Aliases: []string{"i"},
			Usage:   "Collects fat image information and reverse engineers its Dockerfile",
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("[info] missing image ID/name...\n\n")
					cli.ShowCommandHelp(ctx, CmdInfo)
					return nil
				}

				doCheckVersion := ctx.GlobalBool(FlagCheckVersion)

				statePath := ctx.GlobalString(FlagStatePath)

				imageRef := ctx.Args().First()
				clientConfig := getDockerClientConfig(ctx)

				commands.OnInfo(
					doCheckVersion,
					ctx.GlobalString(FlagCommandReport),
					ctx.GlobalBool(FlagDebug),
					statePath,
					clientConfig,
					imageRef)
				return nil
			},
		},
		{
			Name:    CmdBuild,
			Aliases: []string{"b"},
			Usage:   "Collects fat image information and builds a slim image from it",
			Flags: []cli.Flag{
				doHTTPProbeFlag,
				doHTTPProbeCmdFlag,
				doHTTPProbeCmdFileFlag,
				doHTTPProbeRetryCountFlag,
				doHTTPProbeRetryWaitFlag,
				doHTTPProbePortsFlag,
				doHTTPProbeFullFlag,
				doShowContainerLogsFlag,
				doShowBuildLogsFlag,
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
				doUseLinkFlag,
				doUseEtcHostsMapFlag,
				doUseContainerDNSFlag,
				doUseContainerDNSSearchFlag,
				doUseNetworkFlag,
				doUseHostnameFlag,
				doUseExposeFlag,
				doUseNewEntrypointFlag,
				doUseNewCmdFlag,
				doUseNewExposeFlag,
				doUseNewWorkdirFlag,
				doUseNewEnvFlag,
				doExcludeMountsFlag,
				doExcludePathFlag,
				doIncludePathFlag,
				doIncludePathFileFlag,
				doIncludeBinFlag,
				doIncludeExeFlag,
				doIncludeShellFlag,
				doUseMountFlag,
				doConfinueAfterFlag,
			},
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("[build] missing image ID/name...\n\n")
					cli.ShowCommandHelp(ctx, CmdBuild)
					return nil
				}

				doCheckVersion := ctx.GlobalBool(FlagCheckVersion)

				statePath := ctx.GlobalString(FlagStatePath)

				imageRef := ctx.Args().First()
				clientConfig := getDockerClientConfig(ctx)
				doRmFileArtifacts := ctx.Bool("remove-file-artifacts")

				doHTTPProbe := ctx.Bool(FlagHTTPProbe)

				httpProbeCmds, err := getHTTPProbes(ctx)
				if err != nil {
					fmt.Printf("[build] invalid HTTP probes: %v\n", err)
					return err
				}

				if doHTTPProbe {
					//add default probe cmd if the "http-probe" flag is set
					fmt.Println("docker-slim[build]: info=http.probe message='using default probe'")
					httpProbeCmds = append(httpProbeCmds,
						config.HTTPProbeCmd{Protocol: "http", Method: "GET", Resource: "/"})
				}

				if len(httpProbeCmds) > 0 {
					doHTTPProbe = true
				}

				httpProbeRetryCount := ctx.Int(FlagHTTPProbeRetryCount)
				httpProbeRetryWait := ctx.Int(FlagHTTPProbeRetryWait)
				httpProbePorts, err := parseHTTPProbesPorts(ctx.String(FlagHTTPProbePorts))
				if err != nil {
					fmt.Printf("[build] invalid HTTP Probe target ports: %v\n", err)
					return err
				}

				doHTTPProbeFull := ctx.Bool(FlagHTTPProbeFull)

				doShowContainerLogs := ctx.Bool(FlagShowContainerLogs)
				doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
				doTag := ctx.String("tag")

				doImageOverrides := ctx.String("image-overrides")
				overrides, err := getContainerOverrides(ctx)
				if err != nil {
					fmt.Printf("[build] invalid container overrides: %v\n", err)
					return err
				}

				instructions, err := getImageInstructions(ctx)
				if err != nil {
					fmt.Printf("[build] invalid image instructions: %v\n", err)
					return err
				}

				volumeMounts, err := parseVolumeMounts(ctx.StringSlice(FlagMount))
				if err != nil {
					fmt.Printf("[build] invalid volume mounts: %v\n", err)
					return err
				}

				excludePaths := parsePaths(ctx.StringSlice(FlagExcludePath))

				includePaths := parsePaths(ctx.StringSlice(FlagIncludePath))
				moreIncludePaths, err := parsePathsFile(ctx.String(FlagIncludePathFile))
				if err != nil {
					fmt.Printf("[build] could not read include path file (ignoring): %v\n", err)
				} else {
					for k, v := range moreIncludePaths {
						includePaths[k] = v
					}
				}

				includeBins := parsePaths(ctx.StringSlice(FlagIncludeBin))
				includeExes := parsePaths(ctx.StringSlice(FlagIncludeExe))
				doIncludeShell := ctx.Bool(FlagIncludeShell)

				doExcludeMounts := ctx.BoolT(FlagExludeMounts)
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

				commands.OnBuild(
					doCheckVersion,
					ctx.GlobalString(FlagCommandReport),
					ctx.GlobalBool(FlagDebug),
					statePath,
					clientConfig,
					imageRef,
					doTag,
					doHTTPProbe,
					httpProbeCmds,
					httpProbeRetryCount,
					httpProbeRetryWait,
					httpProbePorts,
					doHTTPProbeFull,
					doRmFileArtifacts,
					doShowContainerLogs,
					doShowBuildLogs,
					parseImageOverrides(doImageOverrides),
					overrides,
					instructions,
					ctx.StringSlice(FlagLink),
					ctx.StringSlice(FlagEtcHostsMap),
					ctx.StringSlice(FlagContainerDNS),
					ctx.StringSlice(FlagContainerDNSSearch),
					volumeMounts,
					excludePaths,
					includePaths,
					includeBins,
					includeExes,
					doIncludeShell,
					confinueAfter)

				return nil
			},
		},
		{
			Name:    CmdProfile,
			Aliases: []string{"p"},
			Usage:   "Collects fat image information and generates a fat container report",
			Flags: []cli.Flag{
				doHTTPProbeFlag,
				doHTTPProbeCmdFlag,
				doHTTPProbeCmdFileFlag,
				doHTTPProbeRetryCountFlag,
				doHTTPProbeRetryWaitFlag,
				doHTTPProbePortsFlag,
				doHTTPProbeFullFlag,
				doShowContainerLogsFlag,
				doUseEntrypointFlag,
				doUseCmdFlag,
				doUseWorkdirFlag,
				doUseEnvFlag,
				doUseLinkFlag,
				doUseEtcHostsMapFlag,
				doUseContainerDNSFlag,
				doUseContainerDNSSearchFlag,
				doUseNetworkFlag,
				doUseHostnameFlag,
				doUseExposeFlag,
				doExcludeMountsFlag,
				doExcludePathFlag,
				doIncludePathFlag,
				doIncludePathFileFlag,
				doIncludeBinFlag,
				doIncludeExeFlag,
				doIncludeShellFlag,
				doUseMountFlag,
				doConfinueAfterFlag,
			},
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("[profile] missing image ID/name...\n\n")
					cli.ShowCommandHelp(ctx, CmdProfile)
					return nil
				}

				doCheckVersion := ctx.GlobalBool(FlagCheckVersion)

				statePath := ctx.GlobalString(FlagStatePath)

				imageRef := ctx.Args().First()
				clientConfig := getDockerClientConfig(ctx)

				doHTTPProbe := ctx.Bool(FlagHTTPProbe)

				httpProbeCmds, err := getHTTPProbes(ctx)
				if err != nil {
					fmt.Printf("[profile] invalid HTTP probes: %v\n", err)
					return err
				}

				if doHTTPProbe {
					//add default probe cmd if the "http-probe" flag is explicitly set
					fmt.Println("docker-slim[profile]: info=http.probe message='using default probe'")
					httpProbeCmds = append(httpProbeCmds,
						config.HTTPProbeCmd{Protocol: "http", Method: "GET", Resource: "/"})
				}

				if len(httpProbeCmds) > 0 {
					doHTTPProbe = true
				}

				httpProbeRetryCount := ctx.Int(FlagHTTPProbeRetryCount)
				httpProbeRetryWait := ctx.Int(FlagHTTPProbeRetryWait)
				httpProbePorts, err := parseHTTPProbesPorts(ctx.String(FlagHTTPProbePorts))
				if err != nil {
					fmt.Printf("[profile] invalid HTTP Probe target ports: %v\n", err)
					return err
				}

				doHTTPProbeFull := ctx.Bool(FlagHTTPProbeFull)

				doShowContainerLogs := ctx.Bool(FlagShowContainerLogs)
				overrides, err := getContainerOverrides(ctx)
				if err != nil {
					fmt.Printf("[profile] invalid container overrides: %v", err)
					return err
				}

				volumeMounts, err := parseVolumeMounts(ctx.StringSlice(FlagMount))
				if err != nil {
					fmt.Printf("[profile] invalid volume mounts: %v\n", err)
					return err
				}

				excludePaths := parsePaths(ctx.StringSlice(FlagExcludePath))

				includePaths := parsePaths(ctx.StringSlice(FlagIncludePath))
				moreIncludePaths, err := parsePathsFile(ctx.String(FlagIncludePathFile))
				if err != nil {
					fmt.Printf("[profile] could not read include path file (ignoring): %v\n", err)
				} else {
					for k, v := range moreIncludePaths {
						includePaths[k] = v
					}
				}

				includeBins := parsePaths(ctx.StringSlice(FlagIncludeBin))
				includeExes := parsePaths(ctx.StringSlice(FlagIncludeExe))
				doIncludeShell := ctx.Bool(FlagIncludeShell)

				doExcludeMounts := ctx.BoolT(FlagExludeMounts)
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

				commands.OnProfile(
					doCheckVersion,
					ctx.GlobalString(FlagCommandReport),
					ctx.GlobalBool(FlagDebug),
					statePath,
					clientConfig,
					imageRef,
					doHTTPProbe,
					httpProbeCmds,
					httpProbeRetryCount,
					httpProbeRetryWait,
					httpProbePorts,
					doHTTPProbeFull,
					doShowContainerLogs,
					overrides,
					ctx.StringSlice(FlagLink),
					ctx.StringSlice(FlagEtcHostsMap),
					ctx.StringSlice(FlagContainerDNS),
					ctx.StringSlice(FlagContainerDNSSearch),
					volumeMounts,
					excludePaths,
					includePaths,
					includeBins,
					includeExes,
					doIncludeShell,
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

	doConfinueAfter := ctx.String(FlagContinueAfter)
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
	doUseEntrypoint := ctx.String(FlagEntrypoint)
	doUseCmd := ctx.String(FlagCmd)
	doUseExpose := ctx.StringSlice(FlagExpose)

	overrides := &config.ContainerOverrides{
		Workdir:  ctx.String(FlagWorkdir),
		Env:      ctx.StringSlice(FlagEnv),
		Network:  ctx.String(FlagNetwork),
		Hostname: ctx.String(FlagHostname),
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

func getImageInstructions(ctx *cli.Context) (*config.ImageNewInstructions, error) {
	entrypoint := ctx.String(FlagNewEntrypoint)
	cmd := ctx.String(FlagNewCmd)
	expose := ctx.StringSlice(FlagNewExpose)

	instructions := &config.ImageNewInstructions{
		Workdir: ctx.String(FlagNewWorkdir),
		Env:     ctx.StringSlice(FlagNewEnv),
	}

	//TODO(future): also load instructions from a file

	var err error
	if len(expose) > 0 {
		instructions.ExposedPorts, err = parseDockerExposeOpt(expose)
		if err != nil {
			log.Errorf("getImageInstructions(): invalid expose options => %v", err)
			return nil, err
		}
	}

	instructions.Entrypoint, err = parseExec(entrypoint)
	if err != nil {
		log.Errorf("getImageInstructions(): invalid entrypoint option => %v", err)
		return nil, err
	}

	//one space is a hacky way to indicate that you want to remove this instruction from the image
	instructions.ClearEntrypoint = isOneSpace(entrypoint)

	instructions.Cmd, err = parseExec(cmd)
	if err != nil {
		log.Errorf("getImageInstructions(): invalid cmd option => %v", err)
		return nil, err
	}

	//same hack to indicate you want to remove this instruction
	instructions.ClearCmd = isOneSpace(cmd)

	return instructions, nil
}

func getHTTPProbes(ctx *cli.Context) ([]config.HTTPProbeCmd, error) {
	httpProbeCmds, err := parseHTTPProbes(ctx.StringSlice(FlagHTTPProbeCmd))
	if err != nil {
		return nil, err
	}

	moreHTTPProbeCmds, err := parseHTTPProbesFile(ctx.String(FlagHTTPProbeCmdFile))
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
		UseTLS:      ctx.GlobalBool(FlagUseTLS),
		VerifyTLS:   ctx.GlobalBool(FlagVerifyTLS),
		TLSCertPath: ctx.GlobalString(FlagTLSCertPath),
		Host:        ctx.GlobalString(FlagHost),
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
