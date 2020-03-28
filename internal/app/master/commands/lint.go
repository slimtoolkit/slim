package commands

import (
	"fmt"
	//"os"

	//"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/docker/linter"
	"github.com/docker-slim/docker-slim/pkg/docker/linter/check"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	//"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	//v "github.com/docker-slim/docker-slim/pkg/version"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// OnLint implements the 'lint' docker-slim command
func OnLint(
	gparams *GenericParams,
	targetRef string,
	targetType string,
	doSkipBuildContext bool,
	buildContextDir string,
	doSkipDockerignore bool,
	includeCheckLabels map[string]string,
	excludeCheckLabels map[string]string,
	includeCheckIDs map[string]struct{},
	excludeCheckIDs map[string]struct{},
	ec *ExecutionContext) {
	const cmdName = "lint"
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	prefix := fmt.Sprintf("%s[%s]:", appName, cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewLintCommand(gparams.ReportLocation)
	cmdReport.State = report.CmdStateStarted

	fmt.Printf("%s[%s]: state=started\n", appName, cmdName)
	fmt.Printf("%s[%s]: info=params target=%v\n", appName, cmdName, targetRef)

	/*
		do it only when targetting images
		client, err := dockerclient.New(gparams.ClientConfig)
		if err == dockerclient.ErrNoDockerInfo {
			exitMsg := "missing Docker connection info"
			if gparams.InContainer && gparams.IsDSImage {
				exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
			}
			fmt.Printf("%s[%s]: info=docker.connect.error message='%s'\n", appName, cmdName, exitMsg)
			fmt.Printf("%s[%s]: state=exited version=%s location='%s'\n", appName, cmdName, v.Current(), fsutil.ExeDir())
			os.Exit(ectCommon | ecNoDockerConnectInfo)
		}
		errutil.FailOn(err)
	*/
	var client *dockerapi.Client

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

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

	printLintResults(lintResults, appName, cmdName, cmdReport)

	fmt.Printf("%s[%s]: state=completed\n", appName, cmdName)
	cmdReport.State = report.CmdStateCompleted

	fmt.Printf("%s[%s]: state=done\n", appName, cmdName)

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = report.CmdStateDone
	if cmdReport.Save() {
		fmt.Printf("%s[%s]: info=report file='%s'\n", appName, cmdName, cmdReport.ReportLocation())
	}
}

func printLintResults(lintResults *linter.Report,
	appName, cmdName string,
	cmdReport *report.LintCommand) {
	fmt.Printf("%s[%s]: info=lint.results hits=%d nohits=%d errors=%d:\n",
		appName,
		cmdName,
		len(lintResults.Hits),
		len(lintResults.NoHits),
		len(lintResults.Errors))

	if len(lintResults.Hits) > 0 {
		fmt.Printf("%s[%s]: info=lint.check.hits count=%d\n",
			appName, cmdName, len(lintResults.Hits))
		for id, result := range lintResults.Hits {
			fmt.Printf("%s[%s]: info=lint.check.hit id=%s name='%s' level=%s message='%s'\n",
				appName, cmdName,
				id,
				result.Source.Name,
				result.Source.Labels[check.LabelLevel],
				result.Message)

			if len(result.Matches) > 0 {
				fmt.Printf("%s[%s]: info=lint.check.hit.matches count=%d:\n",
					appName, cmdName, len(result.Matches))

				for _, m := range result.Matches {
					var instructionInfo string
					if m.Instruction != nil {
						instructionInfo = fmt.Sprintf(" instruction(start=%d end=%d name=%s gindex=%d sindex=%d)",
							m.Instruction.StartLine,
							m.Instruction.EndLine,
							m.Instruction.Name,
							m.Instruction.GlobalIndex,
							m.Instruction.StageIndex)
					}

					var stageInfo string
					if m.Stage != nil {
						stageInfo = fmt.Sprintf(" stage(index=%d name='%s')", m.Stage.Index, m.Stage.Name)
					}

					fmt.Printf("%s[%s]: info=lint.check.hit.match message='%s'%s%s\n",
						appName, cmdName, m.Message, instructionInfo, stageInfo)
				}
			}
		}
	}

	if len(lintResults.NoHits) > 0 {
		fmt.Printf("%s[%s]: info=lint.check.nohits count=%d\n",
			appName, cmdName, len(lintResults.NoHits))

		for id, result := range lintResults.NoHits {
			fmt.Printf("%s[%s]: info=lint.check.nohit id=%s name='%s'\n",
				appName, cmdName, id, result.Source.Name)
		}
	}

	if len(lintResults.Errors) > 0 {
		fmt.Printf("%s[%s]: info=lint.check.errors count=%d: %v\n",
			appName, cmdName, len(lintResults.Errors))

		for id, err := range lintResults.Errors {
			fmt.Printf("%s[%s]: info=lint.check.error id=%s message='%v'\n", appName, cmdName, id, err)
		}
	}
}
