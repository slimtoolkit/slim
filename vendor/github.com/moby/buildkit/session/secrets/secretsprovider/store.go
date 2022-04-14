package secretsprovider

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/moby/buildkit/session/secrets"
	"github.com/pkg/errors"
	"github.com/tonistiigi/units"
)

type Source struct {
	ID       string
	FilePath string
	Env      string
}

func NewStore(files []Source) (secrets.SecretStore, error) {
	m := map[string]Source{}
	for _, f := range files {
		if f.ID == "" {
			return nil, errors.Errorf("secret missing ID")
		}
		if f.Env == "" && f.FilePath == "" {
			if _, ok := os.LookupEnv(f.ID); ok {
				f.Env = f.ID
			} else {
				f.FilePath = f.ID
			}
		}
		if f.FilePath != "" {
			fi, err := os.Stat(f.FilePath)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to stat %s", f.FilePath)
			}
			if fi.Size() > MaxSecretSize {
				return nil, errors.Errorf("secret %s too big. max size %#.f", f.ID, MaxSecretSize*units.B)
			}
		}
		m[f.ID] = f
	}
	return &fileStore{
		m: m,
	}, nil
}

type fileStore struct {
	m map[string]Source
}

func (fs *fileStore) GetSecret(ctx context.Context, id string) ([]byte, error) {
	v, ok := fs.m[id]
	if !ok {
		return nil, errors.WithStack(secrets.ErrNotFound)
	}
	if v.Env != "" {
		return []byte(os.Getenv(v.Env)), nil
	}
	dt, err := ioutil.ReadFile(v.FilePath)
	if err != nil {
		return nil, err
	}
	return dt, nil
}
