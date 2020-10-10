package lint

import (
	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/docker/linter/check"

	"github.com/c-bata/go-prompt"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &commands.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: commands.FullFlagName(commands.FlagTarget), Description: FlagLintTargetUsage},
		{Text: commands.FullFlagName(FlagTargetType), Description: FlagTargetTypeUsage},
		{Text: commands.FullFlagName(FlagSkipBuildContext), Description: FlagSkipBuildContextUsage},
		{Text: commands.FullFlagName(FlagBuildContextDir), Description: FlagBuildContextDirUsage},
		{Text: commands.FullFlagName(FlagSkipDockerignore), Description: FlagSkipDockerignoreUsage},
		{Text: commands.FullFlagName(FlagIncludeCheckLabel), Description: FlagIncludeCheckLabelUsage},
		{Text: commands.FullFlagName(FlagExcludeCheckLabel), Description: FlagExcludeCheckLabelUsage},
		{Text: commands.FullFlagName(FlagIncludeCheckID), Description: FlagIncludeCheckIDUsage},
		{Text: commands.FullFlagName(FlagIncludeCheckIDFile), Description: FlagIncludeCheckIDFileUsage},
		{Text: commands.FullFlagName(FlagExcludeCheckID), Description: FlagExcludeCheckIDUsage},
		{Text: commands.FullFlagName(FlagExcludeCheckIDFile), Description: FlagExcludeCheckIDFileUsage},
		{Text: commands.FullFlagName(FlagShowNoHits), Description: FlagShowNoHitsUsage},
		{Text: commands.FullFlagName(FlagShowSnippet), Description: FlagShowSnippetUsage},
		{Text: commands.FullFlagName(FlagListChecks), Description: FlagListChecksUsage},
	},
	Values: map[string]commands.CompleteValue{
		commands.FullFlagName(commands.FlagTarget):    completeLintTarget,
		commands.FullFlagName(FlagTargetType):         completeLintTargetType,
		commands.FullFlagName(FlagSkipBuildContext):   commands.CompleteBool,
		commands.FullFlagName(FlagBuildContextDir):    commands.CompleteFile,
		commands.FullFlagName(FlagSkipDockerignore):   commands.CompleteBool,
		commands.FullFlagName(FlagIncludeCheckID):     completeLintCheckID,
		commands.FullFlagName(FlagIncludeCheckIDFile): commands.CompleteFile,
		commands.FullFlagName(FlagExcludeCheckID):     completeLintCheckID,
		commands.FullFlagName(FlagExcludeCheckIDFile): commands.CompleteFile,
		commands.FullFlagName(FlagShowNoHits):         commands.CompleteBool,
		commands.FullFlagName(FlagShowSnippet):        commands.CompleteTBool,
		commands.FullFlagName(FlagListChecks):         commands.CompleteBool,
	},
}

func completeLintTarget(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	//for now only support selecting Dockerfiles
	//later add an ability to choose (files or images)
	//based on the target-type parameter
	return commands.CompleteFile(ia, token, params)
}

var lintTargetTypeValues = []prompt.Suggest{
	{Text: "dockerfile", Description: "Dockerfile target type"},
	{Text: "image", Description: "Docker image target type"},
}

func completeLintTargetType(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(lintTargetTypeValues, token, true)
}

func completeLintCheckID(ia *commands.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
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

func init() {
	commands.CommandFlagSuggestions[Name] = CommandFlagSuggestions
	commands.CommandSuggestions = append(commands.CommandSuggestions, CommandSuggestion)
}
