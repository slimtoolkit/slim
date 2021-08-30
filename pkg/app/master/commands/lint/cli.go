package lint

import (
	"github.com/urfave/cli"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
)

const (
	Name  = "lint"
	Usage = "Analyzes container instructions in Dockerfiles"
	Alias = "l"
)

var CLI = cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		cflag(commands.FlagTarget),
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
		xc := app.NewExecutionContext(Name)

		doListChecks := ctx.Bool(FlagListChecks)

		targetRef := ctx.String(commands.FlagTarget)
		if !doListChecks {
			if targetRef == "" {
				if len(ctx.Args()) < 1 {
					xc.Out.Error("param.target", "missing target Dockerfile")
					cli.ShowCommandHelp(ctx, Name)
					return nil
				} else {
					targetRef = ctx.Args().First()
				}
			}
		}

		gcvalues, err := commands.GlobalFlagValues(ctx)
		if err != nil {
			xc.Out.Error("param.global", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		targetType := ctx.String(FlagTargetType)
		doSkipBuildContext := ctx.Bool(FlagSkipBuildContext)
		buildContextDir := ctx.String(FlagBuildContextDir)
		doSkipDockerignore := ctx.Bool(FlagSkipDockerignore)

		includeCheckLabels, err := commands.ParseCheckTags(ctx.StringSlice(FlagIncludeCheckLabel))
		if err != nil {
			xc.Out.Error("param.error.invalid.include.check.labels", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		excludeCheckLabels, err := commands.ParseCheckTags(ctx.StringSlice(FlagExcludeCheckLabel))
		if err != nil {
			xc.Out.Error("param.error.invalid.exclude.check.labels", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		includeCheckIDs, err := commands.ParseTokenSet(ctx.StringSlice(FlagIncludeCheckID))
		if err != nil {
			xc.Out.Error("param.error.invalid.include.check.ids", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		includeCheckIDFile := ctx.String(FlagIncludeCheckIDFile)
		moreIncludeCheckIDs, err := commands.ParseTokenSetFile(includeCheckIDFile)
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

		excludeCheckIDs, err := commands.ParseTokenSet(ctx.StringSlice(FlagExcludeCheckID))
		if err != nil {
			xc.Out.Error("param.error.invalid.exclude.check.ids", err.Error())
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			xc.Exit(-1)
		}

		excludeCheckIDFile := ctx.String(FlagExcludeCheckIDFile)
		moreExcludeCheckIDs, err := commands.ParseTokenSetFile(excludeCheckIDFile)
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
