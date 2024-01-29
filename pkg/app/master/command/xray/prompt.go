package xray

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
		{Text: command.FullFlagName(command.FlagCommandParamsFile), Description: command.FlagCommandParamsFileUsage},
		{Text: command.FullFlagName(command.FlagTarget), Description: command.FlagTargetUsage},
		{Text: command.FullFlagName(command.FlagPull), Description: command.FlagPullUsage},
		{Text: command.FullFlagName(command.FlagShowPullLogs), Description: command.FlagShowPullLogsUsage},
		{Text: command.FullFlagName(command.FlagRegistryAccount), Description: command.FlagRegistryAccountUsage},
		{Text: command.FullFlagName(command.FlagRegistrySecret), Description: command.FlagRegistrySecretUsage},
		{Text: command.FullFlagName(command.FlagDockerConfigPath), Description: command.FlagDockerConfigPathUsage},
		{Text: command.FullFlagName(FlagChanges), Description: FlagChangesUsage},
		{Text: command.FullFlagName(FlagChangesOutput), Description: FlagChangesOutputUsage},
		{Text: command.FullFlagName(FlagLayer), Description: FlagLayerUsage},
		{Text: command.FullFlagName(FlagAddImageManifest), Description: FlagAddImageManifestUsage},
		{Text: command.FullFlagName(FlagAddImageConfig), Description: FlagAddImageConfigUsage},
		{Text: command.FullFlagName(FlagLayerChangesMax), Description: FlagLayerChangesMaxUsage},
		{Text: command.FullFlagName(FlagAllChangesMax), Description: FlagAllChangesMaxUsage},
		{Text: command.FullFlagName(FlagAddChangesMax), Description: FlagAddChangesMaxUsage},
		{Text: command.FullFlagName(FlagModifyChangesMax), Description: FlagModifyChangesMaxUsage},
		{Text: command.FullFlagName(FlagDeleteChangesMax), Description: FlagDeleteChangesMaxUsage},
		{Text: command.FullFlagName(FlagChangePath), Description: FlagChangePathUsage},
		{Text: command.FullFlagName(FlagChangeData), Description: FlagChangeDataUsage},
		{Text: command.FullFlagName(FlagReuseSavedImage), Description: FlagReuseSavedImageUsage},
		{Text: command.FullFlagName(FlagHashData), Description: FlagHashDataUsage},
		{Text: command.FullFlagName(FlagDetectUTF8), Description: FlagDetectUTF8Usage},
		{Text: command.FullFlagName(FlagDetectDuplicates), Description: FlagDetectDuplicatesUsage},
		{Text: command.FullFlagName(FlagShowDuplicates), Description: FlagShowDuplicatesUsage},
		{Text: command.FullFlagName(FlagShowSpecialPerms), Description: FlagShowSpecialPermsUsage},
		{Text: command.FullFlagName(FlagChangeDataHash), Description: FlagChangeDataHashUsage},
		{Text: command.FullFlagName(FlagTopChangesMax), Description: FlagTopChangesMaxUsage},
		{Text: command.FullFlagName(FlagDetectAllCertFiles), Description: FlagDetectAllCertFilesUsage},
		{Text: command.FullFlagName(FlagDetectAllCertPKFiles), Description: FlagDetectAllCertPKFilesUsage},
		{Text: command.FullFlagName(FlagDetectIdentities), Description: FlagDetectIdentitiesUsage},
		{Text: command.FullFlagName(FlagDetectIdentitiesParam), Description: FlagDetectIdentitiesParamUsage},
		{Text: command.FullFlagName(FlagDetectIdentitiesDumpRaw), Description: FlagDetectIdentitiesDumpRawUsage},
		{Text: command.FullFlagName(FlagExportAllDataArtifacts), Description: FlagExportAllDataArtifactsUsage},
		{Text: command.FullFlagName(command.FlagRemoveFileArtifacts), Description: command.FlagRemoveFileArtifactsUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagCommandParamsFile):   command.CompleteFile,
		command.FullFlagName(command.FlagPull):                command.CompleteTBool,
		command.FullFlagName(command.FlagShowPullLogs):        command.CompleteBool,
		command.FullFlagName(command.FlagDockerConfigPath):    command.CompleteFile,
		command.FullFlagName(command.FlagTarget):              command.CompleteImage,
		command.FullFlagName(FlagChanges):                     completeLayerChanges,
		command.FullFlagName(FlagChangesOutput):               completeOutputs,
		command.FullFlagName(FlagAddImageManifest):            command.CompleteBool,
		command.FullFlagName(FlagAddImageConfig):              command.CompleteBool,
		command.FullFlagName(FlagHashData):                    command.CompleteBool,
		command.FullFlagName(FlagDetectDuplicates):            command.CompleteBool,
		command.FullFlagName(FlagShowDuplicates):              command.CompleteTBool,
		command.FullFlagName(FlagShowSpecialPerms):            command.CompleteTBool,
		command.FullFlagName(FlagReuseSavedImage):             command.CompleteTBool,
		command.FullFlagName(FlagDetectAllCertFiles):          command.CompleteBool,
		command.FullFlagName(FlagDetectAllCertPKFiles):        command.CompleteBool,
		command.FullFlagName(FlagDetectIdentities):            command.CompleteTBool,
		command.FullFlagName(command.FlagRemoveFileArtifacts): command.CompleteBool,
	},
}

var layerChangeValues = []prompt.Suggest{
	{Text: "none", Description: "Don't show any file system change details in image layers"},
	{Text: "all", Description: "Show all file system change details in image layers"},
	{Text: "delete", Description: "Show only 'delete' file system change details in image layers"},
	{Text: "modify", Description: "Show only 'modify' file system change details in image layers"},
	{Text: "add", Description: "Show only 'add' file system change details in image layers"},
}

func completeLayerChanges(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(layerChangeValues, token, true)
}

var outputsValues = []prompt.Suggest{
	{Text: "all", Description: "Show changes in all outputs"},
	{Text: "report", Description: "Show changes in command report"},
	{Text: "console", Description: "Show changes in console"},
}

func completeOutputs(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(outputsValues, token, true)
}
