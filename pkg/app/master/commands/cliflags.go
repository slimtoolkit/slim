package commands

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

/////////////////////////////////////////////////////////
//Flags
/////////////////////////////////////////////////////////

// Global flag names
const (
	FlagCommandReport = "report"
	FlagCheckVersion  = "check-version"
	FlagDebug         = "debug"
	FlagVerbose       = "verbose"
	FlagLogLevel      = "log-level"
	FlagLog           = "log"
	FlagLogFormat     = "log-format"
	FlagUseTLS        = "tls"
	FlagVerifyTLS     = "tls-verify"
	FlagTLSCertPath   = "tls-cert-path"
	FlagHost          = "host"
	FlagStatePath     = "state-path"
	FlagInContainer   = "in-container"
	FlagArchiveState  = "archive-state"
	FlagNoColor       = "no-color"
)

// Global flag usage info
const (
	FlagCommandReportUsage = "command report location (enabled by default; set it to \"off\" to disable it)"
	FlagCheckVersionUsage  = "check if the current version is outdated"
	FlagDebugUsage         = "enable debug logs"
	FlagVerboseUsage       = "enable info logs"
	FlagLogLevelUsage      = "set the logging level ('trace', 'debug', 'info', 'warn' (default), 'error', 'fatal', 'panic')"
	FlagLogUsage           = "log file to store logs"
	FlagLogFormatUsage     = "set the format used by logs ('text' (default), or 'json')"
	FlagUseTLSUsage        = "use TLS"
	FlagVerifyTLSUsage     = "verify TLS"
	FlagTLSCertPathUsage   = "path to TLS cert files"
	FlagHostUsage          = "Docker host address"
	FlagStatePathUsage     = "DockerSlim state base path"
	FlagInContainerUsage   = "DockerSlim is running in a container"
	FlagArchiveStateUsage  = "archive DockerSlim state to the selected Docker volume (default volume - docker-slim-state). By default, enabled when DockerSlim is running in a container (disabled otherwise). Set it to \"off\" to disable explicitly."
	FlagNoColorUsage       = "disable color output"
)

// Shared command flag names
const (
	FlagTarget       = "target"
	FlagPull         = "pull"
	FlagShowPullLogs = "show-plogs"

	FlagRemoveFileArtifacts = "remove-file-artifacts"
	FlagCopyMetaArtifacts   = "copy-meta-artifacts"

	FlagHTTPProbe                 = "http-probe"
	FlagHTTPProbeOff              = "http-probe-off" //alternative way to disable http probing
	FlagHTTPProbeCmd              = "http-probe-cmd"
	FlagHTTPProbeCmdFile          = "http-probe-cmd-file"
	FlagHTTPProbeRetryCount       = "http-probe-retry-count"
	FlagHTTPProbeRetryWait        = "http-probe-retry-wait"
	FlagHTTPProbePorts            = "http-probe-ports"
	FlagHTTPProbeFull             = "http-probe-full"
	FlagHTTPProbeExitOnFailure    = "http-probe-exit-on-failure"
	FlagHTTPProbeCrawl            = "http-probe-crawl"
	FlagHTTPCrawlMaxDepth         = "http-crawl-max-depth"
	FlagHTTPCrawlMaxPageCount     = "http-crawl-max-page-count"
	FlagHTTPCrawlConcurrency      = "http-crawl-concurrency"
	FlagHTTPMaxConcurrentCrawlers = "http-max-concurrent-crawlers"
	FlagHTTPProbeAPISpec          = "http-probe-apispec"
	FlagHTTPProbeAPISpecFile      = "http-probe-apispec-file"
	FlagHTTPProbeExec             = "http-probe-exec"
	FlagHTTPProbeExecFile         = "http-probe-exec-file"

	FlagPublishPort         = "publish-port"
	FlagPublishExposedPorts = "publish-exposed-ports"

	FlagRunTargetAsUser   = "run-target-as-user"
	FlagShowContainerLogs = "show-clogs"

	FlagExcludePattern  = "exclude-pattern"
	FlagExcludeMounts   = "exclude-mounts"
	FlagUseLocalMounts  = "use-local-mounts"
	FlagUseSensorVolume = "use-sensor-volume"
	FlagContinueAfter   = "continue-after"

	FlagExec     = "exec"
	FlagExecFile = "exec-file"

	//Container Run Options (for build, profile and run commands)
	FlagCRORuntime        = "cro-runtime"
	FlagCROHostConfigFile = "cro-host-config-file"
	FlagCROSysctl         = "cro-sysctl"
	FlagCROShmSize        = "cro-shm-size"

	//Original Container Runtime Options (without cro- prefix)
	FlagEntrypoint         = "entrypoint"
	FlagCmd                = "cmd"
	FlagWorkdir            = "workdir"
	FlagEnv                = "env"
	FlagLabel              = "label"
	FlagVolume             = "volume"
	FlagExpose             = "expose"
	FlagLink               = "link"
	FlagNetwork            = "network"
	FlagHostname           = "hostname"
	FlagEtcHostsMap        = "etc-hosts-map"
	FlagContainerDNS       = "container-dns"
	FlagContainerDNSSearch = "container-dns-search"
	FlagMount              = "mount"
)

// Shared command flag usage info
const (
	FlagTargetUsage       = "Target container image (name or ID)"
	FlagPullUsage         = "Try pulling target if it's not available locally"
	FlagShowPullLogsUsage = "Show image pull logs"

	FlagRemoveFileArtifactsUsage = "remove file artifacts when command is done"
	FlagCopyMetaArtifactsUsage   = "copy metadata artifacts to the selected location when command is done"

	FlagHTTPProbeUsage                 = "Enable or disable HTTP probing"
	FlagHTTPProbeOffUsage              = "Alternative way to disable HTTP probing"
	FlagHTTPProbeCmdUsage              = "User defined HTTP probes"
	FlagHTTPProbeCmdFileUsage          = "File with user defined HTTP probes"
	FlagHTTPProbeRetryCountUsage       = "Number of retries for each HTTP probe"
	FlagHTTPProbeRetryWaitUsage        = "Number of seconds to wait before retrying HTTP probe (doubles when target is not ready)"
	FlagHTTPProbePortsUsage            = "Explicit list of ports to probe (in the order you want them to be probed)"
	FlagHTTPProbeFullUsage             = "Do full HTTP probe for all selected ports (if false, finish after first successful scan)"
	FlagHTTPProbeExitOnFailureUsage    = "Exit when all HTTP probe commands fail"
	FlagHTTPProbeCrawlUsage            = "Enable crawling for the default HTTP probe command"
	FlagHTTPCrawlMaxDepthUsage         = "Max depth to use for the HTTP probe crawler"
	FlagHTTPCrawlMaxPageCountUsage     = "Max number of pages to visit for the HTTP probe crawler"
	FlagHTTPCrawlConcurrencyUsage      = "Number of concurrent workers when crawling an HTTP target"
	FlagHTTPMaxConcurrentCrawlersUsage = "Number of concurrent crawlers in the HTTP probe"
	FlagHTTPProbeAPISpecUsage          = "Run HTTP probes for API spec"
	FlagHTTPProbeAPISpecFileUsage      = "Run HTTP probes for API spec from file"
	FlagHTTPProbeExecUsage             = "App to execute when running HTTP probes"
	FlagHTTPProbeExecFileUsage         = "Apps to execute when running HTTP probes loaded from file"

	FlagPublishPortUsage         = "Map container port to host port (format => port | hostPort:containerPort | hostIP:hostPort:containerPort | hostIP::containerPort )"
	FlagPublishExposedPortsUsage = "Map all exposed ports to the same host ports"

	FlagRunTargetAsUserUsage   = "Run target app as USER"
	FlagShowContainerLogsUsage = "Show container logs"

	FlagExcludeMountsUsage   = "Exclude mounted volumes from image"
	FlagExcludePatternUsage  = "Exclude path pattern (Glob/Match in Go and **) from image"
	FlagUseLocalMountsUsage  = "Mount local paths for target container artifact input and output"
	FlagUseSensorVolumeUsage = "Sensor volume name to use"
	FlagContinueAfterUsage   = "Select continue mode: enter | signal | probe | timeout or numberInSeconds"

	FlagExecUsage     = "A shell script snippet to run via Docker exec"
	FlagExecFileUsage = "A shell script file to run via Docker exec"

	//Container Run Options (for build, profile and run commands)
	FlagCRORuntimeUsage        = "Runtime to use with the created containers"
	FlagCROHostConfigFileUsage = "Base Docker host configuration file (JSON format) to use when running the container"
	FlagCROSysctlUsage         = "Set namespaced kernel parameters in the created container"
	FlagCROShmSizeUsage        = "Shared memory size for /dev/shm in the created container"

	FlagEntrypointUsage         = "Override ENTRYPOINT analyzing image at runtime"
	FlagCmdUsage                = "Override CMD analyzing image at runtime"
	FlagWorkdirUsage            = "Override WORKDIR analyzing image at runtime"
	FlagEnvUsage                = "Override or add ENV analyzing image at runtime"
	FlagLabelUsage              = "Override or add LABEL analyzing image at runtime"
	FlagVolumeUsage             = "Add VOLUME analyzing image at runtime"
	FlagExposeUsage             = "Use additional EXPOSE instructions analyzing image at runtime"
	FlagLinkUsage               = "Add link to another container analyzing image at runtime"
	FlagNetworkUsage            = "Override default container network settings analyzing image at runtime"
	FlagHostnameUsage           = "Override default container hostname analyzing image at runtime"
	FlagEtcHostsMapUsage        = "Add a host to IP mapping to /etc/hosts analyzing image at runtime"
	FlagContainerDNSUsage       = "Add a dns server analyzing image at runtime"
	FlagContainerDNSSearchUsage = "Add a dns search domain for unqualified hostnames analyzing image at runtime"
	FlagMountUsage              = "Mount volume analyzing image"
)

///////////////////////////////////

func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  FlagCommandReport,
			Value: "slim.report.json",
			Usage: "command report location (enabled by default; set it to \"off\" to disable it)",
		},
		cli.BoolTFlag{
			Name:   FlagCheckVersion,
			Usage:  "check if the current version is outdated",
			EnvVar: "DSLIM_CHECK_VERSION",
		},
		cli.BoolFlag{
			Name:  FlagDebug,
			Usage: FlagDebugUsage,
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
		cli.BoolFlag{
			Name:  FlagInContainer,
			Usage: "DockerSlim is running in a container",
		},
		cli.StringFlag{
			Name:  FlagArchiveState,
			Value: "",
			Usage: "archive DockerSlim state to the selected Docker volume (default volume - docker-slim-state). By default, enabled when DockerSlim is running in a container (disabled otherwise). Set it to \"off\" to disable explicitly.",
		},
		cli.BoolFlag{
			Name:  FlagNoColor,
			Usage: FlagNoColorUsage,
		},
	}
}

var CommonFlags = map[string]cli.Flag{
	FlagTarget: cli.StringFlag{
		Name:   FlagTarget,
		Value:  "",
		Usage:  FlagTargetUsage,
		EnvVar: "DSLIM_TARGET",
	},
	FlagPull: cli.BoolFlag{
		Name:   FlagPull,
		Usage:  FlagPullUsage,
		EnvVar: "DSLIM_PULL",
	},
	FlagShowPullLogs: cli.BoolFlag{
		Name:   FlagShowPullLogs,
		Usage:  FlagShowPullLogsUsage,
		EnvVar: "DSLIM_PLOG",
	},
	FlagRemoveFileArtifacts: cli.BoolFlag{
		Name:   FlagRemoveFileArtifacts,
		Usage:  FlagRemoveFileArtifactsUsage,
		EnvVar: "DSLIM_RM_FILE_ARTIFACTS",
	},
	FlagCopyMetaArtifacts: cli.StringFlag{
		Name:   FlagCopyMetaArtifacts,
		Usage:  FlagCopyMetaArtifactsUsage,
		EnvVar: "DSLIM_CP_META_ARTIFACTS",
	},
	//
	FlagHTTPProbe: cli.BoolTFlag{ //true by default
		Name:   FlagHTTPProbe,
		Usage:  FlagHTTPProbeUsage,
		EnvVar: "DSLIM_HTTP_PROBE",
	},
	FlagHTTPProbeOff: cli.BoolFlag{
		Name:   FlagHTTPProbeOff,
		Usage:  FlagHTTPProbeOffUsage,
		EnvVar: "DSLIM_HTTP_PROBE_OFF",
	},
	FlagHTTPProbeCmd: cli.StringSliceFlag{
		Name:   FlagHTTPProbeCmd,
		Value:  &cli.StringSlice{},
		Usage:  FlagHTTPProbeCmdUsage,
		EnvVar: "DSLIM_HTTP_PROBE_CMD",
	},
	FlagHTTPProbeCmdFile: cli.StringFlag{
		Name:   FlagHTTPProbeCmdFile,
		Value:  "",
		Usage:  FlagHTTPProbeCmdFileUsage,
		EnvVar: "DSLIM_HTTP_PROBE_CMD_FILE",
	},
	FlagHTTPProbeAPISpec: cli.StringSliceFlag{
		Name:   FlagHTTPProbeAPISpec,
		Value:  &cli.StringSlice{},
		Usage:  FlagHTTPProbeAPISpecUsage,
		EnvVar: "DSLIM_HTTP_PROBE_API_SPEC",
	},
	FlagHTTPProbeAPISpecFile: cli.StringSliceFlag{
		Name:   FlagHTTPProbeAPISpecFile,
		Value:  &cli.StringSlice{},
		Usage:  FlagHTTPProbeAPISpecFileUsage,
		EnvVar: "DSLIM_HTTP_PROBE_API_SPEC_FILE",
	},
	FlagHTTPProbeRetryCount: cli.IntFlag{
		Name:   FlagHTTPProbeRetryCount,
		Value:  5,
		Usage:  FlagHTTPProbeRetryCountUsage,
		EnvVar: "DSLIM_HTTP_PROBE_RETRY_COUNT",
	},
	FlagHTTPProbeRetryWait: cli.IntFlag{
		Name:   FlagHTTPProbeRetryWait,
		Value:  8,
		Usage:  FlagHTTPProbeRetryWaitUsage,
		EnvVar: "DSLIM_HTTP_PROBE_RETRY_WAIT",
	},
	FlagHTTPProbePorts: cli.StringFlag{
		Name:   FlagHTTPProbePorts,
		Value:  "",
		Usage:  FlagHTTPProbePortsUsage,
		EnvVar: "DSLIM_HTTP_PROBE_PORTS",
	},
	FlagHTTPProbeFull: cli.BoolFlag{
		Name:   FlagHTTPProbeFull,
		Usage:  FlagHTTPProbeFullUsage,
		EnvVar: "DSLIM_HTTP_PROBE_FULL",
	},
	FlagHTTPProbeExitOnFailure: cli.BoolTFlag{ //true by default now
		Name:   FlagHTTPProbeExitOnFailure,
		Usage:  FlagHTTPProbeExitOnFailureUsage,
		EnvVar: "DSLIM_HTTP_PROBE_EXIT_ON_FAILURE",
	},
	FlagHTTPProbeCrawl: cli.BoolTFlag{
		Name:   FlagHTTPProbeCrawl,
		Usage:  FlagHTTPProbeCrawl,
		EnvVar: "DSLIM_HTTP_PROBE_CRAWL",
	},
	FlagHTTPCrawlMaxDepth: cli.IntFlag{
		Name:   FlagHTTPCrawlMaxDepth,
		Value:  3,
		Usage:  FlagHTTPCrawlMaxDepthUsage,
		EnvVar: "DSLIM_HTTP_CRAWL_MAX_DEPTH",
	},
	FlagHTTPCrawlMaxPageCount: cli.IntFlag{
		Name:   FlagHTTPCrawlMaxPageCount,
		Value:  1000,
		Usage:  FlagHTTPCrawlMaxPageCountUsage,
		EnvVar: "DSLIM_HTTP_CRAWL_MAX_PAGE_COUNT",
	},
	FlagHTTPCrawlConcurrency: cli.IntFlag{
		Name:   FlagHTTPCrawlConcurrency,
		Value:  10,
		Usage:  FlagHTTPCrawlConcurrencyUsage,
		EnvVar: "DSLIM_HTTP_CRAWL_CONCURRENCY",
	},
	FlagHTTPMaxConcurrentCrawlers: cli.IntFlag{
		Name:   FlagHTTPMaxConcurrentCrawlers,
		Value:  1,
		Usage:  FlagHTTPMaxConcurrentCrawlersUsage,
		EnvVar: "DSLIM_HTTP_MAX_CONCURRENT_CRAWLERS",
	},
	FlagHTTPProbeExec: cli.StringSliceFlag{
		Name:   FlagHTTPProbeExec,
		Value:  &cli.StringSlice{},
		Usage:  FlagHTTPProbeExecUsage,
		EnvVar: "DSLIM_HTTP_PROBE_EXEC",
	},
	FlagHTTPProbeExecFile: cli.StringFlag{
		Name:   FlagHTTPProbeExecFile,
		Value:  "",
		Usage:  FlagHTTPProbeExecFileUsage,
		EnvVar: "DSLIM_HTTP_PROBE_EXEC_FILE",
	},
	FlagPublishPort: cli.StringSliceFlag{
		Name:   FlagPublishPort,
		Value:  &cli.StringSlice{},
		Usage:  FlagPublishPortUsage,
		EnvVar: "DSLIM_PUBLISH_PORT",
	},
	FlagPublishExposedPorts: cli.BoolFlag{
		Name:   FlagPublishExposedPorts,
		Usage:  FlagPublishExposedPortsUsage,
		EnvVar: "DSLIM_PUBLISH_EXPOSED",
	},
	FlagRunTargetAsUser: cli.BoolTFlag{
		Name:   FlagRunTargetAsUser,
		Usage:  FlagRunTargetAsUserUsage,
		EnvVar: "DSLIM_RUN_TAS_USER",
	},
	FlagShowContainerLogs: cli.BoolFlag{
		Name:   FlagShowContainerLogs,
		Usage:  FlagShowContainerLogsUsage,
		EnvVar: "DSLIM_SHOW_CLOGS",
	},
	FlagExec: cli.StringFlag{
		Name:   FlagExec,
		Value:  "",
		Usage:  FlagExecUsage,
		EnvVar: "DSLIM_RC_EXE",
	},
	FlagExecFile: cli.StringFlag{
		Name:   FlagExecFile,
		Value:  "",
		Usage:  FlagExecFileUsage,
		EnvVar: "DSLIM_RC_EXE_FILE",
	},
	FlagExcludeMounts: cli.BoolTFlag{
		Name:   FlagExcludeMounts, //true by default
		Usage:  FlagExcludeMountsUsage,
		EnvVar: "DSLIM_EXCLUDE_MOUNTS",
	},
	FlagExcludePattern: cli.StringSliceFlag{
		Name:   FlagExcludePattern,
		Value:  &cli.StringSlice{},
		Usage:  FlagExcludePatternUsage,
		EnvVar: "DSLIM_EXCLUDE_PATTERN",
	},
	FlagUseLocalMounts: cli.BoolFlag{
		Name:   FlagUseLocalMounts,
		Usage:  FlagUseLocalMountsUsage,
		EnvVar: "DSLIM_USE_LOCAL_MOUNTS",
	},
	FlagUseSensorVolume: cli.StringFlag{
		Name:   FlagUseSensorVolume,
		Value:  "",
		Usage:  FlagUseSensorVolumeUsage,
		EnvVar: "DSLIM_USE_SENSOR_VOLUME",
	},
	FlagContinueAfter: cli.StringFlag{
		Name:   FlagContinueAfter,
		Value:  "probe",
		Usage:  FlagContinueAfterUsage,
		EnvVar: "DSLIM_CONTINUE_AFTER",
	},
	//Container Run Options
	FlagCRORuntime: cli.StringFlag{
		Name:   FlagCRORuntime,
		Value:  "",
		Usage:  FlagCRORuntimeUsage,
		EnvVar: "DSLIM_CRO_RUNTIME",
	},
	FlagCROHostConfigFile: cli.StringFlag{
		Name:   FlagCROHostConfigFile,
		Value:  "",
		Usage:  FlagCROHostConfigFileUsage,
		EnvVar: "DSLIM_CRO_HOST_CONFIG_FILE",
	},
	FlagCROSysctl: cli.StringSliceFlag{
		Name:   FlagCROSysctl,
		Value:  &cli.StringSlice{},
		Usage:  FlagCROSysctlUsage,
		EnvVar: "DSLIM_CRO_SYSCTL",
	},
	FlagCROShmSize: cli.Int64Flag{
		Name:   FlagCROShmSize,
		Value:  -1,
		Usage:  FlagCROShmSizeUsage,
		EnvVar: "DSLIM_CRO_SHM_SIZE",
	},
	FlagEntrypoint: cli.StringFlag{
		Name:   FlagEntrypoint,
		Value:  "",
		Usage:  FlagEntrypointUsage,
		EnvVar: "DSLIM_RC_ENTRYPOINT",
	},
	FlagCmd: cli.StringFlag{
		Name:   FlagCmd,
		Value:  "",
		Usage:  FlagCmdUsage,
		EnvVar: "DSLIM_RC_CMD",
	},
	FlagWorkdir: cli.StringFlag{
		Name:   FlagWorkdir,
		Value:  "",
		Usage:  FlagWorkdirUsage,
		EnvVar: "DSLIM_RC_WORKDIR",
	},
	FlagEnv: cli.StringSliceFlag{
		Name:   FlagEnv,
		Value:  &cli.StringSlice{},
		Usage:  FlagEnvUsage,
		EnvVar: "DSLIM_RC_ENV",
	},
	FlagLabel: cli.StringSliceFlag{
		Name:   FlagLabel,
		Value:  &cli.StringSlice{},
		Usage:  FlagLabelUsage,
		EnvVar: "DSLIM_RC_LABEL",
	},
	FlagVolume: cli.StringSliceFlag{
		Name:   FlagVolume,
		Value:  &cli.StringSlice{},
		Usage:  FlagVolumeUsage,
		EnvVar: "DSLIM_RC_VOLUME",
	},
	FlagLink: cli.StringSliceFlag{
		Name:   FlagLink,
		Value:  &cli.StringSlice{},
		Usage:  FlagLinkUsage,
		EnvVar: "DSLIM_RC_LINK",
	},
	FlagEtcHostsMap: cli.StringSliceFlag{
		Name:   FlagEtcHostsMap,
		Value:  &cli.StringSlice{},
		Usage:  FlagEtcHostsMapUsage,
		EnvVar: "DSLIM_RC_ETC_HOSTS_MAP",
	},
	FlagContainerDNS: cli.StringSliceFlag{
		Name:   FlagContainerDNS,
		Value:  &cli.StringSlice{},
		Usage:  FlagContainerDNSUsage,
		EnvVar: "DSLIM_RC_DNS",
	},
	FlagContainerDNSSearch: cli.StringSliceFlag{
		Name:   FlagContainerDNSSearch,
		Value:  &cli.StringSlice{},
		Usage:  FlagContainerDNSSearchUsage,
		EnvVar: "DSLIM_RC_DNS_SEARCH",
	},
	FlagHostname: cli.StringFlag{
		Name:   FlagHostname,
		Value:  "",
		Usage:  FlagHostnameUsage,
		EnvVar: "DSLIM_RC_HOSTNAME",
	},
	FlagNetwork: cli.StringFlag{
		Name:   FlagNetwork,
		Value:  "",
		Usage:  FlagNetworkUsage,
		EnvVar: "DSLIM_RC_NET",
	},
	FlagExpose: cli.StringSliceFlag{
		Name:   FlagExpose,
		Value:  &cli.StringSlice{},
		Usage:  FlagExposeUsage,
		EnvVar: "DSLIM_RC_EXPOSE",
	},
	FlagMount: cli.StringSliceFlag{
		Name:   FlagMount,
		Value:  &cli.StringSlice{},
		Usage:  FlagMountUsage,
		EnvVar: "DSLIM_MOUNT",
	},
}

//var CommonFlags

func Cflag(name string) cli.Flag {
	cf, ok := CommonFlags[name]
	if !ok {
		log.Fatalf("commands.Cflag: unknown flag='%s'", name)
	}

	return cf
}

///////////////////////////////////

// Update command flag names
const (
	FlagShowProgress = "show-progress"
)

// Update command flag usage info
const (
	FlagShowProgressUsage = "show progress when the release package is downloaded"
)
