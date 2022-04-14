package secretsprovider

import (
	"context"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaxSecretSize is the maximum byte length allowed for a secret
const MaxSecretSize = 500 * 1024 // 500KB

func NewSecretProvider(store secrets.SecretStore) session.Attachable {
	return &secretProvider{
		store: store,
	}
}

type secretProvider struct {
	store secrets.SecretStore
}

func (sp *secretProvider) Register(server *grpc.Server) {
	secrets.RegisterSecretsServer(server, sp)
}

func (sp *secretProvider) GetSecret(ctx context.Context, req *secrets.GetSecretRequest) (*secrets.GetSecretResponse, error) {
	dt, err := sp.store.GetSecret(ctx, req.ID)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, err
	}
	if l := len(dt); l > MaxSecretSize {
		return nil, errors.Errorf("invalid secret size %d", l)
	}

	return &secrets.GetSecretResponse{
		Data: dt,
	}, nil
}

func FromMap(m map[string][]byte) session.Attachable {
	return NewSecretProvider(mapStore(m))
}

type mapStore map[string][]byte

func (m mapStore) GetSecret(ctx context.Context, id string) ([]byte, error) {
	v, ok := m[id]
	if !ok {
		return nil, errors.WithStack(secrets.ErrNotFound)
	}
	return v, nil
}
