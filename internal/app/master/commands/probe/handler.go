package probe

import (
	"fmt"

	"github.com/docker-slim/docker-slim/internal/app/master/commands"
	"github.com/docker-slim/docker-slim/internal/app/master/docker/dockerclient"
	"github.com/docker-slim/docker-slim/internal/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

type ovars = commands.OutVars

// OnCommand implements the 'probe' docker-slim command
func OnCommand(
	xc *commands.ExecutionContext,
	gparams *commands.GenericParams,
	targetRef string) {
	logger := log.WithFields(log.Fields{"app": appName, "command": Name})
	prefix := fmt.Sprintf("cmd=%s", Name)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewEditCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted

	xc.Out.State("started")
	fmt.Printf("cmd=%s info=params target=%v\n", Name, targetRef)

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}
		fmt.Printf("cmd=%s info=docker.connect.error message='%s'\n", Name, exitMsg)

		exitCode := commands.ECTCommon | commands.ECNoDockerConnectInfo
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		commands.Exit(exitCode)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(prefix, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	xc.Out.State("completed")
	cmdReport.State = command.StateCompleted
	xc.Out.State("done")

	vinfo := <-viChan
	version.PrintCheckVersion(prefix, vinfo)

	cmdReport.State = command.StateDone
	if cmdReport.Save() {
		fmt.Printf("cmd=%s info=report file='%s'\n", Name, cmdReport.ReportLocation())
	}
}
