package commands

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
	"github.com/urfave/cli"

	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerutil"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	"github.com/docker-slim/docker-slim/pkg/version"
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
		ShowCommunityInfo()
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
		return append(CommandSuggestions, GlobalFlagSuggestions...)
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
		suggestions := append(CommandSuggestions, GlobalFlagSuggestions...)
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
		suggestions := append(CommandSuggestions, GlobalFlagSuggestions...)
		return prompt.FilterHasPrefix(suggestions, currentToken, true)
	}

	commandToken := allTokens[commandTokenIdx]

	if commandTokenIdx == (tokenCount - 1) {
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

type CompleteValue func(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest

type FlagSuggestions struct {
	Names  []prompt.Suggest
	Values map[string]CompleteValue
}

var CommandFlagSuggestions = map[string]*FlagSuggestions{}

var CommandSuggestions = []prompt.Suggest{
	{Text: "exit", Description: "Exit app"},
}

//NOTE: command packages will add their prompt command suggestion in their init()

var GlobalFlagSuggestions = []prompt.Suggest{
	{Text: FullFlagName(FlagStatePath), Description: FlagStatePathUsage},
	{Text: FullFlagName(FlagCommandReport), Description: FlagCommandReportUsage},
	{Text: FullFlagName(FlagDebug), Description: FlagDebugUsage},
	{Text: FullFlagName(FlagVerbose), Description: FlagVerboseUsage},
	{Text: FullFlagName(FlagLogLevel), Description: FlagLogLevelUsage},
	{Text: FullFlagName(FlagLog), Description: FlagLogUsage},
	{Text: FullFlagName(FlagLogFormat), Description: FlagLogFormatUsage},
	{Text: FullFlagName(FlagUseTLS), Description: FlagUseTLSUsage},
	{Text: FullFlagName(FlagVerifyTLS), Description: FlagVerifyTLSUsage},
	{Text: FullFlagName(FlagTLSCertPath), Description: FlagTLSCertPathUsage},
	{Text: FullFlagName(FlagHost), Description: FlagHostUsage},
	{Text: FullFlagName(FlagArchiveState), Description: FlagArchiveStateUsage},
	{Text: FullFlagName(FlagInContainer), Description: FlagInContainerUsage},
	{Text: FullFlagName(FlagCheckVersion), Description: FlagCheckVersionUsage},
	{Text: FullFlagName(FlagNoColor), Description: FlagNoColorUsage},
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
	{Text: config.CAMExec, Description: "Automatically continue after container exec is finished running"},
	{Text: config.CAMProbe, Description: "Automatically continue after the HTTP probe is finished running"},
	{Text: config.CAMEnter, Description: "Use the <enter> key to indicate you that you are done using the container"},
	{Text: config.CAMSignal, Description: "Use SIGUSR1 to signal that you are done using the container"},
	{Text: config.CAMTimeout, Description: "Automatically continue after the default timeout (60 seconds)"},
	{Text: "<seconds>", Description: "Enter the number of seconds to wait instead of <seconds>"},
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

func CompleteTarget(ia *InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
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
