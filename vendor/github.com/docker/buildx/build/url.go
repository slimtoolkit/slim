package build

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"github.com/docker/buildx/driver"
	"github.com/docker/buildx/util/progress"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	gwclient "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/pkg/errors"
)

func createTempDockerfileFromURL(ctx context.Context, d driver.Driver, url string, pw progress.Writer) (string, error) {
	c, err := driver.Boot(ctx, ctx, d, pw)
	if err != nil {
		return "", err
	}
	var out string
	ch, done := progress.NewChannel(pw)
	defer func() { <-done }()
	_, err = c.Build(ctx, client.SolveOpt{}, "buildx", func(ctx context.Context, c gwclient.Client) (*gwclient.Result, error) {
		def, err := llb.HTTP(url, llb.Filename("Dockerfile"), llb.WithCustomNamef("[internal] load %s", url)).Marshal(ctx)
		if err != nil {
			return nil, err
		}

		res, err := c.Solve(ctx, gwclient.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, err
		}
		ref, err := res.SingleRef()
		if err != nil {
			return nil, err
		}
		stat, err := ref.StatFile(ctx, gwclient.StatRequest{
			Path: "Dockerfile",
		})
		if err != nil {
			return nil, err
		}
		if stat.Size() > 512*1024 {
			return nil, errors.Errorf("Dockerfile %s bigger than allowed max size", url)
		}

		dt, err := ref.ReadFile(ctx, gwclient.ReadRequest{
			Filename: "Dockerfile",
		})
		if err != nil {
			return nil, err
		}
		dir, err := ioutil.TempDir("", "buildx")
		if err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(filepath.Join(dir, "Dockerfile"), dt, 0600); err != nil {
			return nil, err
		}
		out = dir
		return nil, nil
	}, ch)

	if err != nil {
		return "", err
	}
	return out, nil
}
