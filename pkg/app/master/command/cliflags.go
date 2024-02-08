package command

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/tools/clientcmd"
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
	FlagQuietCLIMode  = "quiet"
	FlagLogLevel      = "log-level"
	FlagLog           = "log"
	FlagLogFormat     = "log-format"
	FlagAPIVersion    = "crt-api-version"
	FlagUseTLS        = "tls"
	FlagVerifyTLS     = "tls-verify"
	FlagTLSCertPath   = "tls-cert-path"
	FlagHost          = "host"
	FlagStatePath     = "state-path"
	FlagInContainer   = "in-container"
	FlagArchiveState  = "archive-state"
	FlagNoColor       = "no-color"
	FlagOutputFormat  = "output-format"
)

const (
	OutputFormatJSON = "json"
	OutputFormatText = "text"
)

// Global flag usage info
const (
	FlagCommandReportUsage = "command report location (enabled by default; set it to \"off\" to disable it)"
	FlagCheckVersionUsage  = "check if the current version is outdated"
	FlagDebugUsage         = "enable debug logs"
	FlagVerboseUsage       = "enable info logs"
	FlagQuietCLIModeUsage  = "Quiet CLI execution mode"
	FlagLogLevelUsage      = "set the logging level ('trace', 'debug', 'info', 'warn' (default), 'error', 'fatal', 'panic')"
	FlagLogUsage           = "log file to store logs"
	FlagLogFormatUsage     = "set the format used by logs ('text' (default), or 'json')"
	FlagOutputFormatUsage  = "set the output format to use ('text' (default), or 'json')"
	FlagUseTLSUsage        = "use TLS"
	FlagVerifyTLSUsage     = "verify TLS"
	FlagTLSCertPathUsage   = "path to TLS cert files"
	FlagAPIVersionUsage    = "Container runtime API version"
	FlagHostUsage          = "Docker host address or socket (prefix with 'tcp://' or 'unix://')"
	FlagStatePathUsage     = "app state base path"
	FlagInContainerUsage   = "app is running in a container"
	FlagArchiveStateUsage  = "archive app state to the selected Docker volume (default volume - slim-state). By default, enabled when app is running in a container (disabled otherwise). Set it to \"off\" to disable explicitly."
	FlagNoColorUsage       = "disable color output"
)

// Shared command flag names
const (
	FlagCommandParamsFile = "command-params-file"
	FlagTarget            = "target"
	FlagPull              = "pull"
	FlagDockerConfigPath  = "docker-config-path"
	FlagRegistryAccount   = "registry-account"
	FlagRegistrySecret    = "registry-secret"
	FlagShowPullLogs      = "show-plogs"

	//Compose-related flags
	FlagComposeFile                    = "compose-file"
	FlagTargetComposeSvc               = "target-compose-svc"
	FlagTargetComposeSvcImage          = "target-compose-svc-image"
	FlagComposeSvcStartWait            = "compose-svc-start-wait"
	FlagComposeSvcNoPorts              = "target-compose-svc-no-ports"
	FlagDepExcludeComposeSvcAll        = "dep-exclude-compose-svc-all"
	FlagDepIncludeComposeSvc           = "dep-include-compose-svc"
	FlagDepExcludeComposeSvc           = "dep-exclude-compose-svc"
	FlagDepIncludeComposeSvcDeps       = "dep-include-compose-svc-deps"
	FlagDepIncludeTargetComposeSvcDeps = "dep-include-target-compose-svc-deps"
	FlagComposeNet                     = "compose-net"
	FlagComposeEnvNoHost               = "compose-env-nohost"
	FlagComposeEnvFile                 = "compose-env-file"
	FlagComposeWorkdir                 = "compose-workdir"
	FlagComposeProjectName             = "compose-project-name"
	FlagPrestartComposeSvc             = "prestart-compose-svc"
	FlagPoststartComposeSvc            = "poststart-compose-svc"
	FlagPrestartComposeWaitExit        = "prestart-compose-wait-exit"
	FlagContainerProbeComposeSvc       = "container-probe-compose-svc"

	//Kubernetes-related flags
	FlagTargetKubeWorkload          = "target-kube-workload" // <kind>/<name> e.g: deployment/foo, job/bar
	FlagTargetKubeWorkloadNamespace = "target-kube-workload-namespace"
	FlagTargetKubeWorkloadContainer = "target-kube-workload-container"
	FlagTargetKubeWorkloadImage     = "target-kube-workload-image"
	FlagKubeManifestFile            = "kube-manifest-file"
	FlagKubeKubeconfigFile          = "kube-kubeconfig-file"
	// TODO: FlagKubeContext        = "kube-context"
	//       FlagKubeCluster        =" kube-cluster"
	//       etc.
	// Naming convention: keep the well known kubectl flag names as-is and prefix them with `--kube-`

	FlagRemoveFileArtifacts = "remove-file-artifacts"
	FlagCopyMetaArtifacts   = "copy-meta-artifacts"

	FlagHTTPProbe                 = "http-probe"
	FlagHTTPProbeOff              = "http-probe-off" //alternative way to disable http probing
	FlagHTTPProbeCmd              = "http-probe-cmd"
	FlagHTTPProbeCmdFile          = "http-probe-cmd-file"
	FlagHTTPProbeStartWait        = "http-probe-start-wait"
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
	FlagHTTPProbeProxyEndpoint    = "http-probe-proxy-endpoint"
	FlagHTTPProbeProxyPort        = "http-probe-proxy-port"

	FlagHostExec     = "host-exec"
	FlagHostExecFile = "host-exec-file"

	FlagPublishPort         = "publish-port"
	FlagPublishExposedPorts = "publish-exposed-ports"

	FlagRunTargetAsUser   = "run-target-as-user"
	FlagShowContainerLogs = "show-clogs"
	FlagEnableMondelLogs  = "enable-mondel" //Mon(itor) Data Event Log

	FlagUseLocalMounts  = "use-local-mounts"
	FlagUseSensorVolume = "use-sensor-volume"
	FlagContinueAfter   = "continue-after"

	//RunTime Analysis Options
	FlagRTAOnbuildBaseImage = "rta-onbuild-base-image"
	FlagRTASourcePT         = "rta-source-ptrace"

	//Sensor IPC Options (for build and profile commands)
	FlagSensorIPCEndpoint = "sensor-ipc-endpoint"
	FlagSensorIPCMode     = "sensor-ipc-mode"

	FlagExec     = "exec"
	FlagExecFile = "exec-file"

	//Container Run Options (for build, profile and run commands)
	FlagCRORuntime        = "cro-runtime"
	FlagCROHostConfigFile = "cro-host-config-file"
	FlagCROSysctl         = "cro-sysctl"
	FlagCROShmSize        = "cro-shm-size"

	//Original Container Runtime Options (without cro- prefix)
	FlagUser               = "user"
	FlagEntrypoint         = "entrypoint"
	FlagCmd                = "cmd"
	FlagWorkdir            = "workdir"
	FlagEnv                = "env"
	FlagEnvFile            = "env-file"
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
	FlagDeleteFatImage     = "delete-generated-fat-image"
)

// Shared command flag usage info
const (
	FlagCommandParamsFileUsage = "JSON file with all command parameters"
	FlagTargetUsage            = "Target container image (name or ID)"
	FlagPullUsage              = "Try pulling target if it's not available locally"
	FlagDockerConfigPathUsage  = "Docker config path (used to fetch registry credentials)"
	FlagRegistryAccountUsage   = "Target registry account used when pulling images from private registries"
	FlagRegistrySecretUsage    = "Target registry secret used when pulling images from private registries"
	FlagShowPullLogsUsage      = "Show image pull logs"

	//Compose-related flags
	FlagComposeFileUsage                    = "Load container info from selected compose file(s)"
	FlagTargetComposeSvcUsage               = "Target service from compose file"
	FlagTargetComposeSvcImageUsage          = "Override the container image name and/or tag when targetting a compose service using the target-compose-svc parameter (format: tag_name or image_name:tag_name)"
	FlagComposeSvcStartWaitUsage            = "Number of seconds to wait before starting each compose service"
	FlagComposeSvcNoPortsUsage              = "Do not publish ports for target service from compose file"
	FlagDepExcludeComposeSvcAllUsage        = "Do not start any compose services as target dependencies"
	FlagDepIncludeComposeSvcUsage           = "Include specific compose service as a target dependency (only selected services will be started)"
	FlagDepExcludeComposeSvcUsage           = "Exclude specific service from the compose services that will be started as target dependencies"
	FlagDepIncludeComposeSvcDepsUsage       = "Include all dependencies for the selected compose service (excluding the service itself) as target dependencies"
	FlagDepIncludeTargetComposeSvcDepsUsage = "Include all dependencies for the target compose service (excluding the service itself) as target dependencies"
	FlagComposeNetUsage                     = "Attach target to the selected compose network(s) otherwise all networks will be attached"
	FlagComposeEnvNoHostUsage               = "Don't include the env vars from the host to compose"
	FlagComposeEnvFileUsage                 = "Load compose env vars from file (host env vars override the values loaded from this file)"
	FlagComposeWorkdirUsage                 = "Set custom work directory for compose"
	FlagContainerProbeComposeSvcUsage       = "Container test/probe service from compose file"
	FlagComposeProjectNameUsage             = "Use custom project name for compose"
	FlagPrestartComposeSvcUsage             = "Run selected compose service(s) before any other compose services or target container"
	FlagPoststartComposeSvcUsage            = "Run selected compose service(s) after the target container is running (need a new continue after mode too)"
	FlagPrestartComposeWaitExitUsage        = "Wait for selected prestart compose services to exit before starting other compose services or target container"

	//Kubernetes-related flags
	FlagTargetKubeWorkloadUsage          = "[Experimental] Target Kubernetes workload from the manifests (if --kube-manifest-file is provided) or in the default kubeconfig cluster (format: <resource>/<name>, e.g., deployments/foobar)"
	FlagTargetKubeWorkloadNamespaceUsage = "[Experimental] Target Kubernetes workload namespace (if not set, the value from the manifest is used if provided, otherwise - \"default\")"
	FlagTargetKubeWorkloadContainerUsage = "[Experimental] Target container in the Kubernetes workload's pod template spec"
	FlagTargetKubeWorkloadImageUsage     = "[Experimental] Override the container image name and/or tag when targetting a Kubernetes workload (format: tag_name or image_name:tag_name)"
	FlagKubeManifestFileUsage            = "[Experimental] Kubernetes manifest(s) to apply before run"
	FlagKubeKubeconfigFileUsage          = "[Experimental] Path to the kubeconfig file"

	FlagRemoveFileArtifactsUsage = "remove file artifacts when command is done"
	FlagCopyMetaArtifactsUsage   = "copy metadata artifacts to the selected location when command is done"

	FlagHTTPProbeUsage                 = "Enable or disable HTTP probing"
	FlagHTTPProbeOffUsage              = "Alternative way to disable HTTP probing"
	FlagHTTPProbeCmdUsage              = "User defined HTTP probe(s) as [[[[\"crawl\":]PROTO:]METHOD:]PATH]"
	FlagHTTPProbeCmdFileUsage          = "File with user defined HTTP probes"
	FlagHTTPProbeStartWaitUsage        = "Number of seconds to wait before starting HTTP probing"
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
	FlagHTTPProbeProxyEndpointUsage    = "Endpoint to proxy HTTP probes"
	FlagHTTPProbeProxyPortUsage        = "Port to proxy HTTP probes (used with HTTP probe proxy endpoint)"

	FlagHostExecUsage     = "Host commands to execute (aka host commands probes)"
	FlagHostExecFileUsage = "Host commands to execute loaded from file (aka host commands probes)"

	FlagPublishPortUsage         = "Map container port to host port (format => port | hostPort:containerPort | hostIP:hostPort:containerPort | hostIP::containerPort )"
	FlagPublishExposedPortsUsage = "Map all exposed ports to the same host ports"

	FlagRunTargetAsUserUsage   = "Run target app as USER"
	FlagShowContainerLogsUsage = "Show container logs"
	FlagEnableMondelLogsUsage  = "Enable data event log for sensor monitors"

	FlagUseLocalMountsUsage  = "Mount local paths for target container artifact input and output"
	FlagUseSensorVolumeUsage = "Sensor volume name to use"
	FlagContinueAfterUsage   = "Select continue mode: enter | signal | probe | timeout-number-in-seconds | container.probe"

	FlagRTAOnbuildBaseImageUsage = "Enable runtime analysis for onbuild base images"
	FlagRTASourcePTUsage         = "Enable PTRACE runtime analysis source"

	FlagSensorIPCEndpointUsage = "Override sensor IPC endpoint"
	FlagSensorIPCModeUsage     = "Select sensor IPC mode: proxy | direct"

	FlagExecUsage     = "A shell script snippet to run via Docker exec"
	FlagExecFileUsage = "A shell script file to run via Docker exec"

	//Container Run Options (for build, profile and run commands)
	FlagCRORuntimeUsage        = "Runtime to use with the created containers"
	FlagCROHostConfigFileUsage = "Base Docker host configuration file (JSON format) to use when running the container"
	FlagCROSysctlUsage         = "Set namespaced kernel parameters in the created container"
	FlagCROShmSizeUsage        = "Shared memory size for /dev/shm in the created container"

	FlagUserUsage               = "Override USER analyzing image at runtime"
	FlagEntrypointUsage         = "Override ENTRYPOINT analyzing image at runtime. To persist ENTRYPOINT changes in the output image, pass the --image-overrides=entrypoint or --image-overrides=all flag as well."
	FlagCmdUsage                = "Override CMD analyzing image at runtime. To persist CMD changes in the output image, pass the --image-overrides=cmd or --image-overrides=all flag as well."
	FlagWorkdirUsage            = "Override WORKDIR analyzing image at runtime. To persist WORKDIR changes in the output image, pass the --image-overrides=workdir or --image-overrides=all flag as well."
	FlagEnvUsage                = "Override or add ENV only during runtime. To persist ENV additions or changes in the output image, pass the --image-overrides=env or --image-overrides=all flag as well."
	FlagEnvFileUsage            = "File to override or add ENV only during runtime. To persist ENV additions or changes in the output image, pass the --image-overrides=env or --image-overrides=all flag as well."
	FlagLabelUsage              = "Override or add LABEL analyzing image at runtime. To persist LABEL additions or changes in the output image, pass the --image-overrides=label or --image-overrides=all flag as well."
	FlagVolumeUsage             = "Add VOLUME analyzing image at runtime. To persist VOLUME additions in the output image, pass the --image-overrides=volume or --image-overrides=all flag as well."
	FlagExposeUsage             = "Use additional EXPOSE instructions analyzing image at runtime. To persist EXPOSE additions in the output image, pass the --image-overrides=expose or --image-overrides=all flag as well."
	FlagLinkUsage               = "Add link to another container analyzing image at runtime"
	FlagNetworkUsage            = "Override default container network settings analyzing image at runtime"
	FlagHostnameUsage           = "Override default container hostname analyzing image at runtime"
	FlagEtcHostsMapUsage        = "Add a host to IP mapping to /etc/hosts analyzing image at runtime"
	FlagContainerDNSUsage       = "Add a dns server analyzing image at runtime"
	FlagContainerDNSSearchUsage = "Add a dns search domain for unqualified hostnames analyzing image at runtime"
	FlagMountUsage              = "Mount volume analyzing image"
	FlagDeleteFatImageUsage     = "Delete generated fat image requires --dockerfile flag"
)

///////////////////////////////////

func GlobalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  FlagCommandReport,
			Value: "slim.report.json",
			Usage: "command report location (enabled by default; set it to \"off\" to disable it)",
		},
		&cli.BoolFlag{
			Name:    FlagCheckVersion,
			Value:   true,
			Usage:   "check if the current version is outdated",
			EnvVars: []string{"DSLIM_CHECK_VERSION"},
		},
		&cli.BoolFlag{
			Name:    FlagDebug,
			Usage:   FlagDebugUsage,
			EnvVars: []string{"DSLIM_DEBUG"},
		},
		&cli.BoolFlag{
			Name:    FlagVerbose,
			Usage:   "enable info logs",
			EnvVars: []string{"DSLIM_VERBOSE"},
		},
		&cli.BoolFlag{
			Name:    FlagQuietCLIMode,
			Usage:   FlagQuietCLIModeUsage,
			EnvVars: []string{"DSLIM_QUIET"},
		},
		&cli.StringFlag{
			Name:    FlagLogLevel,
			Value:   "warn",
			Usage:   "set the logging level ('debug', 'info', 'warn' (default), 'error', 'fatal', 'panic')",
			EnvVars: []string{"DSLIM_LOG_LEVEL"},
		},
		&cli.StringFlag{
			Name:  FlagLog,
			Usage: "log file to store logs",
		},
		&cli.StringFlag{
			Name:  FlagLogFormat,
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
		&cli.StringFlag{
			Name:  FlagOutputFormat,
			Value: "text",
			Usage: FlagOutputFormatUsage,
		},
		&cli.BoolFlag{
			Name:  FlagUseTLS,
			Value: true,
			Usage: "use TLS",
		},
		&cli.BoolFlag{
			Name:  FlagVerifyTLS,
			Value: true,
			Usage: "verify TLS",
		},
		&cli.StringFlag{
			Name:  FlagTLSCertPath,
			Value: "",
			Usage: FlagTLSCertPathUsage,
		},
		&cli.StringFlag{
			Name:    FlagAPIVersion,
			Value:   "1.25", // We need at least 1.25 for to support builds from Dockerfile.
			Usage:   FlagAPIVersionUsage,
			EnvVars: []string{"DSLIM_CRT_API_VER"},
		},
		&cli.StringFlag{
			Name:  FlagHost,
			Value: "",
			Usage: "Docker host address",
		},
		&cli.StringFlag{
			Name:  FlagStatePath,
			Value: "",
			Usage: "app state base path",
		},
		&cli.BoolFlag{
			Name:  FlagInContainer,
			Usage: "app is running in a container",
		},
		&cli.StringFlag{
			Name:  FlagArchiveState,
			Value: "",
			Usage: "archive app state to the selected Docker volume (default volume - slim-state). By default, enabled when app is running in a container (disabled otherwise). Set it to \"off\" to disable explicitly.",
		},
		&cli.BoolFlag{
			Name:  FlagNoColor,
			Usage: FlagNoColorUsage,
		},
	}
}

var CommonFlags = map[string]cli.Flag{
	FlagCommandParamsFile: &cli.StringFlag{
		Name:    FlagCommandParamsFile,
		Value:   "",
		Usage:   FlagCommandParamsFileUsage,
		EnvVars: []string{"DSLIM_COMMAND_PARAMS_FILE"},
	},
	FlagTarget: &cli.StringFlag{
		Name:    FlagTarget,
		Value:   "",
		Usage:   FlagTargetUsage,
		EnvVars: []string{"DSLIM_TARGET"},
	},
	FlagPull: &cli.BoolFlag{
		Name:    FlagPull,
		Value:   true, //enabled by default
		Usage:   FlagPullUsage,
		EnvVars: []string{"DSLIM_PULL"},
	},
	FlagDockerConfigPath: &cli.StringFlag{
		Name:    FlagDockerConfigPath,
		Usage:   FlagDockerConfigPathUsage,
		EnvVars: []string{"DSLIM_DOCKER_CONFIG_PATH"},
	},
	FlagRegistryAccount: &cli.StringFlag{
		Name:    FlagRegistryAccount,
		Usage:   FlagRegistryAccountUsage,
		EnvVars: []string{"DSLIM_REGISTRY_ACCOUNT"},
	},
	FlagRegistrySecret: &cli.StringFlag{
		Name:    FlagRegistrySecret,
		Usage:   FlagRegistrySecretUsage,
		EnvVars: []string{"DSLIM_REGISTRY_SECRET"},
	},
	FlagShowPullLogs: &cli.BoolFlag{
		Name:    FlagShowPullLogs,
		Usage:   FlagShowPullLogsUsage,
		EnvVars: []string{"DSLIM_PLOG"},
	},
	//
	FlagComposeFile: &cli.StringSliceFlag{
		Name:    FlagComposeFile,
		Value:   cli.NewStringSlice(),
		Usage:   FlagComposeFileUsage,
		EnvVars: []string{"DSLIM_COMPOSE_FILE"},
	},
	FlagTargetComposeSvc: &cli.StringFlag{
		Name:    FlagTargetComposeSvc,
		Value:   "",
		Usage:   FlagTargetComposeSvcUsage,
		EnvVars: []string{"DSLIM_TARGET_COMPOSE_SVC"},
	},
	FlagTargetComposeSvcImage: &cli.StringFlag{
		Name:    FlagTargetComposeSvcImage,
		Value:   "",
		Usage:   FlagTargetComposeSvcImageUsage,
		EnvVars: []string{"DSLIM_TARGET_COMPOSE_SVC_IMAGE"},
	},
	FlagComposeSvcStartWait: &cli.IntFlag{
		Name:    FlagComposeSvcStartWait,
		Value:   0,
		Usage:   FlagComposeSvcStartWaitUsage,
		EnvVars: []string{"DSLIM_COMPOSE_SVC_START_WAIT"},
	},
	FlagComposeSvcNoPorts: &cli.BoolFlag{
		Name:    FlagComposeSvcNoPorts,
		Usage:   FlagComposeSvcNoPortsUsage,
		EnvVars: []string{"DSLIM_COMPOSE_SVC_NO_PORTS"},
	},
	FlagDepExcludeComposeSvcAll: &cli.BoolFlag{
		Name:    FlagDepExcludeComposeSvcAll,
		Usage:   FlagDepExcludeComposeSvcAllUsage,
		EnvVars: []string{"DSLIM_DEP_INCLUDE_COMPOSE_SVC_ALL"},
	},
	FlagDepIncludeComposeSvcDeps: &cli.StringFlag{
		Name:    FlagDepIncludeComposeSvcDeps,
		Value:   "",
		Usage:   FlagDepIncludeComposeSvcDepsUsage,
		EnvVars: []string{"DSLIM_DEP_INCLUDE_COMPOSE_SVC_DEPS"},
	},
	FlagDepIncludeComposeSvc: &cli.StringSliceFlag{
		Name:    FlagDepIncludeComposeSvc,
		Value:   cli.NewStringSlice(),
		Usage:   FlagDepIncludeComposeSvcUsage,
		EnvVars: []string{"DSLIM_DEP_INCLUDE_COMPOSE_SVC"},
	},
	FlagDepExcludeComposeSvc: &cli.StringSliceFlag{
		Name:    FlagDepExcludeComposeSvc,
		Value:   cli.NewStringSlice(),
		Usage:   FlagDepExcludeComposeSvcUsage,
		EnvVars: []string{"DSLIM_DEP_EXCLUDE_COMPOSE_SVC"},
	},
	FlagComposeNet: &cli.StringSliceFlag{
		Name:    FlagComposeNet,
		Value:   cli.NewStringSlice(),
		Usage:   FlagComposeNetUsage,
		EnvVars: []string{"DSLIM_COMPOSE_NET"},
	},
	FlagDepIncludeTargetComposeSvcDeps: &cli.BoolFlag{
		Name:    FlagDepIncludeTargetComposeSvcDeps,
		Usage:   FlagDepIncludeTargetComposeSvcDepsUsage,
		EnvVars: []string{"DSLIM_DEP_INCLUDE_TARGET_COMPOSE_SVC_DEPS"},
	},
	FlagComposeEnvNoHost: &cli.BoolFlag{
		Name:    FlagComposeEnvNoHost,
		Usage:   FlagComposeEnvNoHostUsage,
		EnvVars: []string{"DSLIM_COMPOSE_ENV_NOHOST"},
	},
	FlagComposeEnvFile: &cli.StringFlag{
		Name:    FlagComposeEnvFile,
		Value:   "",
		Usage:   FlagComposeEnvFileUsage,
		EnvVars: []string{"DSLIM_COMPOSE_ENV_FILE"},
	},
	FlagComposeProjectName: &cli.StringFlag{
		Name:    FlagComposeProjectName,
		Value:   "",
		Usage:   FlagComposeProjectNameUsage,
		EnvVars: []string{"DSLIM_COMPOSE_PROJECT_NAME"},
	},
	FlagComposeWorkdir: &cli.StringFlag{
		Name:    FlagComposeWorkdir,
		Value:   "",
		Usage:   FlagComposeWorkdirUsage,
		EnvVars: []string{"DSLIM_COMPOSE_WORKDIR"},
	},
	FlagContainerProbeComposeSvc: &cli.StringFlag{
		Name:    FlagContainerProbeComposeSvc,
		Value:   "",
		Usage:   FlagContainerProbeComposeSvcUsage,
		EnvVars: []string{"DSLIM_CONTAINER_PROBE_COMPOSE_SVC"},
	},
	FlagPrestartComposeSvc: &cli.StringSliceFlag{
		Name:    FlagPrestartComposeSvc,
		Value:   cli.NewStringSlice(),
		Usage:   FlagPrestartComposeSvcUsage,
		EnvVars: []string{"DSLIM_PRESTART_COMPOSE_SVC"},
	},
	FlagPrestartComposeWaitExit: &cli.BoolFlag{
		Name:    FlagPrestartComposeWaitExit,
		Usage:   FlagPrestartComposeWaitExitUsage,
		EnvVars: []string{"DSLIM_PRESTART_COMPOSE_WAIT"},
	},
	FlagPoststartComposeSvc: &cli.StringSliceFlag{
		Name:    FlagPoststartComposeSvc,
		Value:   cli.NewStringSlice(),
		Usage:   FlagPoststartComposeSvcUsage,
		EnvVars: []string{"DSLIM_POSTSTART_COMPOSE_SVC"},
	},
	//
	FlagTargetKubeWorkload: &cli.StringFlag{
		Name:    FlagTargetKubeWorkload,
		Value:   "",
		Usage:   FlagTargetKubeWorkloadUsage,
		EnvVars: []string{"DSLIM_TARGET_KUBE_WORKLOAD"},
	},
	FlagTargetKubeWorkloadNamespace: &cli.StringFlag{
		Name:    FlagTargetKubeWorkloadNamespace,
		Value:   "",
		Usage:   FlagTargetKubeWorkloadNamespaceUsage,
		EnvVars: []string{"DSLIM_TARGET_KUBE_WORKLOAD_NAMESPACE"},
	},
	FlagTargetKubeWorkloadContainer: &cli.StringFlag{
		Name:    FlagTargetKubeWorkloadContainer,
		Value:   "",
		Usage:   FlagTargetKubeWorkloadContainerUsage,
		EnvVars: []string{"DSLIM_TARGET_KUBE_WORKLOAD_CONTAINER"},
	},
	FlagTargetKubeWorkloadImage: &cli.StringFlag{
		Name:    FlagTargetKubeWorkloadImage,
		Value:   "",
		Usage:   FlagTargetKubeWorkloadImageUsage,
		EnvVars: []string{"DSLIM_TARGET_KUBE_WORKLOAD_IMAGE"},
	},
	FlagKubeManifestFile: &cli.StringSliceFlag{
		Name:    FlagKubeManifestFile,
		Value:   cli.NewStringSlice(),
		Usage:   FlagKubeManifestFileUsage,
		EnvVars: []string{"DSLIM_KUBE_MANIFEST_FILE"},
	},
	FlagKubeKubeconfigFile: &cli.StringFlag{
		Name:  FlagKubeKubeconfigFile,
		Value: clientcmd.RecommendedHomeFile,
		Usage: FlagKubeKubeconfigFileUsage,
		EnvVars: []string{
			"DSLIM_KUBE_KUBECONFIG_FILE",
			"KUBECONFIG", // subject to an industry-wide convention
		},
	},
	//
	FlagRemoveFileArtifacts: &cli.BoolFlag{
		Name:    FlagRemoveFileArtifacts,
		Usage:   FlagRemoveFileArtifactsUsage,
		EnvVars: []string{"DSLIM_RM_FILE_ARTIFACTS"},
	},
	FlagCopyMetaArtifacts: &cli.StringFlag{
		Name:    FlagCopyMetaArtifacts,
		Usage:   FlagCopyMetaArtifactsUsage,
		EnvVars: []string{"DSLIM_CP_META_ARTIFACTS"},
	},
	//
	FlagHTTPProbe: &cli.BoolFlag{ //true by default
		Name:    FlagHTTPProbe,
		Value:   true,
		Usage:   FlagHTTPProbeUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE"},
	},
	FlagHTTPProbeOff: &cli.BoolFlag{
		Name:    FlagHTTPProbeOff,
		Usage:   FlagHTTPProbeOffUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_OFF"},
	},
	FlagHTTPProbeCmd: &cli.StringSliceFlag{
		Name:    FlagHTTPProbeCmd,
		Value:   cli.NewStringSlice(),
		Usage:   FlagHTTPProbeCmdUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_CMD"},
	},
	FlagHTTPProbeCmdFile: &cli.StringFlag{
		Name:    FlagHTTPProbeCmdFile,
		Value:   "",
		Usage:   FlagHTTPProbeCmdFileUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_CMD_FILE"},
	},
	FlagHTTPProbeAPISpec: &cli.StringSliceFlag{
		Name:    FlagHTTPProbeAPISpec,
		Value:   cli.NewStringSlice(),
		Usage:   FlagHTTPProbeAPISpecUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_API_SPEC"},
	},
	FlagHTTPProbeAPISpecFile: &cli.StringSliceFlag{
		Name:    FlagHTTPProbeAPISpecFile,
		Value:   cli.NewStringSlice(),
		Usage:   FlagHTTPProbeAPISpecFileUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_API_SPEC_FILE"},
	},
	FlagHTTPProbeStartWait: &cli.IntFlag{
		Name:    FlagHTTPProbeStartWait,
		Value:   0,
		Usage:   FlagHTTPProbeStartWaitUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_START_WAIT"},
	},
	FlagHTTPProbeRetryCount: &cli.IntFlag{
		Name:    FlagHTTPProbeRetryCount,
		Value:   5,
		Usage:   FlagHTTPProbeRetryCountUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_RETRY_COUNT"},
	},
	FlagHTTPProbeRetryWait: &cli.IntFlag{
		Name:    FlagHTTPProbeRetryWait,
		Value:   8,
		Usage:   FlagHTTPProbeRetryWaitUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_RETRY_WAIT"},
	},
	FlagHTTPProbePorts: &cli.StringFlag{
		Name:    FlagHTTPProbePorts,
		Value:   "",
		Usage:   FlagHTTPProbePortsUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_PORTS"},
	},
	FlagHTTPProbeFull: &cli.BoolFlag{
		Name:    FlagHTTPProbeFull,
		Usage:   FlagHTTPProbeFullUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_FULL"},
	},
	FlagHTTPProbeExitOnFailure: &cli.BoolFlag{ //true by default now
		Name:    FlagHTTPProbeExitOnFailure,
		Value:   true,
		Usage:   FlagHTTPProbeExitOnFailureUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_EXIT_ON_FAILURE"},
	},
	FlagHTTPProbeCrawl: &cli.BoolFlag{
		Name:    FlagHTTPProbeCrawl,
		Value:   true,
		Usage:   FlagHTTPProbeCrawl,
		EnvVars: []string{"DSLIM_HTTP_PROBE_CRAWL"},
	},
	FlagHTTPCrawlMaxDepth: &cli.IntFlag{
		Name:    FlagHTTPCrawlMaxDepth,
		Value:   3,
		Usage:   FlagHTTPCrawlMaxDepthUsage,
		EnvVars: []string{"DSLIM_HTTP_CRAWL_MAX_DEPTH"},
	},
	FlagHTTPCrawlMaxPageCount: &cli.IntFlag{
		Name:    FlagHTTPCrawlMaxPageCount,
		Value:   1000,
		Usage:   FlagHTTPCrawlMaxPageCountUsage,
		EnvVars: []string{"DSLIM_HTTP_CRAWL_MAX_PAGE_COUNT"},
	},
	FlagHTTPCrawlConcurrency: &cli.IntFlag{
		Name:    FlagHTTPCrawlConcurrency,
		Value:   10,
		Usage:   FlagHTTPCrawlConcurrencyUsage,
		EnvVars: []string{"DSLIM_HTTP_CRAWL_CONCURRENCY"},
	},
	FlagHTTPMaxConcurrentCrawlers: &cli.IntFlag{
		Name:    FlagHTTPMaxConcurrentCrawlers,
		Value:   1,
		Usage:   FlagHTTPMaxConcurrentCrawlersUsage,
		EnvVars: []string{"DSLIM_HTTP_MAX_CONCURRENT_CRAWLERS"},
	},
	FlagHTTPProbeProxyEndpoint: &cli.StringFlag{
		Name:    FlagHTTPProbeProxyEndpoint,
		Value:   "",
		Usage:   FlagHTTPProbeProxyEndpointUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_PROXY_ENDPOINT"},
	},
	FlagHTTPProbeProxyPort: &cli.IntFlag{
		Name:    FlagHTTPProbeProxyPort,
		Value:   0,
		Usage:   FlagHTTPProbeProxyPortUsage,
		EnvVars: []string{"DSLIM_HTTP_PROBE_PROXY_PORT"},
	},
	FlagHostExec: &cli.StringSliceFlag{
		Name:    FlagHostExec,
		Value:   cli.NewStringSlice(),
		Usage:   FlagHostExecUsage,
		EnvVars: []string{"DSLIM_HOST_EXEC"},
	},
	FlagHostExecFile: &cli.StringFlag{
		Name:    FlagHostExecFile,
		Value:   "",
		Usage:   FlagHostExecFileUsage,
		EnvVars: []string{"DSLIM_HOST_EXEC_FILE"},
	},
	FlagPublishPort: &cli.StringSliceFlag{
		Name:    FlagPublishPort,
		Value:   cli.NewStringSlice(),
		Usage:   FlagPublishPortUsage,
		EnvVars: []string{"DSLIM_PUBLISH_PORT"},
	},
	FlagPublishExposedPorts: &cli.BoolFlag{
		Name:    FlagPublishExposedPorts,
		Usage:   FlagPublishExposedPortsUsage,
		EnvVars: []string{"DSLIM_PUBLISH_EXPOSED"},
	},
	FlagRunTargetAsUser: &cli.BoolFlag{
		Name:    FlagRunTargetAsUser,
		Value:   true,
		Usage:   FlagRunTargetAsUserUsage,
		EnvVars: []string{"DSLIM_RUN_TAS_USER"},
	},
	FlagShowContainerLogs: &cli.BoolFlag{
		Name:    FlagShowContainerLogs,
		Usage:   FlagShowContainerLogsUsage,
		EnvVars: []string{"DSLIM_SHOW_CLOGS"},
	},
	FlagEnableMondelLogs: &cli.BoolFlag{
		Name:    FlagEnableMondelLogs,
		Usage:   FlagEnableMondelLogsUsage,
		EnvVars: []string{"DSLIM_ENABLE_MONDEL"},
	},
	FlagSensorIPCMode: &cli.StringFlag{
		Name:    FlagSensorIPCMode,
		Value:   "",
		Usage:   FlagSensorIPCModeUsage,
		EnvVars: []string{"DSLIM_SENSOR_IPC_MODE"},
	},
	FlagSensorIPCEndpoint: &cli.StringFlag{
		Name:    FlagSensorIPCEndpoint,
		Value:   "",
		Usage:   FlagSensorIPCEndpointUsage,
		EnvVars: []string{"DSLIM_SENSOR_IPC_ENDPOINT"},
	},
	FlagExec: &cli.StringFlag{
		Name:    FlagExec,
		Value:   "",
		Usage:   FlagExecUsage,
		EnvVars: []string{"DSLIM_RC_EXE"},
	},
	FlagExecFile: &cli.StringFlag{
		Name:    FlagExecFile,
		Value:   "",
		Usage:   FlagExecFileUsage,
		EnvVars: []string{"DSLIM_RC_EXE_FILE"},
	},
	FlagUseLocalMounts: &cli.BoolFlag{
		Name:    FlagUseLocalMounts,
		Usage:   FlagUseLocalMountsUsage,
		EnvVars: []string{"DSLIM_USE_LOCAL_MOUNTS"},
	},
	FlagUseSensorVolume: &cli.StringFlag{
		Name:    FlagUseSensorVolume,
		Value:   "",
		Usage:   FlagUseSensorVolumeUsage,
		EnvVars: []string{"DSLIM_USE_SENSOR_VOLUME"},
	},
	FlagContinueAfter: &cli.StringFlag{
		Name:    FlagContinueAfter,
		Value:   "probe",
		Usage:   FlagContinueAfterUsage,
		EnvVars: []string{"DSLIM_CONTINUE_AFTER"},
	},
	//Container Run Options
	FlagCRORuntime: &cli.StringFlag{
		Name:    FlagCRORuntime,
		Value:   "",
		Usage:   FlagCRORuntimeUsage,
		EnvVars: []string{"DSLIM_CRO_RUNTIME"},
	},
	FlagCROHostConfigFile: &cli.StringFlag{
		Name:    FlagCROHostConfigFile,
		Value:   "",
		Usage:   FlagCROHostConfigFileUsage,
		EnvVars: []string{"DSLIM_CRO_HOST_CONFIG_FILE"},
	},
	FlagCROSysctl: &cli.StringSliceFlag{
		Name:    FlagCROSysctl,
		Value:   cli.NewStringSlice(),
		Usage:   FlagCROSysctlUsage,
		EnvVars: []string{"DSLIM_CRO_SYSCTL"},
	},
	FlagCROShmSize: &cli.Int64Flag{
		Name:    FlagCROShmSize,
		Value:   -1,
		Usage:   FlagCROShmSizeUsage,
		EnvVars: []string{"DSLIM_CRO_SHM_SIZE"},
	},
	FlagUser: &cli.StringFlag{
		Name:    FlagUser,
		Value:   "",
		Usage:   FlagUserUsage,
		EnvVars: []string{"DSLIM_RC_USER"},
	},
	FlagEntrypoint: &cli.StringFlag{
		Name:    FlagEntrypoint,
		Value:   "",
		Usage:   FlagEntrypointUsage,
		EnvVars: []string{"DSLIM_RC_ENTRYPOINT"},
	},
	FlagCmd: &cli.StringFlag{
		Name:    FlagCmd,
		Value:   "",
		Usage:   FlagCmdUsage,
		EnvVars: []string{"DSLIM_RC_CMD"},
	},
	FlagWorkdir: &cli.StringFlag{
		Name:    FlagWorkdir,
		Value:   "",
		Usage:   FlagWorkdirUsage,
		EnvVars: []string{"DSLIM_RC_WORKDIR"},
	},
	FlagEnv: &cli.StringSliceFlag{
		Name:    FlagEnv,
		Value:   cli.NewStringSlice(),
		Usage:   FlagEnvUsage,
		EnvVars: []string{"DSLIM_RC_ENV"},
	},
	FlagEnvFile: &cli.StringFlag{
		Name:    FlagEnvFile,
		Value:   "",
		Usage:   FlagEnvFileUsage,
		EnvVars: []string{"DSLIM_RC_ENV_FILE"},
	},
	FlagLabel: &cli.StringSliceFlag{
		Name:    FlagLabel,
		Value:   cli.NewStringSlice(),
		Usage:   FlagLabelUsage,
		EnvVars: []string{"DSLIM_RC_LABEL"},
	},
	FlagVolume: &cli.StringSliceFlag{
		Name:    FlagVolume,
		Value:   cli.NewStringSlice(),
		Usage:   FlagVolumeUsage,
		EnvVars: []string{"DSLIM_RC_VOLUME"},
	},
	FlagLink: &cli.StringSliceFlag{
		Name:    FlagLink,
		Value:   cli.NewStringSlice(),
		Usage:   FlagLinkUsage,
		EnvVars: []string{"DSLIM_RC_LINK"},
	},
	FlagEtcHostsMap: &cli.StringSliceFlag{
		Name:    FlagEtcHostsMap,
		Value:   cli.NewStringSlice(),
		Usage:   FlagEtcHostsMapUsage,
		EnvVars: []string{"DSLIM_RC_ETC_HOSTS_MAP"},
	},
	FlagContainerDNS: &cli.StringSliceFlag{
		Name:    FlagContainerDNS,
		Value:   cli.NewStringSlice(),
		Usage:   FlagContainerDNSUsage,
		EnvVars: []string{"DSLIM_RC_DNS"},
	},
	FlagContainerDNSSearch: &cli.StringSliceFlag{
		Name:    FlagContainerDNSSearch,
		Value:   cli.NewStringSlice(),
		Usage:   FlagContainerDNSSearchUsage,
		EnvVars: []string{"DSLIM_RC_DNS_SEARCH"},
	},
	FlagHostname: &cli.StringFlag{
		Name:    FlagHostname,
		Value:   "",
		Usage:   FlagHostnameUsage,
		EnvVars: []string{"DSLIM_RC_HOSTNAME"},
	},
	FlagNetwork: &cli.StringFlag{
		Name:    FlagNetwork,
		Value:   "",
		Usage:   FlagNetworkUsage,
		EnvVars: []string{"DSLIM_RC_NET"},
	},
	FlagExpose: &cli.StringSliceFlag{
		Name:    FlagExpose,
		Value:   cli.NewStringSlice(),
		Usage:   FlagExposeUsage,
		EnvVars: []string{"DSLIM_RC_EXPOSE"},
	},
	FlagMount: &cli.StringSliceFlag{
		Name:    FlagMount,
		Value:   cli.NewStringSlice(),
		Usage:   FlagMountUsage,
		EnvVars: []string{"DSLIM_MOUNT"},
	},
	FlagDeleteFatImage: &cli.BoolFlag{
		Name:    FlagDeleteFatImage,
		Usage:   FlagDeleteFatImageUsage,
		EnvVars: []string{"DSLIM_DELETE_FAT"},
	},
	FlagRTAOnbuildBaseImage: &cli.BoolFlag{ //should be disabled by default
		Name:    FlagRTAOnbuildBaseImage,
		Usage:   FlagRTAOnbuildBaseImageUsage,
		EnvVars: []string{"DSLIM_RTA_ONBUILD_BI"},
	},
	FlagRTASourcePT: &cli.BoolFlag{
		Name:    FlagRTASourcePT,
		Value:   true, //all sources are enabled by default
		Usage:   FlagRTASourcePTUsage,
		EnvVars: []string{"DSLIM_RTA_SRC_PT"},
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

func HTTPProbeFlags() []cli.Flag {
	return append([]cli.Flag{
		Cflag(FlagHTTPProbeOff),
		Cflag(FlagHTTPProbe),
		Cflag(FlagHTTPProbeExitOnFailure),
	}, HTTPProbeFlagsBasic()...)
}

func HTTPProbeFlagsBasic() []cli.Flag {
	return []cli.Flag{
		Cflag(FlagHTTPProbeCmd),
		Cflag(FlagHTTPProbeCmdFile),
		Cflag(FlagHTTPProbeStartWait),
		Cflag(FlagHTTPProbeRetryCount),
		Cflag(FlagHTTPProbeRetryWait),
		Cflag(FlagHTTPProbePorts),
		Cflag(FlagHTTPProbeFull),
		Cflag(FlagHTTPProbeCrawl),
		Cflag(FlagHTTPCrawlMaxDepth),
		Cflag(FlagHTTPCrawlMaxPageCount),
		Cflag(FlagHTTPCrawlConcurrency),
		Cflag(FlagHTTPMaxConcurrentCrawlers),
		Cflag(FlagHTTPProbeAPISpec),
		Cflag(FlagHTTPProbeAPISpecFile),
	}
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
