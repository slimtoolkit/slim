package openapi3

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

type Examples map[string]*ExampleRef

var _ jsonpointer.JSONPointable = (*Examples)(nil)

func (e Examples) JSONLookup(token string) (interface{}, error) {
	ref, ok := e[token]
	if ref == nil || !ok {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

// Example is specified by OpenAPI/Swagger 3.0 standard.
type Example struct {
	ExtensionProps

	Summary       string      `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description   string      `json:"description,omitempty" yaml:"description,omitempty"`
	Value         interface{} `json:"value,omitempty" yaml:"value,omitempty"`
	ExternalValue string      `json:"externalValue,omitempty" yaml:"externalValue,omitempty"`
}

func NewExample(value interface{}) *Example {
	return &Example{
		Value: value,
	}
}

func (example *Example) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(example)
}

func (example *Example) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, example)
}

func (value *Example) Validate(ctx context.Context) error {
	return nil // TODO
}
