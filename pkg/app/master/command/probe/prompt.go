package probe

import (
	"github.com/c-bata/go-prompt"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(FlagTarget), Description: FlagTargetUsage},
		{Text: command.FullFlagName(FlagPort), Description: FlagPortUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeCmd), Description: command.FlagHTTPProbeCmdUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeCmdFile), Description: command.FlagHTTPProbeCmdFileUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeStartWait), Description: command.FlagHTTPProbeStartWaitUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeRetryCount), Description: command.FlagHTTPProbeRetryCountUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeRetryWait), Description: command.FlagHTTPProbeRetryWaitUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbePorts), Description: command.FlagHTTPProbePortsUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeFull), Description: command.FlagHTTPProbeFullUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeExitOnFailure), Description: command.FlagHTTPProbeExitOnFailureUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeCrawl), Description: command.FlagHTTPProbeCrawlUsage},
		{Text: command.FullFlagName(command.FlagHTTPCrawlMaxDepth), Description: command.FlagHTTPCrawlMaxDepthUsage},
		{Text: command.FullFlagName(command.FlagHTTPCrawlMaxPageCount), Description: command.FlagHTTPCrawlMaxPageCountUsage},
		{Text: command.FullFlagName(command.FlagHTTPCrawlConcurrency), Description: command.FlagHTTPCrawlConcurrencyUsage},
		{Text: command.FullFlagName(command.FlagHTTPMaxConcurrentCrawlers), Description: command.FlagHTTPMaxConcurrentCrawlersUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeAPISpec), Description: command.FlagHTTPProbeAPISpecUsage},
		{Text: command.FullFlagName(command.FlagHTTPProbeAPISpecFile), Description: command.FlagHTTPProbeAPISpecFileUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagHTTPProbeCmdFile):     command.CompleteFile,
		command.FullFlagName(command.FlagHTTPProbeFull):        command.CompleteBool,
		command.FullFlagName(command.FlagHTTPProbeCrawl):       command.CompleteTBool,
		command.FullFlagName(command.FlagHTTPProbeAPISpecFile): command.CompleteFile,
	},
}
