// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	api "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type image struct {
	ref          name.Reference
	opener       *imageOpener
	tarballImage v1.Image
	computed     bool
	id           *v1.Hash
	configFile   *v1.ConfigFile

	once sync.Once
	err  error
}

type imageOpener struct {
	ref name.Reference
	ctx context.Context

	buffered bool
	client   Client

	once  sync.Once
	bytes []byte
	err   error
}

func (i *imageOpener) saveImage() (io.ReadCloser, error) {
	return i.client.ImageSave(i.ctx, []string{i.ref.Name()})
}

func (i *imageOpener) bufferedOpener() (io.ReadCloser, error) {
	// Store the tarball in memory and return a new reader into the bytes each time we need to access something.
	i.once.Do(func() {
		i.bytes, i.err = func() ([]byte, error) {
			rc, err := i.saveImage()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			return io.ReadAll(rc)
		}()
	})

	// Wrap the bytes in a ReadCloser so it looks like an opened file.
	return io.NopCloser(bytes.NewReader(i.bytes)), i.err
}

func (i *imageOpener) opener() tarball.Opener {
	if i.buffered {
		return i.bufferedOpener
	}

	// To avoid storing the tarball in memory, do a save every time we need to access something.
	return i.saveImage
}

// Image provides access to an image reference from the Docker daemon,
// applying functional options to the underlying imageOpener before
// resolving the reference into a v1.Image.
func Image(ref name.Reference, options ...Option) (v1.Image, error) {
	o, err := makeOptions(options...)
	if err != nil {
		return nil, err
	}

	i := &imageOpener{
		ref:      ref,
		buffered: o.buffered,
		client:   o.client,
		ctx:      o.ctx,
	}

	img := &image{
		ref:    ref,
		opener: i,
	}

	// Eagerly fetch Image ID to ensure it actually exists.
	// https://github.com/google/go-containerregistry/issues/1186
	id, err := img.ConfigName()
	if err != nil {
		return nil, err
	}
	img.id = &id

	return img, nil
}

func (i *image) initialize() error {
	// Don't re-initialize tarball if already initialized.
	if i.tarballImage == nil {
		i.once.Do(func() {
			i.tarballImage, i.err = tarball.Image(i.opener.opener(), nil)
		})
	}
	return i.err
}

func (i *image) compute() error {
	// Don't re-compute if already computed.
	if i.computed {
		return nil
	}

	inspect, _, err := i.opener.client.ImageInspectWithRaw(i.opener.ctx, i.ref.String())
	if err != nil {
		return err
	}

	configFile, err := i.computeConfigFile(inspect)
	if err != nil {
		return err
	}

	i.configFile = configFile
	i.computed = true

	return nil
}

func (i *image) Layers() ([]v1.Layer, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}
	return i.tarballImage.Layers()
}

func (i *image) MediaType() (types.MediaType, error) {
	if err := i.initialize(); err != nil {
		return "", err
	}
	return i.tarballImage.MediaType()
}

func (i *image) Size() (int64, error) {
	if err := i.initialize(); err != nil {
		return 0, err
	}
	return i.tarballImage.Size()
}

func (i *image) ConfigName() (v1.Hash, error) {
	if i.id != nil {
		return *i.id, nil
	}
	res, _, err := i.opener.client.ImageInspectWithRaw(i.opener.ctx, i.ref.String())
	if err != nil {
		return v1.Hash{}, err
	}
	return v1.NewHash(res.ID)
}

func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	if err := i.compute(); err != nil {
		return nil, err
	}
	return i.configFile.DeepCopy(), nil
}

func (i *image) RawConfigFile() ([]byte, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}

	// RawConfigFile cannot be generated from "docker inspect" because Docker Engine API returns serialized data,
	// and formatting information of the raw config such as indent and prefix will be lost.
	return i.tarballImage.RawConfigFile()
}

func (i *image) Digest() (v1.Hash, error) {
	if err := i.initialize(); err != nil {
		return v1.Hash{}, err
	}
	return i.tarballImage.Digest()
}

func (i *image) Manifest() (*v1.Manifest, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}
	return i.tarballImage.Manifest()
}

func (i *image) RawManifest() ([]byte, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}
	return i.tarballImage.RawManifest()
}

func (i *image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}
	return i.tarballImage.LayerByDigest(h)
}

func (i *image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}
	return i.tarballImage.LayerByDiffID(h)
}

func (i *image) configHistory(author string) ([]v1.History, error) {
	historyItems, err := i.opener.client.ImageHistory(i.opener.ctx, i.ref.String())
	if err != nil {
		return nil, err
	}

	history := make([]v1.History, len(historyItems))
	for j, h := range historyItems {
		history[j] = v1.History{
			Author: author,
			Created: v1.Time{
				Time: time.Unix(h.Created, 0).UTC(),
			},
			CreatedBy:  h.CreatedBy,
			Comment:    h.Comment,
			EmptyLayer: h.Size == 0,
		}
	}
	return history, nil
}

func (i *image) diffIDs(rootFS api.RootFS) ([]v1.Hash, error) {
	diffIDs := make([]v1.Hash, len(rootFS.Layers))
	for j, l := range rootFS.Layers {
		h, err := v1.NewHash(l)
		if err != nil {
			return nil, err
		}
		diffIDs[j] = h
	}
	return diffIDs, nil
}

func (i *image) computeConfigFile(inspect api.ImageInspect) (*v1.ConfigFile, error) {
	diffIDs, err := i.diffIDs(inspect.RootFS)
	if err != nil {
		return nil, err
	}

	history, err := i.configHistory(inspect.Author)
	if err != nil {
		return nil, err
	}

	created, err := time.Parse(time.RFC3339Nano, inspect.Created)
	if err != nil {
		return nil, err
	}

	return &v1.ConfigFile{
		Architecture:  inspect.Architecture,
		Author:        inspect.Author,
		Container:     inspect.Container,
		Created:       v1.Time{Time: created},
		DockerVersion: inspect.DockerVersion,
		History:       history,
		OS:            inspect.Os,
		RootFS: v1.RootFS{
			Type:    inspect.RootFS.Type,
			DiffIDs: diffIDs,
		},
		Config:    i.computeImageConfig(inspect.Config),
		OSVersion: inspect.OsVersion,
	}, nil
}

func (i *image) computeImageConfig(config *container.Config) v1.Config {
	if config == nil {
		return v1.Config{}
	}

	c := v1.Config{
		AttachStderr:    config.AttachStderr,
		AttachStdin:     config.AttachStdin,
		AttachStdout:    config.AttachStdout,
		Cmd:             config.Cmd,
		Domainname:      config.Domainname,
		Entrypoint:      config.Entrypoint,
		Env:             config.Env,
		Hostname:        config.Hostname,
		Image:           config.Image,
		Labels:          config.Labels,
		OnBuild:         config.OnBuild,
		OpenStdin:       config.OpenStdin,
		StdinOnce:       config.StdinOnce,
		Tty:             config.Tty,
		User:            config.User,
		Volumes:         config.Volumes,
		WorkingDir:      config.WorkingDir,
		ArgsEscaped:     config.ArgsEscaped,
		NetworkDisabled: config.NetworkDisabled,
		MacAddress:      config.MacAddress,
		StopSignal:      config.StopSignal,
		Shell:           config.Shell,
	}

	if config.Healthcheck != nil {
		c.Healthcheck = &v1.HealthConfig{
			Test:        config.Healthcheck.Test,
			Interval:    config.Healthcheck.Interval,
			Timeout:     config.Healthcheck.Timeout,
			StartPeriod: config.Healthcheck.StartPeriod,
			Retries:     config.Healthcheck.Retries,
		}
	}

	if len(config.ExposedPorts) > 0 {
		c.ExposedPorts = map[string]struct{}{}
		for port := range c.ExposedPorts {
			c.ExposedPorts[port] = struct{}{}
		}
	}

	return c
}
