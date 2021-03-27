package lint

import (
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// Lint command flag names
const (
	FlagTargetType         = "target-type"
	FlagSkipBuildContext   = "skip-build-context"
	FlagBuildContextDir    = "build-context-dir"
	FlagSkipDockerignore   = "skip-dockerignore"
	FlagIncludeCheckLabel  = "include-check-label"
	FlagExcludeCheckLabel  = "exclude-check-label"
	FlagIncludeCheckID     = "include-check-id"
	FlagIncludeCheckIDFile = "include-check-id-file"
	FlagExcludeCheckID     = "exclude-check-id"
	FlagExcludeCheckIDFile = "exclude-check-id-file"
	FlagShowNoHits         = "show-nohits"
	FlagShowSnippet        = "show-snippet"
	FlagListChecks         = "list-checks"
)

// Lint command flag usage info
const (
	FlagLintTargetUsage         = "Target Dockerfile path (or container image)"
	FlagTargetTypeUsage         = "Explicitly specify the command target type (values: dockerfile, image)"
	FlagSkipBuildContextUsage   = "Don't try to analyze build context"
	FlagBuildContextDirUsage    = "Explicitly specify the build context directory"
	FlagSkipDockerignoreUsage   = "Don't try to analyze .dockerignore"
	FlagIncludeCheckLabelUsage  = "Include checks with the selected label key:value"
	FlagExcludeCheckLabelUsage  = "Exclude checks with the selected label key:value"
	FlagIncludeCheckIDUsage     = "Check ID to include"
	FlagIncludeCheckIDFileUsage = "File with check IDs to include"
	FlagExcludeCheckIDUsage     = "Check ID to exclude"
	FlagExcludeCheckIDFileUsage = "File with check IDs to exclude"
	FlagShowNoHitsUsage         = "Show checks with no matches"
	FlagShowSnippetUsage        = "Show check match snippet"
	FlagListChecksUsage         = "List available checks"
)

var Flags = map[string]cli.Flag{
	commands.FlagTarget: cli.StringFlag{
		Name:   commands.FlagTarget,
		Value:  "",
		Usage:  FlagLintTargetUsage,
		EnvVar: "DSLIM_TARGET",
	},
	FlagTargetType: cli.StringFlag{
		Name:   FlagTargetType,
		Value:  "",
		Usage:  FlagTargetTypeUsage,
		EnvVar: "DSLIM_LINT_TARGET_TYPE",
	},
	FlagSkipBuildContext: cli.BoolFlag{
		Name:   FlagSkipBuildContext,
		Usage:  FlagSkipBuildContextUsage,
		EnvVar: "DSLIM_LINT_SKIP_BC",
	},
	FlagBuildContextDir: cli.StringFlag{
		Name:   FlagBuildContextDir,
		Value:  "",
		Usage:  FlagBuildContextDirUsage,
		EnvVar: "DSLIM_LINT_BC_DIR",
	},
	FlagSkipDockerignore: cli.BoolFlag{
		Name:   FlagSkipDockerignore,
		Usage:  FlagSkipDockerignoreUsage,
		EnvVar: "DSLIM_LINT_SKIP_DI",
	},
	FlagIncludeCheckLabel: cli.StringSliceFlag{
		Name:   FlagIncludeCheckLabel,
		Value:  &cli.StringSlice{""},
		Usage:  FlagIncludeCheckLabelUsage,
		EnvVar: "DSLIM_LINT_INCLUDE_LABEL",
	},
	FlagExcludeCheckLabel: cli.StringSliceFlag{
		Name:   FlagExcludeCheckLabel,
		Value:  &cli.StringSlice{""},
		Usage:  FlagExcludeCheckLabelUsage,
		EnvVar: "DSLIM_LINT_EXCLUDE_LABEL",
	},
	FlagIncludeCheckID: cli.StringSliceFlag{
		Name:   FlagIncludeCheckID,
		Value:  &cli.StringSlice{""},
		Usage:  FlagIncludeCheckIDUsage,
		EnvVar: "DSLIM_LINT_INCLUDE_CID",
	},
	FlagIncludeCheckIDFile: cli.StringFlag{
		Name:   FlagIncludeCheckIDFile,
		Value:  "",
		Usage:  FlagIncludeCheckIDFileUsage,
		EnvVar: "DSLIM_LINT_INCLUDE_CID_FILE",
	},
	FlagExcludeCheckID: cli.StringSliceFlag{
		Name:   FlagExcludeCheckID,
		Value:  &cli.StringSlice{""},
		Usage:  FlagExcludeCheckIDUsage,
		EnvVar: "DSLIM_LINT_EXCLUDE_CID",
	},
	FlagExcludeCheckIDFile: cli.StringFlag{
		Name:   FlagExcludeCheckIDFile,
		Value:  "",
		Usage:  FlagExcludeCheckIDFileUsage,
		EnvVar: "DSLIM_LINT_EXCLUDE_CID_FILE",
	},
	FlagShowNoHits: cli.BoolFlag{
		Name:   FlagShowNoHits,
		Usage:  FlagShowNoHitsUsage,
		EnvVar: "DSLIM_LINT_SHOW_NOHITS",
	},
	FlagShowSnippet: cli.BoolTFlag{
		Name:   FlagShowSnippet,
		Usage:  FlagShowSnippetUsage,
		EnvVar: "DSLIM_LINT_SHOW_SNIPPET",
	},
	FlagListChecks: cli.BoolFlag{
		Name:   FlagListChecks,
		Usage:  FlagListChecksUsage,
		EnvVar: "DSLIM_LINT_LIST_CHECKS",
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
