package openapi3

import (
	"context"

	"github.com/getkin/kin-openapi/jsoninfo"
)

// Discriminator is specified by OpenAPI/Swagger standard version 3.0.
type Discriminator struct {
	ExtensionProps
	PropertyName string            `json:"propertyName" yaml:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty" yaml:"mapping,omitempty"`
}

func (value *Discriminator) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(value)
}

func (value *Discriminator) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, value)
}

func (value *Discriminator) Validate(ctx context.Context) error {
	return nil
}
