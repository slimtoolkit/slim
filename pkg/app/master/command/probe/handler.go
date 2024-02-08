package probe

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/probe/http"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

const appName = command.AppName

type ovars = app.OutVars

// OnCommand implements the 'probe' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	targetEndpoint string,
	targetPorts []uint,
	httpProbeOpts config.HTTPProbeOptions) {
	printState := true
	logger := log.WithFields(log.Fields{"app": appName, "cmd": Name})
	cmdName := fmt.Sprintf("cmd=%s", Name)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewProbeCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted

	xc.Out.State(cmd.StateStarted)
	xc.Out.Info("params",
		ovars{
			"target": targetEndpoint,
		})

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := -222
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	probe, err := http.NewEndpointProbe(xc, targetEndpoint, targetPorts, httpProbeOpts, printState)
	xc.FailOn(err)

	probe.Start()

	xc.Out.Prompt("waiting for the HTTP probe to finish")
	<-probe.DoneChan()
	xc.Out.Info("event",
		ovars{
			"message": "HTTP probe is done",
		})

	if probe != nil && probe.CallCount > 0 && probe.OkCount == 0 {
		xc.Out.Error("probe.error", "no.successful.calls")
	}

	xc.Out.State(cmd.StateCompleted)
	cmdReport.State = cmd.StateCompleted
	xc.Out.State(cmd.StateDone)

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = cmd.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}
