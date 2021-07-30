package xray

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerimage"

	"github.com/urfave/cli"
)

const (
	Name  = "xray"
	Usage = "Shows what's inside of your container image and reverse engineers its Dockerfile"
	Alias = "x"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		commands.Cflag(commands.FlagTarget),
		commands.Cflag(commands.FlagPull),
		commands.Cflag(commands.FlagShowPullLogs),
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
		cflag(FlagDetectDuplicates),
		cflag(FlagShowDuplicates),
		cflag(FlagShowSpecialPerms),
		cflag(FlagChangeDataHash),
		commands.Cflag(commands.FlagRemoveFileArtifacts),
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		xc := commands.NewExecutionContext(Name)

		targetRef := ctx.String(commands.FlagTarget)

		if targetRef == "" {
			if len(ctx.Args()) < 1 {
				xc.Out.Error("param.target", "missing image ID/name")
				cli.ShowCommandHelp(ctx, Name)
				return nil
			} else {
				targetRef = ctx.Args().First()
			}
		}

		gcvalues, err := commands.GlobalCommandFlagValues(ctx)
		if err != nil {
			xc.Out.Error("param.global", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		doPull := ctx.Bool(commands.FlagPull)
		doShowPullLogs := ctx.Bool(commands.FlagShowPullLogs)

		changes, err := parseChangeTypes(ctx.StringSlice(FlagChanges))
		if err != nil {
			xc.Out.Error("param.error.change.types", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		changesOutputs, err := parseChangeOutputTypes(ctx.StringSlice(FlagChangesOutput))
		if err != nil {
			xc.Out.Error("param.error.change.output", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		layers, err := commands.ParseTokenSet(ctx.StringSlice(FlagLayer))
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
		doAddImageConfig := ctx.Bool(FlagAddImageConfig)
		doRmFileArtifacts := ctx.Bool(commands.FlagRemoveFileArtifacts)
		doReuseSavedImage := ctx.Bool(FlagReuseSavedImage)

		doHashData := ctx.Bool(FlagHashData)
		doDetectDuplicates := ctx.Bool(FlagDetectDuplicates)
		if doDetectDuplicates {
			doHashData = true
		}

		utf8Detector, err := parseDetectUTF8(ctx)
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
			targetRef,
			doPull,
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
		)

		commands.ShowCommunityInfo()
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

func parseDetectUTF8(ctx *cli.Context) (*dockerimage.UTF8Detector, error) {
	raw := ctx.String(FlagDetectUTF8)
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
