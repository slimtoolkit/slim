package openapi3

import (
	"context"

	"github.com/getkin/kin-openapi/jsoninfo"
)

// MediaType is specified by OpenAPI/Swagger 3.0 standard.
type MediaType struct {
	ExtensionProps

	Schema   *SchemaRef             `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example  interface{}            `json:"example,omitempty" yaml:"example,omitempty"`
	Examples map[string]*ExampleRef `json:"examples,omitempty" yaml:"examples,omitempty"`
	Encoding map[string]*Encoding   `json:"encoding,omitempty" yaml:"encoding,omitempty"`
}

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

func (mediaType *MediaType) Validate(c context.Context) error {
	if mediaType == nil {
		return nil
	}
	if schema := mediaType.Schema; schema != nil {
		if err := schema.Validate(c); err != nil {
			return err
		}
	}
	return nil
}
