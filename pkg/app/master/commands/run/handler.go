package run

import (
	"fmt"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/container"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/image"
	"github.com/docker-slim/docker-slim/pkg/app/master/signals"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"

	log "github.com/sirupsen/logrus"
)

const appName = commands.AppName

// Run command exit codes
const (
	ecbOther = iota + 1
	ecbTarget
)

type ovars = app.OutVars

// OnCommand implements the 'run' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	cparams *CommandParams) {
	logger := log.WithFields(log.Fields{"app": appName, "command": Name})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewRunCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted
	cmdReport.TargetReference = cparams.TargetRef

	xc.Out.State("started")
	xc.Out.Info("params",
		ovars{
			"cmd.params": fmt.Sprintf("%+v", cparams),
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

		exitCode := commands.ECTCommon | commands.ECCNoDockerConnectInfo
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
		version.Print(xc, Name, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	imageInspector, err := image.NewInspector(client, cparams.TargetRef)
	errutil.FailOn(err)

	if imageInspector.NoImage() {
		if cparams.DoPull {
			xc.Out.Info("target.image",
				ovars{
					"status":  "image.not.found",
					"image":   cparams.TargetRef,
					"message": "trying to pull target image",
				})

			err := imageInspector.Pull(cparams.DoShowPullLogs, cparams.DockerConfigPath, cparams.RegistryAccount, cparams.RegistrySecret)
			errutil.FailOn(err)
		} else {
			xc.Out.Info("target.image.error",
				ovars{
					"status":  "image.not.found",
					"image":   cparams.TargetRef,
					"message": "make sure the target image already exists locally (use --pull flag to auto-download it from registry)",
				})

			exitCode := commands.ECTRun | ecbTarget
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
				})

			xc.Exit(exitCode)
		}
	}

	//refresh the target refs
	cparams.TargetRef = imageInspector.ImageRef
	cmdReport.TargetReference = imageInspector.ImageRef

	xc.Out.State("container.run.start")

	options := &container.ExecutionOptions{
		Entrypoint:   cparams.Entrypoint,
		Cmd:          cparams.Cmd,
		PublishPorts: cparams.PublishPorts,
		EnvVars:      cparams.EnvVars,
		Volumes:      cparams.Volumes,
		LiveLogs:     cparams.DoLiveLogs,
		Terminal:     cparams.DoTerminal,
	}

	if options.Terminal {
		options.LiveLogs = false
	}

	containerEventCh := make(chan *container.ExecutionEvenInfo, 10)
	exe, err := container.NewExecution(
		xc,
		logger,
		client,
		cparams.TargetRef,
		options,
		containerEventCh,
		true,
		true)

	errutil.FailOn(err)

	continueCh := make(chan struct{})
	go func() {
		for {
			select {
			case evt := <-containerEventCh:
				logger.Tracef("Exection Event: name=%s", evt.Event)
				switch evt.Event {
				case container.XEExitedCrash:
					xc.Out.Info("target.container.event",
						ovars{
							"event": evt.Event,
							"image": cparams.TargetRef,
						})

					exe.ShowContainerLogs()
					close(continueCh)
					return
				case container.XEExited:
					close(continueCh)
					return
				}
			case <-signals.AppContinueChan:
				err = exe.Stop()
				if err != nil {
					errutil.WarnOn(err)
				}

				close(continueCh)
				return
			}
		}
	}()

	err = exe.Start()
	errutil.FailOn(err)

	<-continueCh

	if cparams.DoRemoveOnExit {
		exe.Cleanup()
	}

	xc.Out.State("container.run.done")

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
