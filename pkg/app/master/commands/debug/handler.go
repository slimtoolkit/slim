package debug

import (
	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	//"github.com/docker-slim/docker-slim/pkg/app/master/container"
	//"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/report"
	//"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

type ovars = app.OutVars

// OnCommand implements the 'debug' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	commandParams *CommandParams) {
	logger := log.WithFields(log.Fields{"app": appName, "command": Name})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewDebugCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted

	xc.Out.State("started")
	paramVars := ovars{
		"runtime":     commandParams.Runtime,
		"target":      commandParams.TargetRef,
		"debug-image": commandParams.DebugContainerImage,
		"entrypoint":  commandParams.Entrypoint,
		"cmd":         commandParams.Cmd,
		"terminal":    commandParams.DoTerminal,
	}

	if commandParams.Runtime == KubernetesRuntime {
		paramVars["namespace"] = commandParams.TargetNamespace
		paramVars["pod"] = commandParams.TargetPod
	}

	xc.Out.Info("params", paramVars)

	sid := generateSessionID()
	debugContainerName := generateContainerName(sid)
	logger = logger.WithFields(
		log.Fields{
			"sid":                  sid,
			"debug.container.name": debugContainerName,
		})

	switch commandParams.Runtime {
	case DockerRuntime:
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

			exitCode := commands.ECTCommon | commands.ECCNoDockerConnectInfo
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
					"version":   v.Current(),
					"location":  fsutil.ExeDir(),
				})
			xc.Exit(exitCode)
		}
		xc.FailOn(err)

		if gparams.Debug {
			version.Print(xc, Name, logger, client, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleDockerRuntime(logger, xc, gparams, commandParams, client, sid, debugContainerName)
	case KubernetesRuntime:
		if gparams.Debug {
			version.Print(xc, Name, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleKubernetesRuntime(logger, xc, gparams, commandParams, sid, debugContainerName)
	default:
		xc.Out.Error("runtime", "unsupported runtime")
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
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
