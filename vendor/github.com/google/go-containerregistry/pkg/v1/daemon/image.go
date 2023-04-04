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

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type image struct {
	ref          name.Reference
	opener       *imageOpener
	tarballImage v1.Image
	id           *v1.Hash

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
	if err := i.initialize(); err != nil {
		return nil, err
	}
	return i.tarballImage.ConfigFile()
}

func (i *image) RawConfigFile() ([]byte, error) {
	if err := i.initialize(); err != nil {
		return nil, err
	}
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
