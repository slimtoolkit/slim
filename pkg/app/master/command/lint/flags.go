package lint

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/slimtoolkit/slim/pkg/app/master/command"
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
	command.FlagTarget: &cli.StringFlag{
		Name:    command.FlagTarget,
		Value:   "",
		Usage:   FlagLintTargetUsage,
		EnvVars: []string{"DSLIM_TARGET"},
	},
	FlagTargetType: &cli.StringFlag{
		Name:    FlagTargetType,
		Value:   "",
		Usage:   FlagTargetTypeUsage,
		EnvVars: []string{"DSLIM_LINT_TARGET_TYPE"},
	},
	FlagSkipBuildContext: &cli.BoolFlag{
		Name:    FlagSkipBuildContext,
		Usage:   FlagSkipBuildContextUsage,
		EnvVars: []string{"DSLIM_LINT_SKIP_BC"},
	},
	FlagBuildContextDir: &cli.StringFlag{
		Name:    FlagBuildContextDir,
		Value:   "",
		Usage:   FlagBuildContextDirUsage,
		EnvVars: []string{"DSLIM_LINT_BC_DIR"},
	},
	FlagSkipDockerignore: &cli.BoolFlag{
		Name:    FlagSkipDockerignore,
		Usage:   FlagSkipDockerignoreUsage,
		EnvVars: []string{"DSLIM_LINT_SKIP_DI"},
	},
	FlagIncludeCheckLabel: &cli.StringSliceFlag{
		Name:    FlagIncludeCheckLabel,
		Value:   cli.NewStringSlice(""),
		Usage:   FlagIncludeCheckLabelUsage,
		EnvVars: []string{"DSLIM_LINT_INCLUDE_LABEL"},
	},
	FlagExcludeCheckLabel: &cli.StringSliceFlag{
		Name:    FlagExcludeCheckLabel,
		Value:   cli.NewStringSlice(""),
		Usage:   FlagExcludeCheckLabelUsage,
		EnvVars: []string{"DSLIM_LINT_EXCLUDE_LABEL"},
	},
	FlagIncludeCheckID: &cli.StringSliceFlag{
		Name:    FlagIncludeCheckID,
		Value:   cli.NewStringSlice(""),
		Usage:   FlagIncludeCheckIDUsage,
		EnvVars: []string{"DSLIM_LINT_INCLUDE_CID"},
	},
	FlagIncludeCheckIDFile: &cli.StringFlag{
		Name:    FlagIncludeCheckIDFile,
		Value:   "",
		Usage:   FlagIncludeCheckIDFileUsage,
		EnvVars: []string{"DSLIM_LINT_INCLUDE_CID_FILE"},
	},
	FlagExcludeCheckID: &cli.StringSliceFlag{
		Name:    FlagExcludeCheckID,
		Value:   cli.NewStringSlice(""),
		Usage:   FlagExcludeCheckIDUsage,
		EnvVars: []string{"DSLIM_LINT_EXCLUDE_CID"},
	},
	FlagExcludeCheckIDFile: &cli.StringFlag{
		Name:    FlagExcludeCheckIDFile,
		Value:   "",
		Usage:   FlagExcludeCheckIDFileUsage,
		EnvVars: []string{"DSLIM_LINT_EXCLUDE_CID_FILE"},
	},
	FlagShowNoHits: &cli.BoolFlag{
		Name:    FlagShowNoHits,
		Usage:   FlagShowNoHitsUsage,
		EnvVars: []string{"DSLIM_LINT_SHOW_NOHITS"},
	},
	FlagShowSnippet: &cli.BoolFlag{
		Name:    FlagShowSnippet,
		Value:   true,
		Usage:   FlagShowSnippetUsage,
		EnvVars: []string{"DSLIM_LINT_SHOW_SNIPPET"},
	},
	FlagListChecks: &cli.BoolFlag{
		Name:    FlagListChecks,
		Usage:   FlagListChecksUsage,
		EnvVars: []string{"DSLIM_LINT_LIST_CHECKS"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
