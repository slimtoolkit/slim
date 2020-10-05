package app

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"

	"github.com/urfave/cli"
)

var cmdLint = cli.Command{
	Name:    cmdSpecs[CmdLint].name,
	Aliases: []string{cmdSpecs[CmdLint].alias},
	Usage:   cmdSpecs[CmdLint].usage,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:   FlagTarget,
			Value:  "",
			Usage:  FlagLintTargetUsage,
			EnvVar: "DSLIM_TARGET",
		},
		cli.StringFlag{
			Name:   FlagTargetType,
			Value:  "",
			Usage:  FlagTargetTypeUsage,
			EnvVar: "DSLIM_LINT_TARGET_TYPE",
		},
		cli.BoolFlag{
			Name:   FlagSkipBuildContext,
			Usage:  FlagSkipBuildContextUsage,
			EnvVar: "DSLIM_LINT_SKIP_BC",
		},
		cli.StringFlag{
			Name:   FlagBuildContextDir,
			Value:  "",
			Usage:  FlagBuildContextDirUsage,
			EnvVar: "DSLIM_LINT_BC_DIR",
		},
		cli.BoolFlag{
			Name:   FlagSkipDockerignore,
			Usage:  FlagSkipDockerignoreUsage,
			EnvVar: "DSLIM_LINT_SKIP_DI",
		},
		cli.StringSliceFlag{
			Name:   FlagIncludeCheckLabel,
			Value:  &cli.StringSlice{""},
			Usage:  FlagIncludeCheckLabelUsage,
			EnvVar: "DSLIM_LINT_INCLUDE_LABEL",
		},
		cli.StringSliceFlag{
			Name:   FlagExcludeCheckLabel,
			Value:  &cli.StringSlice{""},
			Usage:  FlagExcludeCheckLabelUsage,
			EnvVar: "DSLIM_LINT_EXCLUDE_LABEL",
		},
		cli.StringSliceFlag{
			Name:   FlagIncludeCheckID,
			Value:  &cli.StringSlice{""},
			Usage:  FlagIncludeCheckIDUsage,
			EnvVar: "DSLIM_LINT_INCLUDE_CID",
		},
		cli.StringFlag{
			Name:   FlagIncludeCheckIDFile,
			Value:  "",
			Usage:  FlagIncludeCheckIDFileUsage,
			EnvVar: "DSLIM_LINT_INCLUDE_CID_FILE",
		},
		cli.StringSliceFlag{
			Name:   FlagExcludeCheckID,
			Value:  &cli.StringSlice{""},
			Usage:  FlagExcludeCheckIDUsage,
			EnvVar: "DSLIM_LINT_EXCLUDE_CID",
		},
		cli.StringFlag{
			Name:   FlagExcludeCheckIDFile,
			Value:  "",
			Usage:  FlagExcludeCheckIDFileUsage,
			EnvVar: "DSLIM_LINT_EXCLUDE_CID_FILE",
		},
		cli.BoolFlag{
			Name:   FlagShowNoHits,
			Usage:  FlagShowNoHitsUsage,
			EnvVar: "DSLIM_LINT_SHOW_NOHITS",
		},
		cli.BoolTFlag{
			Name:   FlagShowSnippet,
			Usage:  FlagShowSnippetUsage,
			EnvVar: "DSLIM_LINT_SHOW_SNIPPET",
		},
		cli.BoolFlag{
			Name:   FlagListChecks,
			Usage:  FlagListChecksUsage,
			EnvVar: "DSLIM_LINT_LIST_CHECKS",
		},
	},
	Action: func(ctx *cli.Context) error {
		commands.ShowCommunityInfo()
		doListChecks := ctx.Bool(FlagListChecks)

		targetRef := ctx.String(FlagTarget)
		if !doListChecks {
			if targetRef == "" {
				if len(ctx.Args()) < 1 {
					fmt.Printf("docker-slim[lint]: missing target image/Dockerfile...\n\n")
					cli.ShowCommandHelp(ctx, CmdLint)
					return nil
				} else {
					targetRef = ctx.Args().First()
				}
			}
		}

		gcvalues, err := globalCommandFlagValues(ctx)
		if err != nil {
			return err
		}

		targetType := ctx.String(FlagTargetType)
		doSkipBuildContext := ctx.Bool(FlagSkipBuildContext)
		buildContextDir := ctx.String(FlagBuildContextDir)
		doSkipDockerignore := ctx.Bool(FlagSkipDockerignore)

		includeCheckLabels, err := parseCheckTags(ctx.StringSlice(FlagIncludeCheckLabel))
		if err != nil {
			fmt.Printf("docker-slim[lint]: invalid include check labels: %v\n", err)
			return err
		}

		excludeCheckLabels, err := parseCheckTags(ctx.StringSlice(FlagExcludeCheckLabel))
		if err != nil {
			fmt.Printf("docker-slim[lint]: invalid exclude check labels: %v\n", err)
			return err
		}

		includeCheckIDs, err := parseTokenSet(ctx.StringSlice(FlagIncludeCheckID))
		if err != nil {
			fmt.Printf("docker-slim[lint]: invalid include check IDs: %v\n", err)
			return err
		}

		includeCheckIDFile := ctx.String(FlagIncludeCheckIDFile)
		moreIncludeCheckIDs, err := parseTokenSetFile(includeCheckIDFile)
		if err != nil {
			fmt.Printf("docker-slim[lint]: invalid include check IDs from file(%v): %v\n", includeCheckIDFile, err)
			return err
		}

		for k, v := range moreIncludeCheckIDs {
			includeCheckIDs[k] = v
		}

		excludeCheckIDs, err := parseTokenSet(ctx.StringSlice(FlagExcludeCheckID))
		if err != nil {
			fmt.Printf("docker-slim[lint]: invalid exclude check IDs: %v\n", err)
			return err
		}

		excludeCheckIDFile := ctx.String(FlagExcludeCheckIDFile)
		moreExcludeCheckIDs, err := parseTokenSetFile(excludeCheckIDFile)
		if err != nil {
			fmt.Printf("docker-slim[lint]: invalid exclude check IDs from file(%v): %v\n", excludeCheckIDFile, err)
			return err
		}

		for k, v := range moreExcludeCheckIDs {
			excludeCheckIDs[k] = v
		}

		doShowNoHits := ctx.Bool(FlagShowNoHits)
		doShowSnippet := ctx.Bool(FlagShowSnippet)

		ec := &commands.ExecutionContext{}

		commands.OnLint(
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
			doListChecks,
			ec)
		commands.ShowCommunityInfo()
		return nil
	},
}
