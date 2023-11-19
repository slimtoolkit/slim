package imagereader

import (
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/imagebuilder"
)

type Instance struct {
	imageName       string
	nameRef         name.Reference
	imageRef        v1.Image
	exportedTarPath string
	imageConfig     *imagebuilder.ImageConfig
}

func New(imageName string) (*Instance, error) {
	logger := log.WithFields(log.Fields{
		"op":         "imagereader.New",
		"image.name": imageName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	ref, err := name.ParseReference(imageName) //, name.WeakValidation)
	if err != nil {
		logger.WithError(err).Error("name.ParseReference")
		return nil, err
	}

	//TODO/FUTURE: add other image source options (not just local Docker daemon)
	//TODO/ASAP: need to pass the 'daemon' client otherwise it'll fail if the default client isn't enough
	img, err := daemon.Image(ref)
	if err != nil {
		logger.WithError(err).Error("daemon.Image")
		return nil, err
	}

	instance := &Instance{
		imageName: imageName,
		nameRef:   ref,
		imageRef:  img,
	}

	return instance, nil
}

func (ref *Instance) ImageConfig() (*imagebuilder.ImageConfig, error) {
	logger := log.WithFields(log.Fields{
		"op":         "imagereader.Instance.ImageConfig",
		"image.name": ref.imageName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	if ref.imageConfig != nil {
		return ref.imageConfig, nil
	}

	cf, err := ref.imageRef.ConfigFile()
	if err != nil {
		logger.WithError(err).Error("v1.Image.ConfigFile")
		return nil, err
	}

	ref.imageConfig = &imagebuilder.ImageConfig{
		Created:      cf.Created.Time,
		Author:       cf.Author,
		Architecture: cf.Architecture,
		OS:           cf.OS,
		OSVersion:    cf.OSVersion,
		OSFeatures:   cf.OSFeatures,
		Variant:      cf.Variant,
		//RootFS       *RootFS   `json:"rootfs"`            //not used building images
		//History      []History `json:"history,omitempty"` //not used building images
		Container:     cf.Container,
		DockerVersion: cf.DockerVersion,
		Config: imagebuilder.RunConfig{
			User:            cf.Config.User,
			ExposedPorts:    cf.Config.ExposedPorts,
			Env:             cf.Config.Env,
			Entrypoint:      cf.Config.Entrypoint,
			Cmd:             cf.Config.Cmd,
			Volumes:         cf.Config.Volumes,
			WorkingDir:      cf.Config.WorkingDir,
			Labels:          cf.Config.Labels,
			StopSignal:      cf.Config.StopSignal,
			ArgsEscaped:     cf.Config.ArgsEscaped,
			AttachStderr:    cf.Config.AttachStderr,
			AttachStdin:     cf.Config.AttachStdin,
			AttachStdout:    cf.Config.AttachStdout,
			Domainname:      cf.Config.Domainname,
			Hostname:        cf.Config.Hostname,
			Image:           cf.Config.Image,
			OnBuild:         cf.Config.OnBuild,
			OpenStdin:       cf.Config.OpenStdin,
			StdinOnce:       cf.Config.StdinOnce,
			Tty:             cf.Config.Tty,
			NetworkDisabled: cf.Config.NetworkDisabled,
			MacAddress:      cf.Config.MacAddress,
			Shell:           cf.Config.Shell, //??
			//Healthcheck  *HealthConfig       `json:"Healthcheck,omitempty"`
		},
	}

	return ref.imageConfig, nil
}

func (ref *Instance) FreeExportedFilesystem() error {
	logger := log.WithFields(log.Fields{
		"op":         "imagereader.Instance.FreeExportedFilesystem",
		"image.name": ref.imageName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	if ref.exportedTarPath == "" {
		return nil
	}

	err := os.Remove(ref.exportedTarPath)
	ref.exportedTarPath = ""
	return err
}

func (ref *Instance) ExportFilesystem() (string, error) {
	logger := log.WithFields(log.Fields{
		"op":         "imagereader.Instance.ExportFilesystem",
		"image.name": ref.imageName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	if ref.exportedTarPath != "" {
		return ref.exportedTarPath, nil
	}

	tarFile, err := os.CreateTemp("", "image-exported-fs-*.tar")
	if err != nil {
		return "", err
	}

	defer tarFile.Close()

	err = crane.Export(ref.imageRef, tarFile)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(tarFile.Name()); err != nil {
		return "", err
	}

	ref.exportedTarPath = tarFile.Name()
	return ref.exportedTarPath, nil
}

func (ref *Instance) ExportedTarPath() string {
	return ref.exportedTarPath
}
