package lint

import (
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
)

const (
	Name  = "lint"
	Usage = "Analyzes container instructions in Dockerfiles"
	Alias = "l"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		cflag(command.FlagTarget),
		cflag(FlagTargetType),
		cflag(FlagSkipBuildContext),
		cflag(FlagBuildContextDir),
		cflag(FlagSkipDockerignore),
		cflag(FlagIncludeCheckLabel),
		cflag(FlagExcludeCheckLabel),
		cflag(FlagIncludeCheckID),
		cflag(FlagIncludeCheckIDFile),
		cflag(FlagExcludeCheckID),
		cflag(FlagExcludeCheckIDFile),
		cflag(FlagShowNoHits),
		cflag(FlagShowSnippet),
		cflag(FlagListChecks),
	},
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		doListChecks := ctx.Bool(FlagListChecks)

		targetRef := ctx.String(command.FlagTarget)
		if !doListChecks {
			if targetRef == "" {
				if ctx.Args().Len() < 1 {
					xc.Out.Error("param.target", "missing target Dockerfile")
					cli.ShowCommandHelp(ctx, Name)
					return nil
				} else {
					targetRef = ctx.Args().First()
				}
			}
		}

		targetType := ctx.String(FlagTargetType)
		doSkipBuildContext := ctx.Bool(FlagSkipBuildContext)
		buildContextDir := ctx.String(FlagBuildContextDir)
		doSkipDockerignore := ctx.Bool(FlagSkipDockerignore)

		includeCheckLabels, err := command.ParseCheckTags(ctx.StringSlice(FlagIncludeCheckLabel))
		if err != nil {
			xc.Out.Error("param.error.invalid.include.check.labels", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		excludeCheckLabels, err := command.ParseCheckTags(ctx.StringSlice(FlagExcludeCheckLabel))
		if err != nil {
			xc.Out.Error("param.error.invalid.exclude.check.labels", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		includeCheckIDs, err := command.ParseTokenSet(ctx.StringSlice(FlagIncludeCheckID))
		if err != nil {
			xc.Out.Error("param.error.invalid.include.check.ids", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		includeCheckIDFile := ctx.String(FlagIncludeCheckIDFile)
		moreIncludeCheckIDs, err := command.ParseTokenSetFile(includeCheckIDFile)
		if err != nil {
			xc.Out.Error("param.error.invalid.include.check.ids.from.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		for k, v := range moreIncludeCheckIDs {
			includeCheckIDs[k] = v
		}

		excludeCheckIDs, err := command.ParseTokenSet(ctx.StringSlice(FlagExcludeCheckID))
		if err != nil {
			xc.Out.Error("param.error.invalid.exclude.check.ids", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		excludeCheckIDFile := ctx.String(FlagExcludeCheckIDFile)
		moreExcludeCheckIDs, err := command.ParseTokenSetFile(excludeCheckIDFile)
		if err != nil {
			xc.Out.Error("param.error.invalid.exclude.check.ids.from.file", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		for k, v := range moreExcludeCheckIDs {
			excludeCheckIDs[k] = v
		}

		doShowNoHits := ctx.Bool(FlagShowNoHits)
		doShowSnippet := ctx.Bool(FlagShowSnippet)

		OnCommand(
			xc,
			gcvalues,
			targetRef,
			targetType,
			doSkipBuildContext,
			buildContextDir,
			doSkipDockerignore,
			includeCheckLabels,
			excludeCheckLabels,
			includeCheckIDs,
			excludeCheckIDs,
			doShowNoHits,
			doShowSnippet,
			doListChecks)

		return nil
	},
}
