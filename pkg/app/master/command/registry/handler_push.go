package registry

import (
	"context"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/command"
	"github.com/slimtoolkit/slim/pkg/app/master/version"
	cmd "github.com/slimtoolkit/slim/pkg/command"
	"github.com/slimtoolkit/slim/pkg/docker/dockerclient"
	"github.com/slimtoolkit/slim/pkg/report"
	"github.com/slimtoolkit/slim/pkg/util/fsutil"
	v "github.com/slimtoolkit/slim/pkg/version"
)

// OnPushCommand implements the 'registry push' command
func OnPushCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *PushCommandParams) {
	cmdName := fullCmdName(PushCmdName)
	logger := log.WithFields(log.Fields{
		"app": appName,
		"cmd": cmdName,
		"sub": PushCmdName})

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
	xc.FailOn(err)

	if gparams.Debug {
		version.Print(xc, cmdName, logger, client, false, gparams.InContainer, gparams.IsDSImage)
	}

	remoteOpts := []remote.Option{
		remote.WithContext(context.Background()),
	}
	remoteOpts, err = ConfigureAuth(cparams.CommonCommandParams, remoteOpts)
	xc.FailOn(err)

	nameOpts := []name.Option{
		name.WeakValidation,
		name.Insecure,
	}

	//todo: add support for other target types too
	if cparams.TargetType == ttDocker {
		tarPath, err := uniqueTarFilePath()
		xc.FailOn(err)

		err = saveDockerImage(cparams.TargetRef, tarPath, nameOpts)
		xc.FailOn(err)

		remoteImageName := cparams.TargetRef
		if cparams.AsTag != "" {
			remoteImageName = cparams.AsTag
		}

		err = pushImageFromTar(tarPath, remoteImageName, nameOpts, remoteOpts)
		xc.FailOn(err)
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

func uniqueTarFilePath() (string, error) {
	f, err := os.CreateTemp("", "saved-image-*.tar")
	if err != nil {
		return "", err
	}

	defer f.Close()
	defer os.Remove(f.Name())
	return f.Name(), nil
}

func saveDockerImage(
	localImageName string,
	tarPath string,
	nameOpts []name.Option) error {
	ref, err := name.ParseReference(localImageName, nameOpts...)
	if err != nil {
		return err
	}

	img, err := daemon.Image(ref)
	if err != nil {
		return err
	}

	if err := tarball.WriteToFile(tarPath, ref, img); err != nil {
		return err
	}

	return nil
}

func pushImageFromTar(
	tarPath string,
	remoteImageName string,
	nameOpts []name.Option,
	remoteOpts []remote.Option) error {
	ref, err := name.ParseReference(remoteImageName, nameOpts...)
	if err != nil {
		return err
	}

	img, err := tarball.ImageFromPath(tarPath, nil)
	if err != nil {
		return err
	}

	err = remote.Write(ref, img, remoteOpts...)
	if err != nil {
		return err
	}

	return nil
}
