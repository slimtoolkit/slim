package openapi3

import (
	"context"
	"fmt"

	"github.com/go-openapi/jsonpointer"
)

type Callbacks map[string]*CallbackRef

var _ jsonpointer.JSONPointable = (*Callbacks)(nil)

func (c Callbacks) JSONLookup(token string) (interface{}, error) {
	ref, ok := c[token]
	if ref == nil || !ok {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

// Callback is specified by OpenAPI/Swagger standard version 3.0.
type Callback map[string]*PathItem

func (value Callback) Validate(ctx context.Context) error {
	for _, v := range value {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}
