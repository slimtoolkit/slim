package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

// OnImageIndexCreateCommand implements the 'registry image-index-create' command
func OnImageIndexCreateCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *ImageIndexCreateCommandParams) {
	cmdName := fullCmdName(CopyCmdName)
	logger := log.WithFields(log.Fields{
		"app": appName,
		"cmd": cmdName,
		"sub": ImageIndexCreateCmdName})

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

	if len(cparams.ImageNames) == 0 {
		xc.FailOn(fmt.Errorf("no image references for image index"))
	}

	if !cparams.UseDockerCreds &&
		!(cparams.CredsAccount != "" && cparams.CredsSecret != "") {
		xc.FailOn(fmt.Errorf("missing auth params"))
	}

	remoteOpts := []remote.Option{
		remote.WithContext(context.Background()),
	}

	remoteOpts, err = ConfigureAuth(cparams.CommonCommandParams, remoteOpts)
	xc.FailOn(err)

	nameOpts := []name.Option{
		name.WeakValidation,
	}

	if cparams.InsecureRefs {
		nameOpts = append(nameOpts, name.Insecure)
	}

	imageIndexRef, err := name.ParseReference(cparams.ImageIndexName, nameOpts...)
	if err != nil {
		xc.FailOn(fmt.Errorf("malformed image index reference - %s (%v)", cparams.ImageIndexName, err))
	}

	if _, err := remote.Head(imageIndexRef, remoteOpts...); err != nil {
		if _, ok := err.(*transport.Error); ok {
			logger.Debug("no image index in registry (ok)")
		} else {
			logger.Debugf("error checking image index in registry - %v", err)
		}
	} else {
		logger.Debug("image index is already in the registry")
	}

	imageIndex := v1.ImageIndex(empty.Index)
	indexImageImgRefs := make([]mutate.IndexAddendum, 0, len(cparams.ImageNames))
	for _, imageName := range cparams.ImageNames {
		imgRef, err := name.ParseReference(imageName, nameOpts...)
		if err != nil {
			xc.FailOn(fmt.Errorf("malformed image reference - %s (%v)", imageName, err))
		}

		meta, err := remote.Get(imgRef, remoteOpts...)
		if err != nil {
			xc.FailOn(fmt.Errorf("image reference metadata get error - %s (%v)", imageName, err))
		}

		if meta.MediaType.IsImage() {
			imgMeta, err := meta.Image()
			xc.FailOn(err)

			basicImageInfo(xc, imgMeta)

			imgConfig, err := imgMeta.ConfigFile()
			if err != nil {
				xc.FailOn(err)
			}
			imgRefMeta, err := partial.Descriptor(imgMeta)
			if err != nil {
				xc.FailOn(err)
			}
			imgRefMeta.Platform = imgConfig.Platform()
			indexImageImgRefs = append(indexImageImgRefs,
				mutate.IndexAddendum{
					Add:        imgMeta,
					Descriptor: *imgRefMeta,
				})
		} else {
			xc.FailOn(fmt.Errorf("unexpected target image type - %s (%v)", imageName, meta.MediaType))
		}
	}

	if cparams.AsManifestList {
		imageIndex = mutate.IndexMediaType(imageIndex, types.DockerManifestList)
	}

	imageIndex = mutate.AppendManifests(imageIndex, indexImageImgRefs...)

	if err := remote.WriteIndex(imageIndexRef, imageIndex, remoteOpts...); err != nil {
		var terr *transport.Error
		if errors.As(err, &terr) && terr.StatusCode == http.StatusUnauthorized {
			xc.Out.Info("registry.auth.error",
				ovars{
					"message": "need to authenticate",
				})

			exitCode := -111
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
					"version":   v.Current(),
				})
			xc.Exit(exitCode)
		} else {
			xc.FailOn(fmt.Errorf("saving image index error - %s (%v)", cparams.ImageIndexName, err))
		}
	}

	indexMeta, err := remote.Index(imageIndexRef, remoteOpts...)
	if err != nil {
		xc.FailOn(fmt.Errorf("index reference metadata get error - %s (%v)", cparams.ImageIndexName, err))
	}

	indexMediaType, err := indexMeta.MediaType()
	xc.FailOn(err)

	if !indexMediaType.IsIndex() {
		xc.FailOn(fmt.Errorf("unexpected media type for index"))
	}

	indexDigest, err := indexMeta.Digest()
	xc.FailOn(err)

	indexManifest, err := indexMeta.IndexManifest()
	xc.FailOn(err)
	xc.Out.Info("index.info",
		ovars{
			"reference":                imageIndexRef,
			"digest":                   indexDigest.String(),
			"manifest.schema":          indexManifest.SchemaVersion,
			"manifest.media_type":      indexManifest.MediaType,
			"manifest.image.ref.count": len(indexManifest.Manifests),
		})

	if cparams.DumpRawManifest {
		if rm, err := indexMeta.RawManifest(); err == nil {
			//todo: reformat to pretty print
			fmt.Printf("\n\n%s\n\n", string(rm))
		}
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

func basicImageInfo(
	xc *app.ExecutionContext,
	targetImage v1.Image) {
	cn, err := targetImage.ConfigName()
	xc.FailOn(err)

	d, err := targetImage.Digest()
	xc.FailOn(err)

	cf, err := targetImage.ConfigFile()
	xc.FailOn(err)

	m, err := targetImage.Manifest()
	xc.FailOn(err)

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
