package app

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/system"
	"github.com/docker-slim/docker-slim/pkg/util/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	"github.com/docker-slim/docker-slim/pkg/version"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/dustin/go-humanize"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// DockerSlim app CLI constants
const (
	AppName  = "docker-slim"
	AppUsage = "optimize and secure your Docker containers!"
)

// Command names
const (
	CmdLint         = "lint"
	CmdXray         = "xray"
	CmdProfile      = "profile"
	CmdBuild        = "build"
	CmdContainerize = "containerize"
	CmdVersion      = "version"
	CmdUpdate       = "update"
	CmdHelp         = "help"
)

// Command description / usage info
const (
	CmdLintUsage         = "Lint the target Dockerfile or image"
	CmdXrayUsage         = "Collects fat image information and reverse engineers its Dockerfile"
	CmdProfileUsage      = "Collects fat image information and generates a fat container report"
	CmdBuildUsage        = "Collects fat image information and builds a slim image from it"
	CmdContainerizeUsage = "containerize"
	CmdVersionUsage      = "Shows docker-slim and docker version information"
	CmdUpdateUsage       = "Updates docker-slim"
	CmdHelpUsage         = "Show help info"
)

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
)

// Shared command flag names
const (
	FlagTarget = "target"

	FlagRemoveFileArtifacts = "remove-file-artifacts"
	FlagCopyMetaArtifacts   = "copy-meta-artifacts"

	FlagHTTPProbe              = "http-probe"
	FlagHTTPProbeCmd           = "http-probe-cmd"
	FlagHTTPProbeCmdFile       = "http-probe-cmd-file"
	FlagHTTPProbeRetryCount    = "http-probe-retry-count"
	FlagHTTPProbeRetryWait     = "http-probe-retry-wait"
	FlagHTTPProbePorts         = "http-probe-ports"
	FlagHTTPProbeFull          = "http-probe-full"
	FlagHTTPProbeExitOnFailure = "http-probe-exit-on-failure"

	FlagKeepPerms         = "keep-perms"
	FlagRunTargetAsUser   = "run-target-as-user"
	FlagShowContainerLogs = "show-clogs"

	FlagEntrypoint = "entrypoint"
	FlagCmd        = "cmd"
	FlagWorkdir    = "workdir"
	FlagEnv        = "env"
	FlagExpose     = "expose"

	FlagLink    = "link"
	FlagNetwork = "network"

	FlagHostname           = "hostname"
	FlagEtcHostsMap        = "etc-hosts-map"
	FlagContainerDNS       = "container-dns"
	FlagContainerDNSSearch = "container-dns-search"

	FlagExcludeMounts   = "exclude-mounts"
	FlagExcludePattern  = "exclude-pattern"
	FlagUseLocalMounts  = "use-local-mounts"
	FlagUseSensorVolume = "use-sensor-volume"
	FlagMount           = "mount"
	FlagContinueAfter   = "continue-after"

	FlagPathPerms        = "path-perms"         //shared, but shouldn't be; 'profile' doesn't need it
	FlagPathPermsFile    = "path-perms-file"    //shared, but shouldn't be; 'profile' doesn't need it
	FlagIncludePath      = "include-path"       //shared, but shouldn't be; 'profile' doesn't need it
	FlagIncludePathFile  = "include-path-file"  //shared, but shouldn't be; 'profile' doesn't need it
	FlagIncludeBin       = "include-bin"        //shared, but shouldn't be; 'profile' doesn't need it
	FlagIncludeExe       = "include-exe"        //shared, but shouldn't be; 'profile' doesn't need it
	FlagIncludeShell     = "include-shell"      //shared, but shouldn't be; 'profile' doesn't need it
	FlagKeepTmpArtifacts = "keep-tmp-artifacts" //shared, but shouldn't be; 'profile' doesn't need it
)

// Shared command flag usage info
const (
	FlagTargetUsage = "Target container image (name or ID)"

	FlagRemoveFileArtifactsUsage = "remove file artifacts when command is done"
	FlagCopyMetaArtifactsUsage   = "copy metadata artifacts to the selected location when command is done"

	FlagHTTPProbeUsage              = "Enables HTTP probe"
	FlagHTTPProbeCmdUsage           = "User defined HTTP probes"
	FlagHTTPProbeCmdFileUsage       = "File with user defined HTTP probes"
	FlagHTTPProbeRetryCountUsage    = "Number of retries for each HTTP probe"
	FlagHTTPProbeRetryWaitUsage     = "Number of seconds to wait before retrying HTTP probe (doubles when target is not ready)"
	FlagHTTPProbePortsUsage         = "Explicit list of ports to probe (in the order you want them to be probed)"
	FlagHTTPProbeFullUsage          = "Do full HTTP probe for all selected ports (if false, finish after first successful scan)"
	FlagHTTPProbeExitOnFailureUsage = "Exit when all HTTP probe commands fail"

	FlagKeepPermsUsage         = "Keep artifact permissions as-is"
	FlagRunTargetAsUserUsage   = "Run target app as USER"
	FlagShowContainerLogsUsage = "Show container logs"

	FlagEntrypointUsage = "Override ENTRYPOINT analyzing image"
	FlagCmdUsage        = "Override CMD analyzing image"
	FlagWorkdirUsage    = "Override WORKDIR analyzing image"
	FlagEnvUsage        = "Override ENV analyzing image"
	FlagExposeUsage     = "Use additional EXPOSE instructions analyzing image"

	FlagLinkUsage    = "Add link to another container analyzing image"
	FlagNetworkUsage = "Override default container network settings analyzing image"

	FlagHostnameUsage           = "Override default container hostname analyzing image"
	FlagEtcHostsMapUsage        = "Add a host to IP mapping to /etc/hosts analyzing image"
	FlagContainerDNSUsage       = "Add a dns server analyzing image"
	FlagContainerDNSSearchUsage = "Add a dns search domain for unqualified hostnames analyzing image"

	FlagExcludeMountsUsage   = "Exclude mounted volumes from image"
	FlagExcludePatternUsage  = "Exclude path pattern (Glob/Match in Go and **) from image"
	FlagUseLocalMountsUsage  = "Mount local paths for target container artifact input and output"
	FlagUseSensorVolumeUsage = "Sensor volume name to use"
	FlagMountUsage           = "Mount volume analyzing image"
	FlagContinueAfterUsage   = "Select continue mode: enter | signal | probe | timeout or numberInSeconds"

	FlagPathPermsUsage        = "Set path permissions in optimized image"
	FlagPathPermsFileUsage    = "File with path permissions to set"
	FlagIncludePathUsage      = "Include path from image"
	FlagIncludePathFileUsage  = "File with paths to include from image"
	FlagIncludeBinUsage       = "Include binary from image (executable or shared object using its absolute path)"
	FlagIncludeExeUsage       = "Include executable from image (by executable name)"
	FlagIncludeShellUsage     = "Include basic shell functionality"
	FlagKeepTmpArtifactsUsage = "keep temporary artifacts when command is done"
)

// Build command flag names
const (
	FlagShowBuildLogs = "show-blogs"

	FlagNewEntrypoint = "new-entrypoint"
	FlagNewCmd        = "new-cmd"
	FlagNewExpose     = "new-expose"
	FlagNewWorkdir    = "new-workdir"
	FlagNewEnv        = "new-env"

	FlagTag    = "tag"
	FlagTagFat = "tag-fat"

	FlagImageOverrides = "image-overrides"

	FlagBuildFromDockerfile = "dockerfile"
)

// Build command flag usage info
const (
	FlagShowBuildLogsUsage = "Show build logs"

	FlagNewEntrypointUsage = "New ENTRYPOINT instruction for the optimized image"
	FlagNewCmdUsage        = "New CMD instruction for the optimized image"
	FlagNewExposeUsage     = "New EXPOSE instructions for the optimized image"
	FlagNewWorkdirUsage    = "New WORKDIR instruction for the optimized image"
	FlagNewEnvUsage        = "New ENV instructions for the optimized image"

	FlagTagUsage    = "Custom tag for the generated image"
	FlagTagFatUsage = "Custom tag for the fat image built from Dockerfile"

	FlagImageOverridesUsage = "Use overrides in generated image"

	FlagBuildFromDockerfileUsage = "The source Dockerfile name to build the fat image before it's optimized"
)

// Xray command flag names
const (
	FlagChanges          = "changes"
	FlagLayer            = "layer"
	FlagAddImageManifest = "add-image-manifest"
	FlagAddImageConfig   = "add-image-config"
)

// Xray command flag usage info
const (
	FlagChangesUsage          = "Show layer change details for the selected change type (values: none, all, delete, modify, add)"
	FlagLayerUsage            = "Show details for the selected layer (using layer index or ID)"
	FlagAddImageManifestUsage = "Add raw image manifest to the command execution report file"
	FlagAddImageConfigUsage   = "Add raw image config object to the command execution report file"
)

// Update command flag names
const (
	FlagShowProgress = "show-progress"
)

// Update command flag usage info
const (
	FlagShowProgressUsage = "show progress when the release package is downloaded"
)

type InteractiveApp struct {
	appPrompt   *prompt.Prompt
	fpCompleter completer.FilePathCompleter
	app         *cli.App
	dclient     *dockerapi.Client
}

func NewInteractiveApp(app *cli.App, gparams *commands.GenericParams) *InteractiveApp {
	ia := InteractiveApp{
		app: app,
		fpCompleter: completer.FilePathCompleter{
			IgnoreCase: true,
		},
	}

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("docker-slim: info=docker.connect.error message='%s'\n", exitMsg)
		fmt.Printf("docker-slim: state=exited version=%s location='%s'\n", version.Current(), fsutil.ExeDir())
		os.Exit(-777)
	}
	errutil.FailOn(err)

	ia.dclient = client

	ia.appPrompt = prompt.New(
		ia.execute,
		ia.complete,
		prompt.OptionTitle(fmt.Sprintf("%s: interactive prompt", AppName)),
		prompt.OptionPrefix(">>> "),
		prompt.OptionInputTextColor(prompt.Red),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
	)

	return &ia
}

func (ia *InteractiveApp) execute(command string) {
	command = strings.TrimSpace(command)
	parts, err := shlex.Split(command)
	if err != nil {
		log.Fatal(err)
	}

	if len(parts) == 0 {
		return
	}

	if parts[0] == "exit" {
		os.Exit(0)
	}

	partsCount := len(parts)
	for i := 0; i < partsCount; i++ {
		if parts[i] == "" {
			continue
		}
		if strings.HasPrefix(parts[i], "--") &&
			(i+1) < partsCount &&
			(parts[i+1] == "true" || parts[i+1] == "false") {
			parts[i] = fmt.Sprintf("%s=%s", parts[i], parts[i+1])
			parts[i+1] = ""
		}
	}

	args := append([]string{AppName}, parts...)

	if err := ia.app.Run(args); err != nil {
		log.Fatal(err)
	}
}

func (ia *InteractiveApp) complete(params prompt.Document) []prompt.Suggest {
	allParamsLine := params.TextBeforeCursor()

	allParamsLine = strings.TrimSpace(allParamsLine)
	if allParamsLine == "" {
		return append(commandSuggestions, globalFlagSuggestions...)
	}

	currentToken := params.GetWordBeforeCursor()

	allTokens := strings.Split(allParamsLine, " ")

	var prevToken string
	prevTokenIdx := -1
	tokenCount := len(allTokens)

	if tokenCount > 0 {
		if currentToken == "" {
			//currentToken 'points' past allTokens[last]
			prevTokenIdx = tokenCount - 1
			prevToken = allTokens[prevTokenIdx]
		} else {
			//currentToken 'points' to allTokens[last]
			if tokenCount >= 2 {
				prevTokenIdx = tokenCount - 2
				prevToken = allTokens[prevTokenIdx]
			}
		}
	}

	if prevToken == "" {
		suggestions := append(commandSuggestions, globalFlagSuggestions...)
		return prompt.FilterHasPrefix(suggestions, currentToken, true)
	}

	commandTokenIdx := -1
	for i := 0; i <= prevTokenIdx; i++ {
		if !strings.HasPrefix(allTokens[i], "--") {
			commandTokenIdx = i
			break
		}
	}

	if commandTokenIdx == -1 {
		suggestions := append(commandSuggestions, globalFlagSuggestions...)
		return prompt.FilterHasPrefix(suggestions, currentToken, true)
	}

	commandToken := allTokens[commandTokenIdx]

	if commandTokenIdx == (tokenCount - 1) {
		if currentToken != "" {
			//currentToken still points to the command token
			return prompt.FilterHasPrefix(commandSuggestions, currentToken, true)
		} else {
			//need to return the command flag suggestions
			if cmdSpec, ok := cmdSpecs[commandToken]; ok {
				return prompt.FilterHasPrefix(cmdSpec.suggestions.Names, currentToken, true)
			} else {
				return []prompt.Suggest{}
			}
		}
	}

	cmdSpec, ok := cmdSpecs[commandToken]
	if !ok && cmdSpec.suggestions != nil {
		return []prompt.Suggest{}
	}

	if strings.HasPrefix(prevToken, "--") {
		if completeValue, ok := cmdSpec.suggestions.Values[prevToken]; ok && completeValue != nil {
			return completeValue(ia, currentToken, params)
		}
	} else {
		return prompt.FilterHasPrefix(cmdSpec.suggestions.Names, currentToken, true)
	}

	return []prompt.Suggest{}
}

func (ia *InteractiveApp) Run() {
	ia.appPrompt.Run()
}

var commandSuggestions = []prompt.Suggest{
	{Text: CmdXray, Description: CmdXrayUsage},
	{Text: CmdBuild, Description: CmdBuildUsage},
	{Text: CmdProfile, Description: CmdProfileUsage},
	{Text: CmdLint, Description: CmdLintUsage},
	{Text: CmdVersion, Description: CmdVersionUsage},
	{Text: CmdUpdate, Description: CmdUpdateUsage},
	{Text: CmdHelp, Description: CmdHelpUsage},
	{Text: "exit", Description: "Exit app"},
}

var globalFlagSuggestions = []prompt.Suggest{
	{Text: fullFlagName(FlagStatePath), Description: FlagStatePathUsage},
	{Text: fullFlagName(FlagCommandReport), Description: FlagCommandReportUsage},
	{Text: fullFlagName(FlagDebug), Description: FlagDebugUsage},
	{Text: fullFlagName(FlagVerbose), Description: FlagVerboseUsage},
	{Text: fullFlagName(FlagLogLevel), Description: FlagLogLevelUsage},
	{Text: fullFlagName(FlagLog), Description: FlagLogUsage},
	{Text: fullFlagName(FlagLogFormat), Description: FlagLogFormatUsage},
	{Text: fullFlagName(FlagUseTLS), Description: FlagUseTLSUsage},
	{Text: fullFlagName(FlagVerifyTLS), Description: FlagVerifyTLSUsage},
	{Text: fullFlagName(FlagTLSCertPath), Description: FlagTLSCertPathUsage},
	{Text: fullFlagName(FlagHost), Description: FlagHostUsage},
	{Text: fullFlagName(FlagArchiveState), Description: FlagArchiveStateUsage},
	{Text: fullFlagName(FlagInContainer), Description: FlagInContainerUsage},
	{Text: fullFlagName(FlagCheckVersion), Description: FlagCheckVersionUsage},
}

func fullFlagName(name string) string {
	return fmt.Sprintf("--%s", name)
}

var boolValues = []prompt.Suggest{
	{Text: "false", Description: "default"},
	{Text: "true"},
}

var tboolValues = []prompt.Suggest{
	{Text: "true", Description: "default"},
	{Text: "false"},
}

var layerChangeValues = []prompt.Suggest{
	{Text: "none", Description: "Don't show any file system change details in image layers"},
	{Text: "all", Description: "Show all file system change details in image layers"},
	{Text: "delete", Description: "Show only 'delete' file system change details in image layers"},
	{Text: "modify", Description: "Show only 'modify' file system change details in image layers"},
	{Text: "add", Description: "Show only 'add' file system change details in image layers"},
}

var continueAfterValues = []prompt.Suggest{
	{Text: "probe", Description: "Automatically continue after the HTTP probe is finished running"},
	{Text: "enter", Description: "Use the <enter> key to indicate you that you are done using the container"},
	{Text: "signal", Description: "Use SIGUSR1 to signal that you are done using the container"},
	{Text: "timeout", Description: "Automatically continue after the default timeout (60 seconds)"},
	{Text: "<seconds>", Description: "Enter the number of seconds to wait instead of <seconds>"},
}

func completeProgress(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	switch runtime.GOOS {
	case "darwin":
		return completeTBool(ia, token, params)
	default:
		return completeBool(ia, token, params)
	}
}

func completeBool(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(boolValues, token, true)
}

func completeTBool(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(tboolValues, token, true)
}

func completeLayerChanges(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(layerChangeValues, token, true)
}

func completeContinueAfter(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(continueAfterValues, token, true)
}

func completeTarget(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	images, err := dockerutil.ListImages(ia.dclient, "")
	if err != nil {
		log.Errorf("completeTarget(%q): error - %v", token, err)
		return []prompt.Suggest{}
	}

	var values []prompt.Suggest
	for name, info := range images {
		description := fmt.Sprintf("size=%v created=%v id=%v",
			humanize.Bytes(uint64(info.Size)),
			time.Unix(info.Created, 0).Format(time.RFC3339),
			info.ID)

		entry := prompt.Suggest{
			Text:        name,
			Description: description,
		}

		values = append(values, entry)
	}

	return prompt.FilterContains(values, token, true)
}

func completeVolume(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	names, err := dockerutil.ListVolumes(ia.dclient, token)
	if err != nil {
		log.Errorf("completeVolume(%q): error - %v", token, err)
		return []prompt.Suggest{}
	}

	var values []prompt.Suggest
	for _, name := range names {
		entry := prompt.Suggest{
			Text: name,
		}

		values = append(values, entry)
	}

	return prompt.FilterContains(values, token, true)
}

func completeNetwork(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	names, err := dockerutil.ListNetworks(ia.dclient, token)
	if err != nil {
		log.Errorf("completeNetwork(%q): error - %v", token, err)
		return []prompt.Suggest{}
	}

	var values []prompt.Suggest
	for _, name := range names {
		entry := prompt.Suggest{
			Text: name,
		}

		values = append(values, entry)
	}

	return prompt.FilterContains(values, token, true)
}

func completeFile(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return ia.fpCompleter.Complete(params)
}

type CompleteValue func(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest

type flagSuggestions struct {
	Names  []prompt.Suggest
	Values map[string]CompleteValue
}

type cmdSpec struct {
	name        string
	alias       string
	usage       string
	suggestions *flagSuggestions
}

var cmdSpecs = map[string]cmdSpec{
	CmdHelp: {
		name:  CmdHelp,
		alias: "h",
		usage: CmdHelpUsage,
	},
	CmdLint: {
		name:  CmdLint,
		alias: "l",
		usage: CmdLintUsage,
		suggestions: &flagSuggestions{
			Names:  []prompt.Suggest{},
			Values: map[string]CompleteValue{},
		},
	},
	CmdXray: {
		name:  CmdXray,
		alias: "x",
		usage: CmdXrayUsage,
		suggestions: &flagSuggestions{
			Names: []prompt.Suggest{
				{Text: fullFlagName(FlagTarget), Description: FlagTargetUsage},
				{Text: fullFlagName(FlagChanges), Description: FlagChangesUsage},
				{Text: fullFlagName(FlagLayer), Description: FlagLayerUsage},
				{Text: fullFlagName(FlagAddImageManifest), Description: FlagAddImageManifestUsage},
				{Text: fullFlagName(FlagAddImageConfig), Description: FlagAddImageConfigUsage},
				{Text: fullFlagName(FlagRemoveFileArtifacts), Description: FlagRemoveFileArtifactsUsage},
			},
			Values: map[string]CompleteValue{
				fullFlagName(FlagTarget):              completeTarget,
				fullFlagName(FlagChanges):             completeLayerChanges,
				fullFlagName(FlagAddImageManifest):    completeBool,
				fullFlagName(FlagAddImageConfig):      completeBool,
				fullFlagName(FlagRemoveFileArtifacts): completeBool,
			},
		},
	},
	CmdProfile: {
		name:  CmdProfile,
		alias: "p",
		usage: CmdProfileUsage,
		suggestions: &flagSuggestions{
			Names: []prompt.Suggest{
				{Text: fullFlagName(FlagTarget), Description: FlagTargetUsage},
				{Text: fullFlagName(FlagShowContainerLogs), Description: FlagShowContainerLogsUsage},
				{Text: fullFlagName(FlagHTTPProbe), Description: FlagHTTPProbeUsage},
				{Text: fullFlagName(FlagHTTPProbeCmd), Description: FlagHTTPProbeCmdUsage},
				{Text: fullFlagName(FlagHTTPProbeCmdFile), Description: FlagHTTPProbeCmdFileUsage},
				{Text: fullFlagName(FlagHTTPProbeRetryCount), Description: FlagHTTPProbeRetryCountUsage},
				{Text: fullFlagName(FlagHTTPProbeRetryWait), Description: FlagHTTPProbeRetryWaitUsage},
				{Text: fullFlagName(FlagHTTPProbePorts), Description: FlagHTTPProbePortsUsage},
				{Text: fullFlagName(FlagHTTPProbeFull), Description: FlagHTTPProbeFullUsage},
				{Text: fullFlagName(FlagHTTPProbeExitOnFailure), Description: FlagHTTPProbeExitOnFailureUsage},
				{Text: fullFlagName(FlagKeepPerms), Description: FlagKeepPermsUsage},
				{Text: fullFlagName(FlagRunTargetAsUser), Description: FlagRunTargetAsUserUsage},
				{Text: fullFlagName(FlagCopyMetaArtifacts), Description: FlagCopyMetaArtifactsUsage},
				{Text: fullFlagName(FlagRemoveFileArtifacts), Description: FlagRemoveFileArtifactsUsage},
				{Text: fullFlagName(FlagEntrypoint), Description: FlagEntrypointUsage},
				{Text: fullFlagName(FlagCmd), Description: FlagCmdUsage},
				{Text: fullFlagName(FlagWorkdir), Description: FlagWorkdirUsage},
				{Text: fullFlagName(FlagEnv), Description: FlagEnvUsage},
				{Text: fullFlagName(FlagLink), Description: FlagLinkUsage},
				{Text: fullFlagName(FlagEtcHostsMap), Description: FlagEtcHostsMapUsage},
				{Text: fullFlagName(FlagContainerDNS), Description: FlagContainerDNSUsage},
				{Text: fullFlagName(FlagContainerDNSSearch), Description: FlagContainerDNSSearchUsage},
				{Text: fullFlagName(FlagNetwork), Description: FlagNetworkUsage},
				{Text: fullFlagName(FlagHostname), Description: FlagHostnameUsage},
				{Text: fullFlagName(FlagExpose), Description: FlagExposeUsage},
				{Text: fullFlagName(FlagExcludeMounts), Description: FlagExcludeMountsUsage},
				{Text: fullFlagName(FlagExcludePattern), Description: FlagExcludePatternUsage},
				{Text: fullFlagName(FlagPathPerms), Description: FlagPathPermsUsage},
				{Text: fullFlagName(FlagPathPermsFile), Description: FlagPathPermsFileUsage},
				{Text: fullFlagName(FlagIncludePath), Description: FlagIncludePathUsage},
				{Text: fullFlagName(FlagIncludePathFile), Description: FlagIncludePathFileUsage},
				{Text: fullFlagName(FlagIncludeBin), Description: FlagIncludeBinUsage},
				{Text: fullFlagName(FlagIncludeExe), Description: FlagIncludeExeUsage},
				{Text: fullFlagName(FlagIncludeShell), Description: FlagIncludeShellUsage},
				{Text: fullFlagName(FlagMount), Description: FlagMountUsage},
				{Text: fullFlagName(FlagContinueAfter), Description: FlagContinueAfterUsage},
				{Text: fullFlagName(FlagUseLocalMounts), Description: FlagUseLocalMountsUsage},
				{Text: fullFlagName(FlagUseSensorVolume), Description: FlagUseSensorVolumeUsage},
				{Text: fullFlagName(FlagKeepTmpArtifacts), Description: FlagKeepTmpArtifactsUsage},
			},
			Values: map[string]CompleteValue{
				fullFlagName(FlagTarget):              completeTarget,
				fullFlagName(FlagShowContainerLogs):   completeBool,
				fullFlagName(FlagHTTPProbe):           completeTBool,
				fullFlagName(FlagHTTPProbeCmdFile):    completeFile,
				fullFlagName(FlagHTTPProbeFull):       completeBool,
				fullFlagName(FlagHTTPProbeExitOnFailure): completeBool,
				fullFlagName(FlagKeepPerms):           completeTBool,
				fullFlagName(FlagRunTargetAsUser):     completeTBool,
				fullFlagName(FlagRemoveFileArtifacts): completeBool,
				fullFlagName(FlagNetwork):             completeNetwork,
				fullFlagName(FlagExcludeMounts):       completeTBool,
				fullFlagName(FlagPathPermsFile):       completeFile,
				fullFlagName(FlagIncludePathFile):     completeFile,
				fullFlagName(FlagIncludeShell):        completeBool,
				fullFlagName(FlagContinueAfter):       completeContinueAfter,
				fullFlagName(FlagUseLocalMounts):      completeBool,
				fullFlagName(FlagUseSensorVolume):     completeVolume,
				fullFlagName(FlagKeepTmpArtifacts):    completeBool,
			},
		},
	},
	CmdBuild: {
		name:  CmdBuild,
		alias: "b",
		usage: CmdBuildUsage,
		suggestions: &flagSuggestions{
			Names: []prompt.Suggest{
				{Text: fullFlagName(FlagTarget), Description: FlagTargetUsage},
				{Text: fullFlagName(FlagBuildFromDockerfile), Description: FlagBuildFromDockerfileUsage},
				{Text: fullFlagName(FlagShowBuildLogs), Description: FlagShowBuildLogsUsage},
				{Text: fullFlagName(FlagShowContainerLogs), Description: FlagShowContainerLogsUsage},
				{Text: fullFlagName(FlagHTTPProbe), Description: FlagHTTPProbeUsage},
				{Text: fullFlagName(FlagHTTPProbeCmd), Description: FlagHTTPProbeCmdUsage},
				{Text: fullFlagName(FlagHTTPProbeCmdFile), Description: FlagHTTPProbeCmdFileUsage},
				{Text: fullFlagName(FlagHTTPProbeRetryCount), Description: FlagHTTPProbeRetryCountUsage},
				{Text: fullFlagName(FlagHTTPProbeRetryWait), Description: FlagHTTPProbeRetryWaitUsage},
				{Text: fullFlagName(FlagHTTPProbePorts), Description: FlagHTTPProbePortsUsage},
				{Text: fullFlagName(FlagHTTPProbeFull), Description: FlagHTTPProbeFullUsage},
				{Text: fullFlagName(FlagHTTPProbeExitOnFailure), Description: FlagHTTPProbeExitOnFailureUsage},
				{Text: fullFlagName(FlagKeepPerms), Description: FlagKeepPermsUsage},
				{Text: fullFlagName(FlagRunTargetAsUser), Description: FlagRunTargetAsUserUsage},
				{Text: fullFlagName(FlagCopyMetaArtifacts), Description: FlagCopyMetaArtifactsUsage},
				{Text: fullFlagName(FlagRemoveFileArtifacts), Description: FlagRemoveFileArtifactsUsage},
				{Text: fullFlagName(FlagTag), Description: FlagTagUsage},
				{Text: fullFlagName(FlagTagFat), Description: FlagTagFatUsage},
				{Text: fullFlagName(FlagImageOverrides), Description: FlagImageOverridesUsage},
				{Text: fullFlagName(FlagEntrypoint), Description: FlagEntrypointUsage},
				{Text: fullFlagName(FlagCmd), Description: FlagCmdUsage},
				{Text: fullFlagName(FlagWorkdir), Description: FlagWorkdirUsage},
				{Text: fullFlagName(FlagEnv), Description: FlagEnvUsage},
				{Text: fullFlagName(FlagLink), Description: FlagLinkUsage},
				{Text: fullFlagName(FlagEtcHostsMap), Description: FlagEtcHostsMapUsage},
				{Text: fullFlagName(FlagContainerDNS), Description: FlagContainerDNSUsage},
				{Text: fullFlagName(FlagContainerDNSSearch), Description: FlagContainerDNSSearchUsage},
				{Text: fullFlagName(FlagNetwork), Description: FlagNetworkUsage},
				{Text: fullFlagName(FlagHostname), Description: FlagHostnameUsage},
				{Text: fullFlagName(FlagExpose), Description: FlagExposeUsage},
				{Text: fullFlagName(FlagNewEntrypoint), Description: FlagNewEntrypointUsage},
				{Text: fullFlagName(FlagNewCmd), Description: FlagNewCmdUsage},
				{Text: fullFlagName(FlagNewExpose), Description: FlagNewExposeUsage},
				{Text: fullFlagName(FlagNewWorkdir), Description: FlagNewWorkdirUsage},
				{Text: fullFlagName(FlagNewEnv), Description: FlagNewEnvUsage},
				{Text: fullFlagName(FlagExcludeMounts), Description: FlagExcludeMountsUsage},
				{Text: fullFlagName(FlagExcludePattern), Description: FlagExcludePatternUsage},
				{Text: fullFlagName(FlagPathPerms), Description: FlagPathPermsUsage},
				{Text: fullFlagName(FlagPathPermsFile), Description: FlagPathPermsFileUsage},
				{Text: fullFlagName(FlagIncludePath), Description: FlagIncludePathUsage},
				{Text: fullFlagName(FlagIncludePathFile), Description: FlagIncludePathFileUsage},
				{Text: fullFlagName(FlagIncludeBin), Description: FlagIncludeBinUsage},
				{Text: fullFlagName(FlagIncludeExe), Description: FlagIncludeExeUsage},
				{Text: fullFlagName(FlagIncludeShell), Description: FlagIncludeShellUsage},
				{Text: fullFlagName(FlagMount), Description: FlagMountUsage},
				{Text: fullFlagName(FlagContinueAfter), Description: FlagContinueAfterUsage},
				{Text: fullFlagName(FlagUseLocalMounts), Description: FlagUseLocalMountsUsage},
				{Text: fullFlagName(FlagUseSensorVolume), Description: FlagUseSensorVolumeUsage},
				{Text: fullFlagName(FlagKeepTmpArtifacts), Description: FlagKeepTmpArtifactsUsage},
			},
			Values: map[string]CompleteValue{
				fullFlagName(FlagTarget):              completeTarget,
				fullFlagName(FlagShowBuildLogs):       completeBool,
				fullFlagName(FlagShowContainerLogs):   completeBool,
				fullFlagName(FlagHTTPProbe):           completeTBool,
				fullFlagName(FlagHTTPProbeCmdFile):    completeFile,
				fullFlagName(FlagHTTPProbeFull):       completeBool,
				fullFlagName(FlagHTTPProbeExitOnFailure): completeBool,
				fullFlagName(FlagKeepPerms):           completeTBool,
				fullFlagName(FlagRunTargetAsUser):     completeTBool,
				fullFlagName(FlagRemoveFileArtifacts): completeBool,
				fullFlagName(FlagNetwork):             completeNetwork,
				fullFlagName(FlagExcludeMounts):       completeTBool,
				fullFlagName(FlagPathPermsFile):       completeFile,
				fullFlagName(FlagIncludePathFile):     completeFile,
				fullFlagName(FlagIncludeShell):        completeBool,
				fullFlagName(FlagContinueAfter):       completeContinueAfter,
				fullFlagName(FlagUseLocalMounts):      completeBool,
				fullFlagName(FlagUseSensorVolume):     completeVolume,
				fullFlagName(FlagKeepTmpArtifacts):    completeBool,
			},
		},
	},
	CmdContainerize: {
		name:  CmdContainerize,
		alias: "c",
		usage: "Containerize the target artifacts",
	},
	CmdVersion: {
		name:  CmdVersion,
		alias: "v",
		usage: CmdVersionUsage,
	},
	CmdUpdate: {
		name:  CmdUpdate,
		alias: "u",
		usage: CmdUpdateUsage,
		suggestions: &flagSuggestions{
			Names: []prompt.Suggest{
				{Text: fullFlagName(FlagShowProgress), Description: FlagShowProgressUsage},
			},
			Values: map[string]CompleteValue{
				fullFlagName(FlagShowProgress): completeProgress,
			},
		},
	},
}

var app *cli.App

func globalFlags() []cli.Flag {
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
		cli.BoolFlag{
			Name:  FlagInContainer,
			Usage: "DockerSlim is running in a container",
		},
		cli.StringFlag{
			Name:  FlagArchiveState,
			Value: "",
			Usage: "archive DockerSlim state to the selected Docker volume (default volume - docker-slim-state). By default, enabled when DockerSlim is running in a container (disabled otherwise). Set it to \"off\" to disable explicitly.",
		},
	}
}

func globalCommandFlagValues(ctx *cli.Context) (*commands.GenericParams, error) {
	values := commands.GenericParams{
		CheckVersion:   ctx.GlobalBool(FlagCheckVersion),
		Debug:          ctx.GlobalBool(FlagDebug),
		StatePath:      ctx.GlobalString(FlagStatePath),
		ReportLocation: ctx.GlobalString(FlagCommandReport),
	}

	if values.ReportLocation == "off" {
		values.ReportLocation = ""
	}

	values.InContainer, values.IsDSImage = isInContainer(ctx.GlobalBool(FlagInContainer))
	values.ArchiveState = archiveState(ctx.GlobalString(FlagArchiveState), values.InContainer)

	values.ClientConfig = getDockerClientConfig(ctx)

	return &values, nil
}

func init() {
	app = cli.NewApp()
	app.Version = version.Current()
	app.Name = AppName
	app.Usage = AppUsage
	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Printf("unknown command - %v \n\n", command)
		cli.ShowAppHelp(ctx)
	}

	app.Flags = globalFlags()

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

	app.Action = func(ctx *cli.Context) error {
		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		ia := NewInteractiveApp(app, gcvalues)
		ia.Run()
		return nil
	}

	doTargetFlag := cli.StringFlag{
		Name:   FlagTarget,
		Value:  "",
		Usage:  FlagTargetUsage,
		EnvVar: "DSLIM_TARGET",
	}

	doRemoveFileArtifactsFlag := cli.BoolFlag{
		Name:   FlagRemoveFileArtifacts,
		Usage:  FlagRemoveFileArtifactsUsage,
		EnvVar: "DSLIM_RM_FILE_ARTIFACTS",
	}

	doCopyMetaArtifactsFlag := cli.StringFlag{
		Name:   FlagCopyMetaArtifacts,
		Usage:  FlagCopyMetaArtifactsUsage,
		EnvVar: "DSLIM_CP_META_ARTIFACTS",
	}

	//true by default
	doHTTPProbeFlag := cli.BoolTFlag{
		Name:   FlagHTTPProbe,
		Usage:  FlagHTTPProbeUsage,
		EnvVar: "DSLIM_HTTP_PROBE",
	}

	doHTTPProbeCmdFlag := cli.StringSliceFlag{
		Name:   FlagHTTPProbeCmd,
		Value:  &cli.StringSlice{},
		Usage:  FlagHTTPProbeCmdUsage,
		EnvVar: "DSLIM_HTTP_PROBE_CMD",
	}

	doHTTPProbeCmdFileFlag := cli.StringFlag{
		Name:   FlagHTTPProbeCmdFile,
		Value:  "",
		Usage:  FlagHTTPProbeCmdFileUsage,
		EnvVar: "DSLIM_HTTP_PROBE_CMD_FILE",
	}

	doHTTPProbeRetryCountFlag := cli.IntFlag{
		Name:   FlagHTTPProbeRetryCount,
		Value:  5,
		Usage:  FlagHTTPProbeRetryCountUsage,
		EnvVar: "DSLIM_HTTP_PROBE_RETRY_COUNT",
	}

	doHTTPProbeRetryWaitFlag := cli.IntFlag{
		Name:   FlagHTTPProbeRetryWait,
		Value:  8,
		Usage:  FlagHTTPProbeRetryWaitUsage,
		EnvVar: "DSLIM_HTTP_PROBE_RETRY_WAIT",
	}

	doHTTPProbePortsFlag := cli.StringFlag{
		Name:   FlagHTTPProbePorts,
		Value:  "",
		Usage:  FlagHTTPProbePortsUsage,
		EnvVar: "DSLIM_HTTP_PROBE_PORTS",
	}

	doHTTPProbeFullFlag := cli.BoolFlag{
		Name:   FlagHTTPProbeFull,
		Usage:  FlagHTTPProbeFullUsage,
		EnvVar: "DSLIM_HTTP_PROBE_FULL",
	}

	doHTTPProbeExitOnFailureFlag := cli.BoolFlag{
		Name:   FlagHTTPProbeExitOnFailure,
		Usage:  FlagHTTPProbeExitOnFailureUsage,
		EnvVar: "DSLIM_HTTP_PROBE_EXIT_ON_FAILURE",
	}

	doKeepPermsFlag := cli.BoolTFlag{
		Name:   FlagKeepPerms,
		Usage:  FlagKeepPermsUsage,
		EnvVar: "DSLIM_KEEP_PERMS",
	}

	doRunTargetAsUserFlag := cli.BoolTFlag{
		Name:   FlagRunTargetAsUser,
		Usage:  FlagRunTargetAsUserUsage,
		EnvVar: "DSLIM_RUN_TAS_USER",
	}

	doShowContainerLogsFlag := cli.BoolFlag{
		Name:   FlagShowContainerLogs,
		Usage:  FlagShowContainerLogsUsage,
		EnvVar: "DSLIM_SHOW_CLOGS",
	}

	doShowBuildLogsFlag := cli.BoolFlag{
		Name:   FlagShowBuildLogs,
		Usage:  FlagShowBuildLogsUsage,
		EnvVar: "DSLIM_SHOW_BLOGS",
	}

	doUseNewEntrypointFlag := cli.StringFlag{
		Name:   FlagNewEntrypoint,
		Value:  "",
		Usage:  FlagNewEntrypointUsage,
		EnvVar: "DSLIM_NEW_ENTRYPOINT",
	}

	doUseNewCmdFlag := cli.StringFlag{
		Name:   FlagNewCmd,
		Value:  "",
		Usage:  FlagNewCmdUsage,
		EnvVar: "DSLIM_NEW_CMD",
	}

	doUseNewExposeFlag := cli.StringSliceFlag{
		Name:   FlagNewExpose,
		Value:  &cli.StringSlice{},
		Usage:  FlagNewExposeUsage,
		EnvVar: "DSLIM_NEW_EXPOSE",
	}

	doUseNewWorkdirFlag := cli.StringFlag{
		Name:   FlagNewWorkdir,
		Value:  "",
		Usage:  FlagNewWorkdirUsage,
		EnvVar: "DSLIM_NEW_WORKDIR",
	}

	doUseNewEnvFlag := cli.StringSliceFlag{
		Name:   FlagNewEnv,
		Value:  &cli.StringSlice{},
		Usage:  FlagNewEnvUsage,
		EnvVar: "DSLIM_NEW_ENV",
	}

	doUseEntrypointFlag := cli.StringFlag{
		Name:   FlagEntrypoint,
		Value:  "",
		Usage:  FlagEntrypointUsage,
		EnvVar: "DSLIM_ENTRYPOINT",
	}

	doUseCmdFlag := cli.StringFlag{
		Name:   FlagCmd,
		Value:  "",
		Usage:  FlagCmdUsage,
		EnvVar: "DSLIM_TARGET_CMD",
	}

	doUseWorkdirFlag := cli.StringFlag{
		Name:   FlagWorkdir,
		Value:  "",
		Usage:  FlagWorkdirUsage,
		EnvVar: "DSLIM_TARGET_WORKDIR",
	}

	doUseEnvFlag := cli.StringSliceFlag{
		Name:   FlagEnv,
		Value:  &cli.StringSlice{},
		Usage:  FlagEnvUsage,
		EnvVar: "DSLIM_TARGET_ENV",
	}

	doUseLinkFlag := cli.StringSliceFlag{
		Name:   FlagLink,
		Value:  &cli.StringSlice{},
		Usage:  FlagLinkUsage,
		EnvVar: "DSLIM_TARGET_LINK",
	}

	doUseEtcHostsMapFlag := cli.StringSliceFlag{
		Name:   FlagEtcHostsMap,
		Value:  &cli.StringSlice{},
		Usage:  FlagEtcHostsMapUsage,
		EnvVar: "DSLIM_TARGET_ETC_HOSTS_MAP",
	}

	doUseContainerDNSFlag := cli.StringSliceFlag{
		Name:   FlagContainerDNS,
		Value:  &cli.StringSlice{},
		Usage:  FlagContainerDNSUsage,
		EnvVar: "DSLIM_TARGET_DNS",
	}

	doUseContainerDNSSearchFlag := cli.StringSliceFlag{
		Name:   FlagContainerDNSSearch,
		Value:  &cli.StringSlice{},
		Usage:  FlagContainerDNSSearchUsage,
		EnvVar: "DSLIM_TARGET_DNS_SEARCH",
	}

	doUseHostnameFlag := cli.StringFlag{
		Name:   FlagHostname,
		Value:  "",
		Usage:  FlagHostnameUsage,
		EnvVar: "DSLIM_TARGET_HOSTNAME",
	}

	doUseNetworkFlag := cli.StringFlag{
		Name:   FlagNetwork,
		Value:  "",
		Usage:  FlagNetworkUsage,
		EnvVar: "DSLIM_TARGET_NET",
	}

	doUseExposeFlag := cli.StringSliceFlag{
		Name:   FlagExpose,
		Value:  &cli.StringSlice{},
		Usage:  FlagExposeUsage,
		EnvVar: "DSLIM_TARGET_EXPOSE",
	}

	//true by default
	doExcludeMountsFlag := cli.BoolTFlag{
		Name:   FlagExcludeMounts,
		Usage:  FlagExcludeMountsUsage,
		EnvVar: "DSLIM_EXCLUDE_MOUNTS",
	}

	doExcludePatternFlag := cli.StringSliceFlag{
		Name:   FlagExcludePattern,
		Value:  &cli.StringSlice{},
		Usage:  FlagExcludePatternUsage,
		EnvVar: "DSLIM_EXCLUDE_PATTERN",
	}

	doSetPathPermsFlag := cli.StringSliceFlag{
		Name:   FlagPathPerms,
		Value:  &cli.StringSlice{},
		Usage:  FlagPathPermsUsage,
		EnvVar: "DSLIM_PATH_PERMS",
	}

	doSetPathPermsFileFlag := cli.StringFlag{
		Name:   FlagPathPermsFile,
		Value:  "",
		Usage:  FlagPathPermsFileUsage,
		EnvVar: "DSLIM_PATH_PERMS_FILE",
	}

	doIncludePathFlag := cli.StringSliceFlag{
		Name:   FlagIncludePath,
		Value:  &cli.StringSlice{},
		Usage:  FlagIncludePathUsage,
		EnvVar: "DSLIM_INCLUDE_PATH",
	}

	doIncludePathFileFlag := cli.StringFlag{
		Name:   FlagIncludePathFile,
		Value:  "",
		Usage:  FlagIncludePathFileUsage,
		EnvVar: "DSLIM_INCLUDE_PATH_FILE",
	}

	doIncludeBinFlag := cli.StringSliceFlag{
		Name:   FlagIncludeBin,
		Value:  &cli.StringSlice{},
		Usage:  FlagIncludeBinUsage,
		EnvVar: "DSLIM_INCLUDE_BIN",
	}

	doIncludeExeFlag := cli.StringSliceFlag{
		Name:   FlagIncludeExe,
		Value:  &cli.StringSlice{},
		Usage:  FlagIncludeExeUsage,
		EnvVar: "DSLIM_INCLUDE_EXE",
	}

	doIncludeShellFlag := cli.BoolFlag{
		Name:   FlagIncludeShell,
		Usage:  FlagIncludeShellUsage,
		EnvVar: "DSLIM_INCLUDE_SHELL",
	}

	doKeepTmpArtifactsFlag := cli.BoolFlag{
		Name:   FlagKeepTmpArtifacts,
		Usage:  FlagKeepTmpArtifactsUsage,
		EnvVar: "DSLIM_KEEP_TMP_ARTIFACTS",
	}

	doUseLocalMountsFlag := cli.BoolFlag{
		Name:   FlagUseLocalMounts,
		Usage:  FlagUseLocalMountsUsage,
		EnvVar: "DSLIM_USE_LOCAL_MOUNTS",
	}

	doUseSensorVolumeFlag := cli.StringFlag{
		Name:   FlagUseSensorVolume,
		Value:  "",
		Usage:  FlagUseSensorVolumeUsage,
		EnvVar: "DSLIM_USE_SENSOR_VOLUME",
	}

	doUseMountFlag := cli.StringSliceFlag{
		Name:   FlagMount,
		Value:  &cli.StringSlice{},
		Usage:  FlagMountUsage,
		EnvVar: "DSLIM_MOUNT",
	}

	doContinueAfterFlag := cli.StringFlag{
		Name:   FlagContinueAfter,
		Value:  "probe",
		Usage:  FlagContinueAfterUsage,
		EnvVar: "DSLIM_CONTINUE_AFTER",
	}

	//enable 'show-progress' by default only on Mac OS X
	var doShowProgressFlag cli.Flag
	switch runtime.GOOS {
	case "darwin":
		doShowProgressFlag = cli.BoolTFlag{
			Name:   FlagShowProgress,
			Usage:  fmt.Sprintf("%s (default: true)", FlagShowProgressUsage),
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	default:
		doShowProgressFlag = cli.BoolFlag{
			Name:   FlagShowProgress,
			Usage:  fmt.Sprintf("%s (default: false)", FlagShowProgressUsage),
			EnvVar: "DSLIM_UPDATE_SHOW_PROGRESS",
		}
	}

	app.Commands = []cli.Command{
		{
			Name:    cmdSpecs[CmdHelp].name,
			Aliases: []string{cmdSpecs[CmdHelp].alias},
			Usage:   cmdSpecs[CmdHelp].usage,
			Action: func(ctx *cli.Context) error {
				cli.ShowAppHelp(ctx)
				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdVersion].name,
			Aliases: []string{cmdSpecs[CmdVersion].alias},
			Usage:   cmdSpecs[CmdVersion].usage,
			Action: func(ctx *cli.Context) error {
				doDebug := ctx.GlobalBool(FlagDebug)
				inContainer, isDSImage := isInContainer(ctx.GlobalBool(FlagInContainer))
				clientConfig := getDockerClientConfig(ctx)
				commands.OnVersion(doDebug, inContainer, isDSImage, clientConfig)
				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdUpdate].name,
			Aliases: []string{cmdSpecs[CmdUpdate].alias},
			Usage:   cmdSpecs[CmdUpdate].usage,
			Flags: []cli.Flag{
				doShowProgressFlag,
			},
			Action: func(ctx *cli.Context) error {
				doDebug := ctx.GlobalBool(FlagDebug)
				statePath := ctx.GlobalString(FlagStatePath)
				inContainer, isDSImage := isInContainer(ctx.GlobalBool(FlagInContainer))
				archiveState := archiveState(ctx.GlobalString(FlagArchiveState), inContainer)
				doShowProgress := ctx.Bool(FlagShowProgress)

				commands.OnUpdate(doDebug, statePath, archiveState, inContainer, isDSImage, doShowProgress)
				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdContainerize].name,
			Aliases: []string{cmdSpecs[CmdContainerize].alias},
			Usage:   cmdSpecs[CmdContainerize].usage,
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("docker-slim[containerize]: missing target info...\n\n")
					cli.ShowCommandHelp(ctx, CmdContainerize)
					return nil
				}

				gcvalues, err := globalCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				targetRef := ctx.Args().First()

				ec := &commands.ExecutionContext{}

				commands.OnContainerize(
					gcvalues,
					targetRef,
					ec)
				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdLint].name,
			Aliases: []string{cmdSpecs[CmdLint].alias},
			Usage:   cmdSpecs[CmdLint].usage,
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) < 1 {
					fmt.Printf("docker-slim[lint]: missing target image/Dockerfile...\n\n")
					cli.ShowCommandHelp(ctx, CmdLint)
					return nil
				}

				gcvalues, err := globalCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				targetRef := ctx.Args().First()

				ec := &commands.ExecutionContext{}

				commands.OnLint(
					gcvalues,
					targetRef,
					ec)

				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdXray].name,
			Aliases: []string{cmdSpecs[CmdXray].alias},
			Usage:   cmdSpecs[CmdXray].usage,
			Flags: []cli.Flag{
				doTargetFlag,
				cli.StringSliceFlag{
					Name:   FlagChanges,
					Value:  &cli.StringSlice{""},
					Usage:  FlagChangesUsage,
					EnvVar: "DSLIM_CHANGES",
				},
				cli.StringSliceFlag{
					Name:   FlagLayer,
					Value:  &cli.StringSlice{},
					Usage:  FlagLayerUsage,
					EnvVar: "DSLIM_LAYER",
				},
				cli.BoolFlag{
					Name:   FlagAddImageManifest,
					Usage:  FlagAddImageManifestUsage,
					EnvVar: "DSLIM_XRAY_IMAGE_MANIFEST",
				},
				cli.BoolFlag{
					Name:   FlagAddImageConfig,
					Usage:  FlagAddImageConfigUsage,
					EnvVar: "DSLIM_XRAY_IMAGE_CONFIG",
				},
				doRemoveFileArtifactsFlag,
			},
			Action: func(ctx *cli.Context) error {
				targetRef := ctx.String(FlagTarget)

				if targetRef == "" {
					if len(ctx.Args()) < 1 {
						fmt.Printf("docker-slim[xray]: missing image ID/name...\n\n")
						cli.ShowCommandHelp(ctx, CmdXray)
						return nil
					} else {
						targetRef = ctx.Args().First()
					}
				}

				gcvalues, err := globalCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				changes, err := parseChangeTypes(ctx.StringSlice(FlagChanges))
				if err != nil {
					fmt.Printf("docker-slim[xray]: invalid change types: %v\n", err)
					return err
				}

				layers, err := parseLayerSelectors(ctx.StringSlice(FlagLayer))
				if err != nil {
					fmt.Printf("docker-slim[xray]: invalid layer selectors: %v\n", err)
					return err
				}

				doAddImageManifest := ctx.Bool(FlagAddImageManifest)
				doAddImageConfig := ctx.Bool(FlagAddImageConfig)
				doRmFileArtifacts := ctx.Bool(FlagRemoveFileArtifacts)

				ec := &commands.ExecutionContext{}

				commands.OnXray(
					gcvalues,
					targetRef,
					changes,
					layers,
					doAddImageManifest,
					doAddImageConfig,
					doRmFileArtifacts,
					ec)
				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdBuild].name,
			Aliases: []string{cmdSpecs[CmdBuild].alias},
			Usage:   cmdSpecs[CmdBuild].usage,
			Flags: []cli.Flag{
				doTargetFlag,
				cli.StringFlag{
					Name:   FlagBuildFromDockerfile,
					Value:  "",
					Usage:  FlagBuildFromDockerfileUsage,
					EnvVar: "DSLIM_BUILD_DOCKERFILE",
				},
				doHTTPProbeFlag,
				doHTTPProbeCmdFlag,
				doHTTPProbeCmdFileFlag,
				doHTTPProbeRetryCountFlag,
				doHTTPProbeRetryWaitFlag,
				doHTTPProbePortsFlag,
				doHTTPProbeFullFlag,
				doHTTPProbeExitOnFailureFlag,
				doKeepPermsFlag,
				doRunTargetAsUserFlag,
				doShowContainerLogsFlag,
				doShowBuildLogsFlag,
				doCopyMetaArtifactsFlag,
				doRemoveFileArtifactsFlag,
				cli.StringFlag{
					Name:   FlagTag,
					Value:  "",
					Usage:  FlagTagUsage,
					EnvVar: "DSLIM_TARGET_TAG",
				},
				cli.StringFlag{
					Name:   FlagTagFat,
					Value:  "",
					Usage:  FlagTagFatUsage,
					EnvVar: "DSLIM_TARGET_TAG_FAT",
				},
				cli.StringFlag{
					Name:   FlagImageOverrides,
					Value:  "",
					Usage:  FlagImageOverridesUsage,
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
				doExcludePatternFlag,
				doSetPathPermsFlag,
				doSetPathPermsFileFlag,
				doIncludePathFlag,
				doIncludePathFileFlag,
				doIncludeBinFlag,
				doIncludeExeFlag,
				doIncludeShellFlag,
				doUseMountFlag,
				doContinueAfterFlag,
				doUseLocalMountsFlag,
				doUseSensorVolumeFlag,
				doKeepTmpArtifactsFlag,
			},
			Action: func(ctx *cli.Context) error {
				targetRef := ctx.String(FlagTarget)

				if targetRef == "" {
					if len(ctx.Args()) < 1 {
						fmt.Printf("docker-slim[build]: missing image ID/name...\n\n")
						cli.ShowCommandHelp(ctx, CmdBuild)
						return nil
					} else {
						targetRef = ctx.Args().First()
					}
				}

				gcvalues, err := globalCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				doRmFileArtifacts := ctx.Bool(FlagRemoveFileArtifacts)
				doCopyMetaArtifacts := ctx.String(FlagCopyMetaArtifacts)

				buildFromDockerfile := ctx.String(FlagBuildFromDockerfile)

				doHTTPProbe := ctx.Bool(FlagHTTPProbe)

				httpProbeCmds, err := getHTTPProbes(ctx)
				if err != nil {
					fmt.Printf("docker-slim[build]: invalid HTTP probes: %v\n", err)
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
					fmt.Printf("docker-slim[build]: invalid HTTP Probe target ports: %v\n", err)
					return err
				}

				doHTTPProbeFull := ctx.Bool(FlagHTTPProbeFull)
				doHTTPProbeExitOnFailure := ctx.Bool(FlagHTTPProbeExitOnFailure)

				doKeepPerms := ctx.Bool(FlagKeepPerms)

				doRunTargetAsUser := ctx.Bool(FlagRunTargetAsUser)

				doShowContainerLogs := ctx.Bool(FlagShowContainerLogs)
				doShowBuildLogs := ctx.Bool(FlagShowBuildLogs)
				doTag := ctx.String(FlagTag)
				doTagFat := ctx.String(FlagTagFat)

				doImageOverrides := ctx.String(FlagImageOverrides)
				overrides, err := getContainerOverrides(ctx)
				if err != nil {
					fmt.Printf("docker-slim[build]: invalid container overrides: %v\n", err)
					return err
				}

				instructions, err := getImageInstructions(ctx)
				if err != nil {
					fmt.Printf("docker-slim[build]: invalid image instructions: %v\n", err)
					return err
				}

				volumeMounts, err := parseVolumeMounts(ctx.StringSlice(FlagMount))
				if err != nil {
					fmt.Printf("docker-slim[build]: invalid volume mounts: %v\n", err)
					return err
				}

				excludePatterns := parsePaths(ctx.StringSlice(FlagExcludePattern))

				includePaths := parsePaths(ctx.StringSlice(FlagIncludePath))
				moreIncludePaths, err := parsePathsFile(ctx.String(FlagIncludePathFile))
				if err != nil {
					fmt.Printf("docker-slim[build]: could not read include path file (ignoring): %v\n", err)
				} else {
					for k, v := range moreIncludePaths {
						includePaths[k] = v
					}
				}

				pathPerms := parsePaths(ctx.StringSlice(FlagPathPerms))
				morePathPerms, err := parsePathsFile(ctx.String(FlagPathPermsFile))
				if err != nil {
					fmt.Printf("docker-slim[build]: could not read path perms file (ignoring): %v\n", err)
				} else {
					for k, v := range morePathPerms {
						pathPerms[k] = v
					}
				}

				includeBins := parsePaths(ctx.StringSlice(FlagIncludeBin))
				includeExes := parsePaths(ctx.StringSlice(FlagIncludeExe))
				doIncludeShell := ctx.Bool(FlagIncludeShell)

				doUseLocalMounts := ctx.Bool(FlagUseLocalMounts)
				doUseSensorVolume := ctx.String(FlagUseSensorVolume)

				doKeepTmpArtifacts := ctx.Bool(FlagKeepTmpArtifacts)

				doExcludeMounts := ctx.BoolT(FlagExcludeMounts)
				if doExcludeMounts {
					for mpath := range volumeMounts {
						excludePatterns[mpath] = nil
						mpattern := fmt.Sprintf("%s/**", mpath)
						excludePatterns[mpattern] = nil
					}
				}

				continueAfter, err := getContinueAfter(ctx)
				if err != nil {
					fmt.Printf("docker-slim[build]: invalid continue-after mode: %v\n", err)
					return err
				}

				if !doHTTPProbe && continueAfter.Mode == "probe" {
					fmt.Printf("docker-slim[build]: info=probe message='changing continue-after from probe to enter because http-probe is disabled'\n")
					continueAfter.Mode = "enter"
				}

				commandReport := ctx.GlobalString(FlagCommandReport)
				if commandReport == "off" {
					commandReport = ""
				}

				ec := &commands.ExecutionContext{}

				commands.OnBuild(
					gcvalues,
					targetRef,
					buildFromDockerfile,
					doTag,
					doTagFat,
					doHTTPProbe,
					httpProbeCmds,
					httpProbeRetryCount,
					httpProbeRetryWait,
					httpProbePorts,
					doHTTPProbeFull,
					doHTTPProbeExitOnFailure,
					doRmFileArtifacts,
					doCopyMetaArtifacts,
					doRunTargetAsUser,
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
					doKeepPerms,
					pathPerms,
					excludePatterns,
					includePaths,
					includeBins,
					includeExes,
					doIncludeShell,
					doUseLocalMounts,
					doUseSensorVolume,
					doKeepTmpArtifacts,
					continueAfter,
					ec)

				return nil
			},
		},
		{
			Name:    cmdSpecs[CmdProfile].name,
			Aliases: []string{cmdSpecs[CmdProfile].alias},
			Usage:   cmdSpecs[CmdProfile].usage,
			Flags: []cli.Flag{
				doTargetFlag,
				doShowContainerLogsFlag,
				doHTTPProbeFlag,
				doHTTPProbeCmdFlag,
				doHTTPProbeCmdFileFlag,
				doHTTPProbeRetryCountFlag,
				doHTTPProbeRetryWaitFlag,
				doHTTPProbePortsFlag,
				doHTTPProbeFullFlag,
				doHTTPProbeExitOnFailureFlag,
				doKeepPermsFlag,
				doRunTargetAsUserFlag,
				doCopyMetaArtifactsFlag,
				doRemoveFileArtifactsFlag,
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
				doExcludePatternFlag,
				doSetPathPermsFlag,
				doSetPathPermsFileFlag,
				doIncludePathFlag,
				doIncludePathFileFlag,
				doIncludeBinFlag,
				doIncludeExeFlag,
				doIncludeShellFlag,
				doUseMountFlag,
				doContinueAfterFlag,
				doUseLocalMountsFlag,
				doUseSensorVolumeFlag,
				doKeepTmpArtifactsFlag,
			},
			Action: func(ctx *cli.Context) error {
				targetRef := ctx.String(FlagTarget)

				if targetRef == "" {
					if len(ctx.Args()) < 1 {
						fmt.Printf("docker-slim[profile]: missing image ID/name...\n\n")
						cli.ShowCommandHelp(ctx, CmdProfile)
						return nil
					} else {
						targetRef = ctx.Args().First()
					}
				}

				gcvalues, err := globalCommandFlagValues(ctx)
				if err != nil {
					return err
				}

				doRmFileArtifacts := ctx.Bool(FlagRemoveFileArtifacts)
				doCopyMetaArtifacts := ctx.String(FlagCopyMetaArtifacts)

				doHTTPProbe := ctx.Bool(FlagHTTPProbe)

				httpProbeCmds, err := getHTTPProbes(ctx)
				if err != nil {
					fmt.Printf("docker-slim[profile]: invalid HTTP probes: %v\n", err)
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
					fmt.Printf("docker-slim[profile]: invalid HTTP Probe target ports: %v\n", err)
					return err
				}

				doHTTPProbeFull := ctx.Bool(FlagHTTPProbeFull)
				doHTTPProbeExitOnFailure := ctx.Bool(FlagHTTPProbeExitOnFailure)

				doKeepPerms := ctx.Bool(FlagKeepPerms)

				doRunTargetAsUser := ctx.Bool(FlagRunTargetAsUser)

				doShowContainerLogs := ctx.Bool(FlagShowContainerLogs)
				overrides, err := getContainerOverrides(ctx)
				if err != nil {
					fmt.Printf("docker-slim[profile]: invalid container overrides: %v", err)
					return err
				}

				volumeMounts, err := parseVolumeMounts(ctx.StringSlice(FlagMount))
				if err != nil {
					fmt.Printf("docker-slim[profile]: invalid volume mounts: %v\n", err)
					return err
				}

				excludePatterns := parsePaths(ctx.StringSlice(FlagExcludePattern))

				includePaths := parsePaths(ctx.StringSlice(FlagIncludePath))
				moreIncludePaths, err := parsePathsFile(ctx.String(FlagIncludePathFile))
				if err != nil {
					fmt.Printf("docker-slim[profile]: could not read include path file (ignoring): %v\n", err)
				} else {
					for k, v := range moreIncludePaths {
						includePaths[k] = v
					}
				}

				pathPerms := parsePaths(ctx.StringSlice(FlagPathPerms))
				morePathPerms, err := parsePathsFile(ctx.String(FlagPathPermsFile))
				if err != nil {
					fmt.Printf("docker-slim[profile]: could not read path perms file (ignoring): %v\n", err)
				} else {
					for k, v := range morePathPerms {
						pathPerms[k] = v
					}
				}

				includeBins := parsePaths(ctx.StringSlice(FlagIncludeBin))
				includeExes := parsePaths(ctx.StringSlice(FlagIncludeExe))
				doIncludeShell := ctx.Bool(FlagIncludeShell)

				doUseLocalMounts := ctx.Bool(FlagUseLocalMounts)
				doUseSensorVolume := ctx.String(FlagUseSensorVolume)

				doKeepTmpArtifacts := ctx.Bool(FlagKeepTmpArtifacts)

				doExcludeMounts := ctx.BoolT(FlagExcludeMounts)
				if doExcludeMounts {
					for mpath := range volumeMounts {
						excludePatterns[mpath] = nil
						mpattern := fmt.Sprintf("%s/**", mpath)
						excludePatterns[mpattern] = nil
					}
				}

				continueAfter, err := getContinueAfter(ctx)
				if err != nil {
					fmt.Printf("docker-slim[profile]: invalid continue-after mode: %v\n", err)
					return err
				}

				if !doHTTPProbe && continueAfter.Mode == "probe" {
					fmt.Printf("docker-slim[profile]: info=probe message='changing continue-after from probe to enter because http-probe is disabled'\n")
					continueAfter.Mode = "enter"
				}

				commandReport := ctx.GlobalString(FlagCommandReport)
				if commandReport == "off" {
					commandReport = ""
				}

				ec := &commands.ExecutionContext{}

				commands.OnProfile(
					gcvalues,
					targetRef,
					doHTTPProbe,
					httpProbeCmds,
					httpProbeRetryCount,
					httpProbeRetryWait,
					httpProbePorts,
					doHTTPProbeFull,
					doHTTPProbeExitOnFailure,
					doRmFileArtifacts,
					doCopyMetaArtifacts,
					doRunTargetAsUser,
					doShowContainerLogs,
					overrides,
					ctx.StringSlice(FlagLink),
					ctx.StringSlice(FlagEtcHostsMap),
					ctx.StringSlice(FlagContainerDNS),
					ctx.StringSlice(FlagContainerDNSSearch),
					volumeMounts,
					doKeepPerms,
					pathPerms,
					excludePatterns,
					includePaths,
					includeBins,
					includeExes,
					doIncludeShell,
					doUseLocalMounts,
					doUseSensorVolume,
					doKeepTmpArtifacts,
					continueAfter,
					ec)

				return nil
			},
		},
	}
}

func getContinueAfter(ctx *cli.Context) (*config.ContinueAfter, error) {
	info := &config.ContinueAfter{
		Mode: "enter",
	}

	doContinueAfter := ctx.String(FlagContinueAfter)
	switch doContinueAfter {
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
		if waitTime, err := strconv.Atoi(doContinueAfter); err == nil && waitTime > 0 {
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

	getEnv(dockerclient.EnvDockerHost)
	getEnv(dockerclient.EnvDockerTLSVerify)
	getEnv(dockerclient.EnvDockerCertPath)

	return config
}

func runCli() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
