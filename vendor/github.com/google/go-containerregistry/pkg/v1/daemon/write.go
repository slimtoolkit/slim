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
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Tag adds a tag to an already existent image.
func Tag(src, dest name.Tag, options ...Option) error {
	o, err := makeOptions(options...)
	if err != nil {
		return err
	}

	return o.client.ImageTag(o.ctx, src.String(), dest.String())
}

// Write saves the image into the daemon as the given tag.
func Write(tag name.Tag, img v1.Image, options ...Option) (string, error) {
	o, err := makeOptions(options...)
	if err != nil {
		return "", err
	}

	// If we already have this image by this image ID, we can skip loading it.
	id, err := img.ConfigName()
	if err != nil {
		return "", fmt.Errorf("computing image ID: %w", err)
	}
	if resp, _, err := o.client.ImageInspectWithRaw(o.ctx, id.String()); err == nil {
		want := tag.String()

		// If we already have this tag, we can skip tagging it.
		for _, have := range resp.RepoTags {
			if have == want {
				return "", nil
			}
		}

		return "", o.client.ImageTag(o.ctx, id.String(), want)
	}

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(tarball.Write(tag, img, pw))
	}()

	// write the image in docker save format first, then load it
	resp, err := o.client.ImageLoad(o.ctx, pr, false)
	if err != nil {
		return "", fmt.Errorf("error loading image: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	response := string(b)
	if err != nil {
		return response, fmt.Errorf("error reading load response body: %w", err)
	}
	return response, nil
}
