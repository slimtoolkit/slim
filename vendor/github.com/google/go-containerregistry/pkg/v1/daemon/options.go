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
	"context"
	"io"

	"github.com/docker/docker/api/types"
	api "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// ImageOption is an alias for Option.
// Deprecated: Use Option instead.
type ImageOption Option

// Option is a functional option for daemon operations.
type Option func(*options)

type options struct {
	ctx      context.Context
	client   Client
	buffered bool
}

var defaultClient = func() (Client, error) {
	return client.NewClientWithOpts(client.FromEnv)
}

func makeOptions(opts ...Option) (*options, error) {
	o := &options{
		buffered: true,
		ctx:      context.Background(),
	}
	for _, opt := range opts {
		opt(o)
	}

	if o.client == nil {
		client, err := defaultClient()
		if err != nil {
			return nil, err
		}
		o.client = client
	}
	o.client.NegotiateAPIVersion(o.ctx)

	return o, nil
}

// WithBufferedOpener buffers the image.
func WithBufferedOpener() Option {
	return func(o *options) {
		o.buffered = true
	}
}

// WithUnbufferedOpener streams the image to avoid buffering.
func WithUnbufferedOpener() Option {
	return func(o *options) {
		o.buffered = false
	}
}

// WithClient is a functional option to allow injecting a docker client.
//
// By default, github.com/docker/docker/client.FromEnv is used.
func WithClient(client Client) Option {
	return func(o *options) {
		o.client = client
	}
}

// WithContext is a functional option to pass through a context.Context.
//
// By default, context.Background() is used.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.ctx = ctx
	}
}

// Client represents the subset of a docker client that the daemon
// package uses.
type Client interface {
	NegotiateAPIVersion(ctx context.Context)
	ImageSave(context.Context, []string) (io.ReadCloser, error)
	ImageLoad(context.Context, io.Reader, bool) (types.ImageLoadResponse, error)
	ImageTag(context.Context, string, string) error
	ImageInspectWithRaw(context.Context, string) (types.ImageInspect, []byte, error)
	ImageHistory(context.Context, string) ([]api.HistoryResponseItem, error)
}
