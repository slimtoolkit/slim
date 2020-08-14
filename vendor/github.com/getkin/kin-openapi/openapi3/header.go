package openapi3

import (
	"context"

	"github.com/getkin/kin-openapi/jsoninfo"
)

type Header struct {
	ExtensionProps

	// Optional description. Should use CommonMark syntax.
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Required    bool                   `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      *SchemaRef             `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example     interface{}            `json:"example,omitempty" yaml:"example,omitempty"`
	Examples    map[string]*ExampleRef `json:"examples,omitempty" yaml:"examples,omitempty"`
	Content     Content                `json:"content,omitempty" yaml:"content,omitempty"`
}

func (value *Header) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, value)
}

func (value *Header) Validate(c context.Context) error {
	if v := value.Schema; v != nil {
		if err := v.Validate(c); err != nil {
			return err
		}
	}
	return nil
}
