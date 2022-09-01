package lint

import (
	"fmt"
	"strings"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/linter"
	"github.com/docker-slim/docker-slim/pkg/docker/linter/check"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

type ovars = app.OutVars

// OnCommand implements the 'lint' docker-slim command
func OnCommand(
	xc *app.ExecutionContext,
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
	xc.Out.Info("params",
		ovars{
			"target":      targetRef,
			"list.checks": doListChecks,
		})

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
		version.Print(xc, prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	if doListChecks {
		checks := linter.ListChecks()
		printLintChecks(xc, checks, appName, cmdName)
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

		printLintResults(xc, lintResults, appName, cmdName, cmdReport, doShowNoHits, doShowSnippet)
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted
	xc.Out.State("done")

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}

func printLintChecks(
	xc *app.ExecutionContext,
	checks []*check.Info,
	appName string,
	cmdName command.Type) {
	xc.Out.Info("lint.checks",
		ovars{
			"count": len(checks),
		})

	for _, info := range checks {
		xc.Out.Info("lint.check.info",
			ovars{
				"id":          info.ID,
				"name":        info.Name,
				"labels":      kvMapString(info.Labels),
				"description": info.Description,
				"url":         info.DetailsURL,
			})
	}
}

func kvMapString(m map[string]string) string {
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(pairs, ",")
}

func printLintResults(
	xc *app.ExecutionContext,
	lintResults *linter.Report,
	appName string,
	cmdName command.Type,
	cmdReport *report.LintCommand,
	doShowNoHits bool,
	doShowSnippet bool) {
	cmdReport.HitsCount = len(lintResults.Hits)
	cmdReport.NoHitsCount = len(lintResults.NoHits)
	cmdReport.ErrorsCount = len(lintResults.Errors)

	xc.Out.Info("lint.results",
		ovars{
			"hits":   cmdReport.HitsCount,
			"nohits": cmdReport.NoHitsCount,
			"errors": cmdReport.ErrorsCount,
		})

	if cmdReport.HitsCount > 0 {
		xc.Out.Info("lint.check.hits",
			ovars{
				"count": cmdReport.HitsCount,
			})

		for id, result := range lintResults.Hits {
			xc.Out.Info("lint.check.hit",
				ovars{
					"id":      id,
					"name":    result.Source.Name,
					"level":   result.Source.Labels[check.LabelLevel],
					"message": result.Message,
				})

			if len(result.Matches) > 0 {
				xc.Out.Info("lint.check.hit.matches",
					ovars{
						"count": len(result.Matches),
					})

				for _, m := range result.Matches {
					//var instructionInfo string
					//the match message has the instruction info already
					//if m.Instruction != nil {
					//	instructionInfo = fmt.Sprintf(" instruction(start=%d end=%d name=%s gindex=%d sindex=%d)",
					//		m.Instruction.StartLine,
					//		m.Instruction.EndLine,
					//		m.Instruction.Name,
					//		m.Instruction.GlobalIndex,
					//		m.Instruction.StageIndex)
					//}

					minfo := ovars{}
					if m.Stage != nil {
						minfo["stage"] = fmt.Sprintf("%d:%s", m.Stage.Index, m.Stage.Name)
					}

					minfo["message"] = m.Message
					xc.Out.Info("lint.check.hit.match", minfo)

					if m.Instruction != nil &&
						len(m.Instruction.RawLines) > 0 &&
						doShowSnippet {
						for idx, data := range m.Instruction.RawLines {
							xc.Out.Info("lint.check.hit.match.snippet",
								ovars{
									"line": idx + m.Instruction.StartLine,
									"data": data,
								})
						}
					}
				}
			}
		}
	}

	if doShowNoHits && cmdReport.NoHitsCount > 0 {
		xc.Out.Info("lint.check.nohits",
			ovars{
				"count": cmdReport.NoHitsCount,
			})

		for id, result := range lintResults.NoHits {
			xc.Out.Info("lint.check.nohit",
				ovars{
					"id":   id,
					"name": result.Source.Name,
				})
		}
	}

	if cmdReport.ErrorsCount > 0 {
		xc.Out.Info("lint.check.errors",
			ovars{
				"count": cmdReport.ErrorsCount,
			})

		for id, err := range lintResults.Errors {
			xc.Out.Info("lint.check.error",
				ovars{
					"id":      id,
					"message": err,
				})
		}
	}
}
