package lint

import (
	"fmt"
	"strings"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/linter"
	"github.com/docker-slim/docker-slim/pkg/docker/linter/check"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

type ovars = commands.OutVars

// OnCommand implements the 'lint' docker-slim command
func OnCommand(
	xc *commands.ExecutionContext,
	gparams *commands.GenericParams,
	targetRef string,
	targetType string,
	doSkipBuildContext bool,
	buildContextDir string,
	doSkipDockerignore bool,
	includeCheckLabels map[string]string,
	excludeCheckLabels map[string]string,
	includeCheckIDs map[string]struct{},
	excludeCheckIDs map[string]struct{},
	doShowNoHits bool,
	doShowSnippet bool,
	doListChecks bool) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewLintCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted

	xc.Out.State("started")
	fmt.Printf("cmd=%s info=params target=%v list.checks=%v\n", cmdName, targetRef, doListChecks)

	/*
		do it only when targetting images
		client, err := dockerclient.New(gparams.ClientConfig)
		if err == dockerclient.ErrNoDockerInfo {
			exitMsg := "missing Docker connection info"
			if gparams.InContainer && gparams.IsDSImage {
				exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
			}
			fmt.Printf("cmd=%s info=docker.connect.error message='%s'\n", cmdName, exitMsg)
			fmt.Printf("cmd=%s state=exited version=%s location='%s'\n", cmdName, v.Current(), fsutil.ExeDir())
			os.Exit(ectCommon | ecNoDockerConnectInfo)
		}
		errutil.FailOn(err)
	*/
	var client *dockerapi.Client

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if doListChecks {
		checks := linter.ListChecks()
		printLintChecks(checks, appName, cmdName)
	} else {
		cmdReport.TargetType = linter.DockerfileTargetType
		cmdReport.TargetReference = targetRef

		options := linter.Options{
			DockerfilePath:   targetRef,
			SkipBuildContext: doSkipBuildContext,
			BuildContextDir:  buildContextDir,
			SkipDockerignore: doSkipDockerignore,
			Selector: linter.CheckSelector{
				IncludeCheckLabels: includeCheckLabels,
				IncludeCheckIDs:    includeCheckIDs,
				ExcludeCheckLabels: excludeCheckLabels,
				ExcludeCheckIDs:    excludeCheckIDs,
			},
		}

		lintResults, err := linter.Execute(options)
		errutil.FailOn(err)

		cmdReport.BuildContextDir = lintResults.BuildContextDir
		cmdReport.Hits = lintResults.Hits
		cmdReport.Errors = lintResults.Errors

		printLintResults(lintResults, appName, cmdName, cmdReport, doShowNoHits, doShowSnippet)
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted
	xc.Out.State("done")

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		fmt.Printf("cmd=%s info=report file='%s'\n", cmdName, cmdReport.ReportLocation())
	}
}

func printLintChecks(checks []*check.Info,
	appName string,
	cmdName command.Type) {
	fmt.Printf("cmd=%s info=lint.checks count=%d:\n",
		cmdName,
		len(checks))

	for _, info := range checks {
		fmt.Printf("cmd=%s info=lint.check.info id=%s name='%s' labels='%s' description='%s' url='%s'\n",
			cmdName,
			info.ID,
			info.Name,
			kvMapString(info.Labels),
			info.Description,
			info.DetailsURL)
	}
}

func kvMapString(m map[string]string) string {
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(pairs, ",")
}

func printLintResults(lintResults *linter.Report,
	appName string,
	cmdName command.Type,
	cmdReport *report.LintCommand,
	doShowNoHits bool,
	doShowSnippet bool) {
	cmdReport.HitsCount = len(lintResults.Hits)
	cmdReport.NoHitsCount = len(lintResults.NoHits)
	cmdReport.ErrorsCount = len(lintResults.Errors)

	fmt.Printf("cmd=%s info=lint.results hits=%d nohits=%d errors=%d:\n",
		cmdName,
		cmdReport.HitsCount,
		cmdReport.NoHitsCount,
		cmdReport.ErrorsCount)

	if cmdReport.HitsCount > 0 {
		fmt.Printf("cmd=%s info=lint.check.hits count=%d\n",
			cmdName, cmdReport.HitsCount)

		for id, result := range lintResults.Hits {
			fmt.Printf("cmd=%s info=lint.check.hit id=%s name='%s' level=%s message='%s'\n",
				cmdName,
				id,
				result.Source.Name,
				result.Source.Labels[check.LabelLevel],
				result.Message)

			if len(result.Matches) > 0 {
				fmt.Printf("cmd=%s info=lint.check.hit.matches count=%d:\n",
					cmdName, len(result.Matches))

				for _, m := range result.Matches {
					var instructionInfo string
					//the match message has the instruction info already
					//if m.Instruction != nil {
					//	instructionInfo = fmt.Sprintf(" instruction(start=%d end=%d name=%s gindex=%d sindex=%d)",
					//		m.Instruction.StartLine,
					//		m.Instruction.EndLine,
					//		m.Instruction.Name,
					//		m.Instruction.GlobalIndex,
					//		m.Instruction.StageIndex)
					//}

					var stageInfo string
					if m.Stage != nil {
						stageInfo = fmt.Sprintf(" stage(index=%d name='%s')", m.Stage.Index, m.Stage.Name)
					}

					fmt.Printf("cmd=%s info=lint.check.hit.match message='%s'%s%s\n",
						cmdName, m.Message, instructionInfo, stageInfo)

					if m.Instruction != nil &&
						len(m.Instruction.RawLines) > 0 &&
						doShowSnippet {
						for idx, data := range m.Instruction.RawLines {
							fmt.Printf("cmd=%s info=lint.check.hit.match.snippet line=%d data='%s'\n",
								cmdName, idx+m.Instruction.StartLine, data)
						}
					}
				}
			}
		}
	}

	if doShowNoHits && cmdReport.NoHitsCount > 0 {
		fmt.Printf("cmd=%s info=lint.check.nohits count=%d\n",
			cmdName, cmdReport.NoHitsCount)

		for id, result := range lintResults.NoHits {
			fmt.Printf("cmd=%s info=lint.check.nohit id=%s name='%s'\n",
				cmdName, id, result.Source.Name)
		}
	}

	if cmdReport.ErrorsCount > 0 {
		fmt.Printf("cmd=%s info=lint.check.errors count=%d\n",
			cmdName, cmdReport.ErrorsCount)

		for id, err := range lintResults.Errors {
			fmt.Printf("cmd=%s info=lint.check.error id=%s message='%v'\n", cmdName, id, err)
		}
	}
}
