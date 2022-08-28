package openapi3

import (
	"context"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

// MediaType is specified by OpenAPI/Swagger 3.0 standard.
type MediaType struct {
	ExtensionProps

	Schema   *SchemaRef           `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example  interface{}          `json:"example,omitempty" yaml:"example,omitempty"`
	Examples Examples             `json:"examples,omitempty" yaml:"examples,omitempty"`
	Encoding map[string]*Encoding `json:"encoding,omitempty" yaml:"encoding,omitempty"`
}

var _ jsonpointer.JSONPointable = (*MediaType)(nil)

func NewMediaType() *MediaType {
	return &MediaType{}
}

func (mediaType *MediaType) WithSchema(schema *Schema) *MediaType {
	if schema == nil {
		mediaType.Schema = nil
	} else {
		mediaType.Schema = &SchemaRef{Value: schema}
	}
	return mediaType
}

func (mediaType *MediaType) WithSchemaRef(schema *SchemaRef) *MediaType {
	mediaType.Schema = schema
	return mediaType
}

func (mediaType *MediaType) WithExample(name string, value interface{}) *MediaType {
	example := mediaType.Examples
	if example == nil {
		example = make(map[string]*ExampleRef)
		mediaType.Examples = example
	}
	example[name] = &ExampleRef{
		Value: NewExample(value),
	}
	return mediaType
}

func (mediaType *MediaType) WithEncoding(name string, enc *Encoding) *MediaType {
	encoding := mediaType.Encoding
	if encoding == nil {
		encoding = make(map[string]*Encoding)
		mediaType.Encoding = encoding
	}
	encoding[name] = enc
	return mediaType
}

func (mediaType *MediaType) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(mediaType)
}

func (mediaType *MediaType) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, mediaType)
}

func (value *MediaType) Validate(ctx context.Context) error {
	if value == nil {
		return nil
	}
	if schema := value.Schema; schema != nil {
		if err := schema.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (mediaType MediaType) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "schema":
		if mediaType.Schema != nil {
			if mediaType.Schema.Ref != "" {
				return &Ref{Ref: mediaType.Schema.Ref}, nil
			}
			return mediaType.Schema.Value, nil
		}
	case "example":
		return mediaType.Example, nil
	case "examples":
		return mediaType.Examples, nil
	case "encoding":
		return mediaType.Encoding, nil
	}
	v, _, err := jsonpointer.GetForToken(mediaType.ExtensionProps, token)
	return v, err
}
