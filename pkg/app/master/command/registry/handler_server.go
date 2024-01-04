package registry

import (
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	stdlog "log"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/errutil"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

// OnServerCommand implements the 'registry server' command
func OnServerCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *ServerCommandParams) {
	cmdName := fullCmdName(ServerCmdName)
	logger := log.WithFields(log.Fields{
		"app": appName,
		"cmd": cmdName,
		"sub": ServerCmdName})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewRegistryCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted

	xc.Out.State(cmd.StateStarted)

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := command.ECTCommon | command.ECCNoDockerConnectInfo
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

	opts := []registry.Option{registry.Logger(stdlog.New(logger.Logger.Out, "", stdlog.LstdFlags))}
	if cparams.EnableReferrersAPI {
		opts = append(opts, registry.WithReferrersSupport(true))
	}

	l, err := net.Listen("tcp", "127.0.0.1:5000")
	errutil.FailOn(err)

	xc.Out.Message("Starting registry server on port 5000")

	// We want to print the command report and the version check report after the command has executed
	// The command execution is considered to be successful when the server has successfully started up

	// As http.Serve() is a blocking call, we need to run the reporting tasks in a separate goroutine
	// To achieve this, we wait for 3 seconds for the server to start up and then print the reports
	// In case the server fails to start up, the process will exit and the reports will not be printed
	// This is a hack, but it works for now
	go func() {
		time.Sleep(3 * time.Second)

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
	}()

	err = http.Serve(l, registry.New(opts...))
	errutil.FailOn(err)
}
