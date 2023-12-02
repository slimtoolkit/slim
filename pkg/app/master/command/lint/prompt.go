package lint

import (
	"github.com/c-bata/go-prompt"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/docker/linter/check"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(command.FlagTarget), Description: FlagLintTargetUsage},
		{Text: command.FullFlagName(FlagTargetType), Description: FlagTargetTypeUsage},
		{Text: command.FullFlagName(FlagSkipBuildContext), Description: FlagSkipBuildContextUsage},
		{Text: command.FullFlagName(FlagBuildContextDir), Description: FlagBuildContextDirUsage},
		{Text: command.FullFlagName(FlagSkipDockerignore), Description: FlagSkipDockerignoreUsage},
		{Text: command.FullFlagName(FlagIncludeCheckLabel), Description: FlagIncludeCheckLabelUsage},
		{Text: command.FullFlagName(FlagExcludeCheckLabel), Description: FlagExcludeCheckLabelUsage},
		{Text: command.FullFlagName(FlagIncludeCheckID), Description: FlagIncludeCheckIDUsage},
		{Text: command.FullFlagName(FlagIncludeCheckIDFile), Description: FlagIncludeCheckIDFileUsage},
		{Text: command.FullFlagName(FlagExcludeCheckID), Description: FlagExcludeCheckIDUsage},
		{Text: command.FullFlagName(FlagExcludeCheckIDFile), Description: FlagExcludeCheckIDFileUsage},
		{Text: command.FullFlagName(FlagShowNoHits), Description: FlagShowNoHitsUsage},
		{Text: command.FullFlagName(FlagShowSnippet), Description: FlagShowSnippetUsage},
		{Text: command.FullFlagName(FlagListChecks), Description: FlagListChecksUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagTarget):     completeLintTarget,
		command.FullFlagName(FlagTargetType):         completeLintTargetType,
		command.FullFlagName(FlagSkipBuildContext):   command.CompleteBool,
		command.FullFlagName(FlagBuildContextDir):    command.CompleteFile,
		command.FullFlagName(FlagSkipDockerignore):   command.CompleteBool,
		command.FullFlagName(FlagIncludeCheckID):     completeLintCheckID,
		command.FullFlagName(FlagIncludeCheckIDFile): command.CompleteFile,
		command.FullFlagName(FlagExcludeCheckID):     completeLintCheckID,
		command.FullFlagName(FlagExcludeCheckIDFile): command.CompleteFile,
		command.FullFlagName(FlagShowNoHits):         command.CompleteBool,
		command.FullFlagName(FlagShowSnippet):        command.CompleteTBool,
		command.FullFlagName(FlagListChecks):         command.CompleteBool,
	},
}

func completeLintTarget(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	//for now only support selecting Dockerfiles
	//later add an ability to choose (files or images)
	//based on the target-type parameter
	return command.CompleteFile(ia, token, params)
}

var lintTargetTypeValues = []prompt.Suggest{
	{Text: "dockerfile", Description: "Dockerfile target type"},
	{Text: "image", Description: "Docker image target type"},
}

func completeLintTargetType(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(lintTargetTypeValues, token, true)
}

func completeLintCheckID(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	for _, check := range check.AllChecks {
		info := check.Get()
		entry := prompt.Suggest{
			Text:        info.ID,
			Description: info.Name,
		}

		values = append(values, entry)
	}

	return prompt.FilterContains(values, token, true)
}
