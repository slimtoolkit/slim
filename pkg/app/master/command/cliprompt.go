package command

//CLI prompts

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/dustin/go-humanize"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/docker/dockerutil"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	"github.com/slimtoolkit/slim/pkg/version"
)

type InteractiveApp struct {
	appPrompt   *prompt.Prompt
	fpCompleter completer.FilePathCompleter
	app         *cli.App
	dclient     *dockerapi.Client
}

func NewInteractiveApp(app *cli.App, gparams *GenericParams) *InteractiveApp {
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
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}
		fmt.Printf("slim: info=docker.connect.error message='%s'\n", exitMsg)
		fmt.Printf("slim: state=exited version=%s location='%s'\n", version.Current(), fsutil.ExeDir())
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
		app.ShowCommunityInfo(OutputFormatText)
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

	args := []string{AppName}
	for _, val := range parts {
		if val == "" {
			continue
		}

		args = append(args, val)
	}

	if err := ia.app.Run(args); err != nil {
		log.Fatal(err)
	}
}

func (ia *InteractiveApp) complete(params prompt.Document) []prompt.Suggest {
	allParamsLine := params.TextBeforeCursor()

	allParamsLine = strings.TrimSpace(allParamsLine)
	if allParamsLine == "" {
		return append(CommandSuggestions, GlobalFlagSuggestions...)
	}

	commandState := newCurrentCommandState()
	commandState.Dclient = ia.dclient

	currentToken := params.GetWordBeforeCursor()
	commandState.CurrentToken = currentToken

	allTokens := strings.Split(allParamsLine, " ")
	commandState.AllTokensList = allTokens

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

	commandState.PrevToken = prevToken
	commandState.PrevTokenIdx = prevTokenIdx
	commandState.State = InputStateCommand

	if prevToken == "" {
		saveCurrentCommandState(commandState)

		suggestions := append(CommandSuggestions, GlobalFlagSuggestions...)
		return prompt.FilterHasPrefix(suggestions, currentToken, true)
	}

	commandTokenIdx := -1
	lastValueIdx := -1
	var lastFlagName string

	for i := 0; i <= prevTokenIdx; i++ {
		if strings.HasPrefix(allTokens[i], "--") {
			lastFlagName = allTokens[i]
			lastValueIdx = -1
		} else {
			if lastFlagName == "" {
				//non-flag name token
				//command token if the previous token was not a flag name
				commandTokenIdx = i
				break
			}

			if lastFlagName == "" {
				lastValueIdx = i
			} else {
				lastFlagName = ""
			}
		}
	}

	if commandTokenIdx == -1 && lastValueIdx > -1 {
		commandTokenIdx = lastValueIdx
	}

	if commandTokenIdx == -1 {
		saveCurrentCommandState(commandState)

		if strings.HasPrefix(prevToken, "--") {
			if completeValue, ok := GlobalFlagValueSuggestions[prevToken]; ok && completeValue != nil {
				return completeValue(ia, currentToken, params)
			}
		} else {
			suggestions := append(CommandSuggestions, GlobalFlagSuggestions...)
			return prompt.FilterHasPrefix(suggestions, currentToken, true)
		}

		return []prompt.Suggest{}
	}

	commandToken := allTokens[commandTokenIdx]

	commandState.CommandTokenIdx = commandTokenIdx
	commandState.Command = commandToken

	if strings.HasPrefix(commandState.CurrentToken, "--") {
		commandState.State = InputStateCommandFlag
	} else {
		commandState.State = InputStateCommandFlagValue
	}

	lastFlagName = ""
	for i := 0; i < commandTokenIdx; i++ {
		if strings.HasPrefix(allTokens[i], "--") {
			lastFlagName = allTokens[i]
		} else {
			if lastFlagName != "" {
				commandState.GlobalFlags[lastFlagName] = allTokens[i]
				lastFlagName = ""
			}
		}
	}

	if commandTokenIdx == (tokenCount - 1) {
		saveCurrentCommandState(commandState)
		if currentToken != "" {
			//currentToken still points to the command token
			return prompt.FilterHasPrefix(CommandSuggestions, currentToken, true)
		} else {
			//need to return the command flag suggestions
			if cmdFlagSuggestions, ok := CommandFlagSuggestions[commandToken]; ok && cmdFlagSuggestions != nil {
				return prompt.FilterHasPrefix(cmdFlagSuggestions.Names, currentToken, true)
			} else {
				return []prompt.Suggest{}
			}
		}
	} else {
		lastFlagName = ""
		for i := commandTokenIdx + 1; i < tokenCount; i++ {
			if strings.HasPrefix(allTokens[i], "--") {
				lastFlagName = allTokens[i]
			} else {
				if lastFlagName != "" {
					valList := commandState.CommandFlags[lastFlagName]
					valList = append(valList, allTokens[i])
					commandState.CommandFlags[lastFlagName] = valList
					lastFlagName = ""
				}
			}
		}

		saveCurrentCommandState(commandState)
	}

	cmdFlagSuggestions, ok := CommandFlagSuggestions[commandToken]
	if !ok {
		return []prompt.Suggest{}
	}

	if strings.HasPrefix(prevToken, "--") {
		if completeValue, ok := cmdFlagSuggestions.Values[prevToken]; ok && completeValue != nil {
			return completeValue(ia, currentToken, params)
		}
	} else {
		return prompt.FilterHasPrefix(cmdFlagSuggestions.Names, currentToken, true)
	}

	return []prompt.Suggest{}
}

func (ia *InteractiveApp) Run() {
	ia.appPrompt.Run()
}

/////////////////////////////////////////////

type CompleteValue func(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest

type FlagSuggestions struct {
	Names  []prompt.Suggest
	Values map[string]CompleteValue
}

var CommandFlagSuggestions = map[string]*FlagSuggestions{}

var CommandSuggestions = []prompt.Suggest{
	{Text: "exit", Description: "Exit app"},
}

const (
	InputStateEmpty            = "empty"
	InputStateGlobalFlag       = "global.flag"
	InputStateGlobalFlagValue  = "global.flag.value"
	InputStateCommand          = "command"
	InputStateCommandFlag      = "command.flag"
	InputStateCommandFlagValue = "command.flag.value"
)

type CurrentCommandState struct {
	Dclient *dockerapi.Client
	State   string

	AllTokensList   []string
	CurrentToken    string
	PrevToken       string
	PrevTokenIdx    int
	CommandTokenIdx int

	GlobalFlags  map[string]string
	Command      string
	CommandFlags map[string][]string
}

func newCurrentCommandState() *CurrentCommandState {
	return &CurrentCommandState{
		State:           InputStateEmpty,
		PrevTokenIdx:    -1,
		CommandTokenIdx: -1,
		GlobalFlags:     map[string]string{},
		CommandFlags:    map[string][]string{},
	}
}

func (ref *CurrentCommandState) GetCFValue(name string) string {
	return ref.GetCFValueWithDefault(name, "")
}

func (ref *CurrentCommandState) GetCFValueWithDefault(name string, dvalue string) string {
	fullFlag := FullFlagName(name)
	vals, found := ref.CommandFlags[fullFlag]
	if found && len(vals) > 0 && vals[0] != "" {
		return vals[0]
	}

	return dvalue
}

func saveCurrentCommandState(value *CurrentCommandState) {
	//fmt.Printf("\nsaveCurrentCommandState: %#v\n\n",value)
	gCurrentCommandState = value
}

var gCurrentCommandState *CurrentCommandState

func GetCurrentCommandState() *CurrentCommandState {
	return gCurrentCommandState
}

/////////////////////////////////////////////

//NOTE: command packages will add their prompt command suggestion in their init()

var GlobalFlagSuggestions = []prompt.Suggest{
	{Text: FullFlagName(FlagStatePath), Description: FlagStatePathUsage},
	{Text: FullFlagName(FlagCommandReport), Description: FlagCommandReportUsage},
	{Text: FullFlagName(FlagDebug), Description: FlagDebugUsage},
	{Text: FullFlagName(FlagVerbose), Description: FlagVerboseUsage},
	{Text: FullFlagName(FlagLogLevel), Description: FlagLogLevelUsage},
	{Text: FullFlagName(FlagLog), Description: FlagLogUsage},
	{Text: FullFlagName(FlagLogFormat), Description: FlagLogFormatUsage},
	{Text: FullFlagName(FlagQuietCLIMode), Description: FlagQuietCLIModeUsage},
	{Text: FullFlagName(FlagOutputFormat), Description: FlagOutputFormatUsage},
	{Text: FullFlagName(FlagUseTLS), Description: FlagUseTLSUsage},
	{Text: FullFlagName(FlagVerifyTLS), Description: FlagVerifyTLSUsage},
	{Text: FullFlagName(FlagTLSCertPath), Description: FlagTLSCertPathUsage},
	{Text: FullFlagName(FlagHost), Description: FlagHostUsage},
	{Text: FullFlagName(FlagArchiveState), Description: FlagArchiveStateUsage},
	{Text: FullFlagName(FlagInContainer), Description: FlagInContainerUsage},
	{Text: FullFlagName(FlagCheckVersion), Description: FlagCheckVersionUsage},
	{Text: FullFlagName(FlagNoColor), Description: FlagNoColorUsage},
}

var GlobalFlagValueSuggestions = map[string]CompleteValue{
	FullFlagName(FlagQuietCLIMode): CompleteBool,
	FullFlagName(FlagOutputFormat): CompleteOutputFormat,
	FullFlagName(FlagDebug):        CompleteBool,
	FullFlagName(FlagVerbose):      CompleteBool,
	FullFlagName(FlagNoColor):      CompleteBool,
	FullFlagName(FlagCheckVersion): CompleteTBool,
}

func FullFlagName(name string) string {
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

var continueAfterValues = []prompt.Suggest{
	{Text: config.CAMAppExit, Description: "Continue after the target app exits"},
	{Text: config.CAMHostExec, Description: "Continue after host command execution is finished running"},
	{Text: config.CAMExec, Description: "Continue after container command execution is finished running"},
	{Text: config.CAMProbe, Description: "Continue after the HTTP probe is finished running"},
	{Text: config.CAMEnter, Description: "Use the <enter> key to indicate you that you are done using the container"},
	{Text: config.CAMSignal, Description: "Use SIGUSR1 to signal that you are done using the container"},
	{Text: config.CAMTimeout, Description: "Continue after the default timeout (60 seconds)"},
	{Text: config.CAMContainerProbe, Description: "Continue after the probed container exits"},
	{Text: "<seconds>", Description: "Enter the number of seconds to wait instead of <seconds>"},
}

var consoleOutputValues = []prompt.Suggest{
	{Text: OutputFormatText, Description: "Default, output in text format (as a table in quiet CLI mode)"},
	{Text: OutputFormatJSON, Description: "JSON output format"},
}

var ipcModeValues = []prompt.Suggest{
	{Text: "proxy", Description: "Proxy sensor ipc mode"},
	{Text: "direct", Description: "Direct sensor ipc mode"},
}

func CompleteProgress(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	switch runtime.GOOS {
	case "darwin":
		return CompleteTBool(ia, token, params)
	default:
		return CompleteBool(ia, token, params)
	}
}

func CompleteBool(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(boolValues, token, true)
}

func CompleteTBool(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(tboolValues, token, true)
}

func CompleteContinueAfter(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(continueAfterValues, token, true)
}

func CompleteOutputFormat(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(consoleOutputValues, token, true)
}

func CompleteIPCMode(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(ipcModeValues, token, true)
}

func CompleteImage(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	images, err := dockerutil.ListImages(ia.dclient, "")
	if err != nil {
		log.Errorf("CompleteImage(%q): error - %v", token, err)
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

func CompleteVolume(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
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

func CompleteNetwork(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
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

func CompleteFile(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return ia.fpCompleter.Complete(params)
}
