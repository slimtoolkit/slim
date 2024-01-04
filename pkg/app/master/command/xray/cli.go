package xray

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerimage"
)

const (
	Name  = "xray"
	Usage = "Shows what's inside of your container image and reverse engineers its Dockerfile"
	Alias = "x"
)

type DetectOpParam struct {
	/// Operation is enabled
	Enabled bool
	/// Dump/save raw data
	DumpRaw bool
	/// Dump raw data to console
	IsConsoleOut bool
	/// Dump raw data to directory (otherwise save to an archive file)
	IsDirOut bool
	/// Output path (directory or archive path)
	OutputPath string
	/// Input parameters for the operation
	InputParams map[string]string
}

type CommandParams struct {
	DetectIdentities     *DetectOpParam `json:"detect_identities,omitempty"`
	DetectScheduledTasks *DetectOpParam `json:"detect_scheduled_tasks,omitempty"`
	DetectServices       *DetectOpParam `json:"detect_services,omitempty"`
	DetectSystemHooks    *DetectOpParam `json:"detect_system_hooks,omitempty"`

	//todo: migrate simple bool param to DetectOpParam
	DetectAllCertFiles   bool `json:"detect_all_cert_files,omitempty"`
	DetectAllCertPKFiles bool `json:"detect_all_cert_pks,omitempty"`
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		command.Cflag(command.FlagCommandParamsFile),
		command.Cflag(command.FlagTarget),
		command.Cflag(command.FlagPull),
		command.Cflag(command.FlagDockerConfigPath),
		command.Cflag(command.FlagRegistryAccount),
		command.Cflag(command.FlagRegistrySecret),
		command.Cflag(command.FlagShowPullLogs),
		cflag(FlagChanges),
		cflag(FlagChangesOutput),
		cflag(FlagLayer),
		cflag(FlagAddImageManifest),
		cflag(FlagAddImageConfig),
		cflag(FlagLayerChangesMax),
		cflag(FlagAllChangesMax),
		cflag(FlagAddChangesMax),
		cflag(FlagModifyChangesMax),
		cflag(FlagDeleteChangesMax),
		cflag(FlagChangePath),
		cflag(FlagChangeData),
		cflag(FlagReuseSavedImage),
		cflag(FlagTopChangesMax),
		cflag(FlagChangeMatchLayersOnly),
		cflag(FlagHashData),
		cflag(FlagDetectUTF8),
		cflag(FlagDetectAllCertFiles),
		cflag(FlagDetectAllCertPKFiles),
		cflag(FlagDetectDuplicates),
		cflag(FlagShowDuplicates),
		cflag(FlagShowSpecialPerms),
		cflag(FlagChangeDataHash),
		cflag(FlagExportAllDataArtifacts),
		cflag(FlagDetectIdentities),
		cflag(FlagDetectIdentitiesParam),
		cflag(FlagDetectIdentitiesDumpRaw),
		cflag(FlagDetectScheduledTasks),
		cflag(FlagDetectScheduledTasksParam),
		cflag(FlagDetectScheduledTasksDumpRaw),
		cflag(FlagDetectServices),
		cflag(FlagDetectServicesParam),
		cflag(FlagDetectServicesDumpRaw),
		cflag(FlagDetectSystemHooks),
		cflag(FlagDetectSystemHooksParam),
		cflag(FlagDetectSystemHooksDumpRaw),
		command.Cflag(command.FlagRemoveFileArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
		if !ok || gcvalues == nil {
			return command.ErrNoGlobalParams
		}

		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		//NOTE: this is a placeholder to load all command params from a file
		_ = ctx.String(command.FlagCommandParamsFile)

		targetRef := ctx.String(command.FlagTarget)
		if targetRef == "" {
			if ctx.Args().Len() < 1 {
				xc.Out.Error("param.target", "missing image ID/name")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		detectIdentities, err := getDetectOpParam(ctx,
			FlagDetectIdentities,
			FlagDetectIdentitiesParam,
			FlagDetectIdentitiesDumpRaw,
			detectIdentitiesDumpRawDefault,
			detectIdentitiesOpParamKeys)
		if err != nil {
			xc.Out.Error("param.detect_identities", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		detectScheduledTasks, err := getDetectOpParam(ctx,
			FlagDetectScheduledTasks,
			FlagDetectScheduledTasksParam,
			FlagDetectScheduledTasksDumpRaw,
			detectScheduledTasksDumpRawDefault,
			detectScheduledTasksOpParamKeys)
		if err != nil {
			xc.Out.Error("param.detect_scheduled_tasks", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		detectServices, err := getDetectOpParam(ctx,
			FlagDetectServices,
			FlagDetectServicesParam,
			FlagDetectServicesDumpRaw,
			detectServicesDumpRawDefault,
			detectServicesOpParamKeys)
		if err != nil {
			xc.Out.Error("param.detect_services", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		detectSystemHooks, err := getDetectOpParam(ctx,
			FlagDetectSystemHooks,
			FlagDetectSystemHooksParam,
			FlagDetectSystemHooksDumpRaw,
			detectSystemHooksDumpRawDefault,
			detectSystemHooksOpParamKeys)
		if err != nil {
			xc.Out.Error("param.detect_system_hooks", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		//todo:
		//1. migrate all param fields to CommandParams
		//2. load command params from file if command.FlagCommandParamsFile is provided
		cparams := &CommandParams{
			DetectAllCertFiles:   ctx.Bool(FlagDetectAllCertFiles),
			DetectAllCertPKFiles: ctx.Bool(FlagDetectAllCertPKFiles),
			DetectIdentities:     detectIdentities,
			DetectScheduledTasks: detectScheduledTasks,
			DetectServices:       detectServices,
			DetectSystemHooks:    detectSystemHooks,
		}

		xdArtifactsPath := ctx.String(FlagExportAllDataArtifacts)

		doPull := ctx.Bool(command.FlagPull)
		dockerConfigPath := ctx.String(command.FlagDockerConfigPath)
		registryAccount := ctx.String(command.FlagRegistryAccount)
		registrySecret := ctx.String(command.FlagRegistrySecret)
		doShowPullLogs := ctx.Bool(command.FlagShowPullLogs)

		changes, err := parseChangeTypes(ctx.StringSlice(FlagChanges))
		if err != nil {
			xc.Out.Error("param.error.change.types", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if xdArtifactsPath != "" {
			//need 'changes all' when exporting all data artifacts
			changes["delete"] = struct{}{}
			changes["modify"] = struct{}{}
			changes["add"] = struct{}{}
		}

		rawChangesOutputs := ctx.StringSlice(FlagChangesOutput)
		if xdArtifactsPath != "" && len(rawChangesOutputs) == 0 {
			rawChangesOutputs = append(rawChangesOutputs, "report")
		}

		changesOutputs, err := parseChangeOutputTypes(rawChangesOutputs)
		if err != nil {
			xc.Out.Error("param.error.change.output", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if xdArtifactsPath != "" {
			changesOutputs["report"] = struct{}{}
		}

		layers, err := command.ParseTokenSet(ctx.StringSlice(FlagLayer))
		if err != nil {
			xc.Out.Error("param.error.layer", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		layerChangesMax := ctx.Int(FlagLayerChangesMax)
		allChangesMax := ctx.Int(FlagAllChangesMax)
		addChangesMax := ctx.Int(FlagAddChangesMax)
		modifyChangesMax := ctx.Int(FlagModifyChangesMax)
		deleteChangesMax := ctx.Int(FlagDeleteChangesMax)
		topChangesMax := ctx.Int(FlagTopChangesMax)

		changePathMatchers, err := parseChangePathMatchers(ctx.StringSlice(FlagChangePath))
		if err != nil {
			xc.Out.Error("param.error.change.path", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		changeDataMatchers, err := parseChangeDataMatchers(ctx.StringSlice(FlagChangeData))
		if err != nil {
			xc.Out.Error("param.error.change.data", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doAddImageManifest := ctx.Bool(FlagAddImageManifest)
		if xdArtifactsPath != "" {
			doAddImageManifest = true
		}

		doAddImageConfig := ctx.Bool(FlagAddImageConfig)
		if xdArtifactsPath != "" {
			doAddImageConfig = true
		}

		doRmFileArtifacts := ctx.Bool(command.FlagRemoveFileArtifacts)
		doReuseSavedImage := ctx.Bool(FlagReuseSavedImage)

		doHashData := ctx.Bool(FlagHashData)
		if xdArtifactsPath != "" {
			doHashData = true
		}

		doDetectDuplicates := ctx.Bool(FlagDetectDuplicates)
		if doDetectDuplicates {
			doHashData = true
		}

		rawDetectUTF8 := ctx.String(FlagDetectUTF8)
		if xdArtifactsPath != "" && rawDetectUTF8 == "" {
			rawDetectUTF8 = "dump:utf8.tgz::10000000"
		}

		utf8Detector, err := parseDetectUTF8(rawDetectUTF8)
		if err != nil {
			xc.Out.Error("param.error.detect.utf8", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		if utf8Detector != nil && !doHashData {
			xc.Out.Error("param.error.detect.utf8", "--detect-utf8 requires option --hash-data")
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doShowDuplicates := ctx.Bool(FlagShowDuplicates)
		doShowSpecialPerms := ctx.Bool(FlagShowSpecialPerms)

		changeDataHashMatchers, err := parseChangeDataHashMatchers(ctx.StringSlice(FlagChangeDataHash))
		if err != nil {
			xc.Out.Error("param.error.change.data.hash", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		changeMatchLayersOnly := ctx.Bool(FlagChangeMatchLayersOnly)

		OnCommand(
			xc,
			gcvalues,
			cparams,
			targetRef,
			doPull,
			dockerConfigPath,
			registryAccount,
			registrySecret,
			doShowPullLogs,
			changes,
			changesOutputs,
			layers,
			layerChangesMax,
			allChangesMax,
			addChangesMax,
			modifyChangesMax,
			deleteChangesMax,
			topChangesMax,
			changePathMatchers,
			changeDataMatchers,
			changeDataHashMatchers,
			doHashData,
			doDetectDuplicates,
			doShowDuplicates,
			doShowSpecialPerms,
			changeMatchLayersOnly,
			doAddImageManifest,
			doAddImageConfig,
			doReuseSavedImage,
			doRmFileArtifacts,
			utf8Detector,
			xdArtifactsPath,
		)

		return nil
	},
}

func parseChangeTypes(values []string) (map[string]struct{}, error) {
	changes := map[string]struct{}{}
	if len(values) == 0 {
		values = append(values, "all")
	}

	for _, item := range values {
		switch item {
		case "none":
			return nil, nil
		case "all":
			changes["delete"] = struct{}{}
			changes["modify"] = struct{}{}
			changes["add"] = struct{}{}
		case "delete":
			changes["delete"] = struct{}{}
		case "modify":
			changes["modify"] = struct{}{}
		case "add":
			changes["add"] = struct{}{}
		}
	}

	return changes, nil
}

func parseChangeOutputTypes(values []string) (map[string]struct{}, error) {
	outputs := map[string]struct{}{}
	if len(values) == 0 {
		values = append(values, "all")
	}

	for _, item := range values {
		switch item {
		case "all":
			outputs["report"] = struct{}{}
			outputs["console"] = struct{}{}
		case "report":
			outputs["report"] = struct{}{}
		case "console":
			outputs["console"] = struct{}{}
		}
	}

	return outputs, nil
}

func parseChangeDataMatchers(values []string) ([]*dockerimage.ChangeDataMatcher, error) {
	var matchers []*dockerimage.ChangeDataMatcher

	for _, raw := range values {
		var m dockerimage.ChangeDataMatcher

		if strings.HasPrefix(raw, "dump:") {
			parts := strings.SplitN(raw, ":", 4)
			if len(parts) != 4 {
				return nil, fmt.Errorf("malformed change data matcher: %s", raw)
			}

			m.Dump = true

			outTarget := strings.TrimSpace(parts[1])
			if len(outTarget) == 0 || outTarget == dockerimage.CDMDumpToConsole {
				m.DumpConsole = true
			} else {
				m.DumpDir = outTarget
			}

			m.PathPattern = parts[2]
			m.DataPattern = parts[3]

			//"dump:output:path_ptrn:data_regex"
			//"::path_ptrn:data_regex"
			//":::data_regex"
			//"data_regex"
		} else {
			if !strings.HasPrefix(raw, ":") {
				m.DataPattern = raw
			} else {
				parts := strings.SplitN(raw, ":", 4)
				if len(parts) != 4 {
					return nil, fmt.Errorf("malformed change data matcher: %s", raw)
				}

				m.PathPattern = parts[2]
				m.DataPattern = parts[3]
			}
		}

		matchers = append(matchers, &m)
	}

	return matchers, nil
}

func parseChangePathMatchers(values []string) ([]*dockerimage.ChangePathMatcher, error) {
	var matchers []*dockerimage.ChangePathMatcher

	for _, raw := range values {
		var m dockerimage.ChangePathMatcher

		if strings.HasPrefix(raw, "dump:") {
			parts := strings.SplitN(raw, ":", 3)
			if len(parts) != 3 {
				return nil, fmt.Errorf("malformed change path matcher: %s", raw)
			}

			m.Dump = true

			outTarget := strings.TrimSpace(parts[1])
			if len(outTarget) == 0 || outTarget == dockerimage.CDMDumpToConsole {
				m.DumpConsole = true
			} else {
				m.DumpDir = outTarget
			}

			m.PathPattern = parts[2]

			//"dump:output:path_ptrn"
			//"::path_ptrn"
			//"path_ptrn"
		} else {
			if !strings.HasPrefix(raw, ":") {
				m.PathPattern = raw
			} else {
				parts := strings.SplitN(raw, ":", 3)
				if len(parts) != 3 {
					return nil, fmt.Errorf("malformed change path matcher: %s", raw)
				}

				m.PathPattern = parts[2]
			}
		}

		matchers = append(matchers, &m)
	}

	return matchers, nil
}

func parseChangeDataHashMatchers(values []string) ([]*dockerimage.ChangeDataHashMatcher, error) {
	var matchers []*dockerimage.ChangeDataHashMatcher

	for _, raw := range values {
		var m dockerimage.ChangeDataHashMatcher

		if strings.HasPrefix(raw, "dump:") {
			parts := strings.SplitN(raw, ":", 3)
			if len(parts) != 3 {
				return nil, fmt.Errorf("malformed change data hash matcher: %s", raw)
			}

			m.Dump = true

			outTarget := strings.TrimSpace(parts[1])
			if len(outTarget) == 0 || outTarget == dockerimage.CDMDumpToConsole {
				m.DumpConsole = true
			} else {
				m.DumpDir = outTarget
			}

			m.Hash = strings.ToLower(strings.TrimSpace(parts[2]))

			//"dump:output:hash"
			//"::hash"
			//"hash"
		} else {
			if !strings.HasPrefix(raw, ":") {
				m.Hash = strings.ToLower(strings.TrimSpace(raw))
			} else {
				parts := strings.SplitN(raw, ":", 3)
				if len(parts) != 3 {
					return nil, fmt.Errorf("malformed change data hash matcher: %s", raw)
				}

				m.Hash = strings.ToLower(strings.TrimSpace(parts[2]))
			}
		}

		matchers = append(matchers, &m)
	}

	return matchers, nil
}

func parseDetectUTF8(raw string) (*dockerimage.UTF8Detector, error) {
	if raw == "" {
		return nil, nil
	}

	var detector dockerimage.UTF8Detector
	if raw == "dump" {
		detector.Dump = true
		detector.DumpConsole = true
	} else if strings.HasPrefix(raw, "dump:") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed find utf8: %s", raw)
		}

		detector.Dump = true

		outTarget := strings.TrimSpace(parts[1])
		if len(outTarget) == 0 || outTarget == dockerimage.CDMDumpToConsole {
			detector.DumpConsole = true
		} else {
			if strings.Count(outTarget, ":") == 2 {
				parts = strings.SplitN(outTarget, ":", 3)
				if len(parts) != 3 {
					return nil, fmt.Errorf("malformed find utf8: %s", raw)
				}
				outTarget = parts[0]
				_ = parts[1] // TODO implemement path pattern matcher
				maxSizeBytes := parts[2]
				var err error
				detector.MaxSizeBytes, err = strconv.Atoi(maxSizeBytes)
				if err != nil {
					return nil, err
				}
			} else if strings.Count(outTarget, ":") == 3 {
				parts = strings.SplitN(outTarget, ":", 4)
				if len(parts) != 4 {
					return nil, fmt.Errorf("malformed find utf8: %s", raw)
				}
				outTarget = parts[0]
				_ = parts[1] // TODO implemement path pattern matcher
				_ = parts[2] // TODO implemement data regex matcher
				maxSizeBytes := parts[3]
				var err error
				detector.MaxSizeBytes, err = strconv.Atoi(maxSizeBytes)
				if err != nil {
					return nil, err
				}
			}
			if strings.HasSuffix(outTarget, ".tgz") ||
				strings.HasSuffix(outTarget, ".tar.gz") {
				detector.DumpArchive = outTarget

				dar, err := dockerimage.NewTarWriter(outTarget)
				if err != nil {
					return nil, err
				}

				detector.Archive = dar
			} else {
				detector.DumpDir = outTarget
			}
		}
	} else {
		if raw != "true" {
			return nil, nil
		}
	}

	//TODO:
	//get detector filters if we need to find/extract only a subset of the utf8

	return &detector, nil
}

func getDetectOpParam(
	ctx *cli.Context,
	enableFlag string,
	kvListFlag string,
	dumpRawFlag string,
	defaultDumpRawFile string,
	validOpParamKeys map[string]struct{}) (*DetectOpParam, error) {
	var param DetectOpParam

	enable := ctx.Bool(enableFlag)
	kvList := ctx.StringSlice(kvListFlag)
	dumpRaw := ctx.String(dumpRawFlag)

	if enable == false && len(kvList) == 0 && dumpRaw == "" {
		return &param, nil
	}

	param.Enabled = true
	param.InputParams = map[string]string{}
	for _, raw := range kvList {
		raw = strings.TrimSpace(raw)
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			continue
		}

		if parts[0] == "" || parts[1] == "" {
			continue
		}

		if _, found := validOpParamKeys[parts[0]]; !found {
			continue
		}

		param.InputParams[parts[0]] = parts[1]
	}

	if dumpRaw != "" {
		param.DumpRaw = true
		switch dumpRaw {
		case ".":
			param.OutputPath = defaultDumpRawFile
			return &param, nil
		case "console":
			param.IsConsoleOut = true
			return &param, nil
		case "no":
			return &param, nil
		}

		parts := strings.SplitN(dumpRaw, ":", 2)
		if parts[1] == "" {
			param.IsConsoleOut = true
			return &param, nil
		}

		switch parts[0] {
		case "dir":
			param.IsDirOut = true
		default: //"file"
			param.OutputPath = parts[1]
		}
	}

	return &param, nil
}

const (
	detectIdentitiesDumpRawDefault     = "./raw-identities.tar"
	detectScheduledTasksDumpRawDefault = "./raw-scheduled-tasks.tar"
	detectServicesDumpRawDefault       = "./raw-services.tar"
	detectSystemHooksDumpRawDefault    = "./raw-system-hooks.tar"
)

var (
	detectIdentitiesOpParamKeys     = map[string]struct{}{} //no keys yet
	detectScheduledTasksOpParamKeys = map[string]struct{}{} //no keys yet
	detectServicesOpParamKeys       = map[string]struct{}{} //no keys yet
	detectSystemHooksOpParamKeys    = map[string]struct{}{} //no keys yet
)
