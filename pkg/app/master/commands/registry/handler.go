package registry

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gocrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/version"
	"github.com/docker-slim/docker-slim/pkg/command"
	"github.com/docker-slim/docker-slim/pkg/docker/dockerclient"
	"github.com/docker-slim/docker-slim/pkg/report"
	"github.com/docker-slim/docker-slim/pkg/util/errutil"
	"github.com/docker-slim/docker-slim/pkg/util/fsutil"
	v "github.com/docker-slim/docker-slim/pkg/version"
)

const appName = commands.AppName

type ovars = app.OutVars

// OnPullCommand implements the 'registry pull' command
func OnPullCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams,
	cparams *PullCommandParams) {
	cmdName := fullCmdName(PullCmdName)
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewRegistryCommand(gparams.ReportLocation, gparams.InContainer)
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
			exitMsg = "make sure to pass the Docker connect parameters to the docker-slim container"
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
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	//todo: pass a custom client to Pull (based on `client` above)
	targetImage, err := crane.Pull(cparams.TargetRef)
	errutil.FailOn(err)
	outImageInfo(xc, targetImage)

	if cparams.SaveToDocker {
		xc.Out.State("save.docker.start")

		tag, err := name.NewTag(cparams.TargetRef)
		errutil.FailOn(err)

		rawResponse, err := daemon.Write(tag, targetImage)
		errutil.FailOn(err)
		logger.Tracef("Image save to Docker response: %v", rawResponse)

		xc.Out.State("save.docker.done")
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

// OnPushCommand implements the 'registry push' docker-slim command
func OnPushCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams) {
	cmdName := fullCmdName(PushCmdName)
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	Name := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewRegistryCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted

	xc.Out.State("started")

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

// OnCopyCommand implements the 'registry copy' docker-slim command
func OnCopyCommand(
	xc *app.ExecutionContext,
	gparams *commands.GenericParams) {
	cmdName := fullCmdName(CopyCmdName)
	logger := log.WithFields(log.Fields{"app": appName, "command": cmdName})
	Name := fmt.Sprintf("cmd=%s", cmdName)

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewRegistryCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = command.StateStarted

	xc.Out.State("started")

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

func outImageInfo(
	xc *app.ExecutionContext,
	targetImage gocrv1.Image) {
	cn, err := targetImage.ConfigName()
	errutil.FailOn(err)

	d, err := targetImage.Digest()
	errutil.FailOn(err)

	cf, err := targetImage.ConfigFile()
	errutil.FailOn(err)

	m, err := targetImage.Manifest()
	errutil.FailOn(err)

	xc.Out.Info("image.info",
		ovars{
			"id":                         fmt.Sprintf("%s:%s", cn.Algorithm, cn.Hex),
			"digest":                     fmt.Sprintf("%s:%s", d.Algorithm, d.Hex),
			"architecture":               cf.Architecture,
			"os":                         cf.OS,
			"manifest.schema":            m.SchemaVersion,
			"manifest.media_type":        m.MediaType,
			"manifest.config.media_type": m.Config.MediaType,
			"manifest.config.size":       fmt.Sprintf("%v", m.Config.Size),
			"manifest.config.digest":     fmt.Sprintf("%s:%s", m.Config.Digest.Algorithm, m.Config.Digest.Hex),
			"manifest.layers.count":      fmt.Sprintf("%v", len(m.Layers)),
		})
}
