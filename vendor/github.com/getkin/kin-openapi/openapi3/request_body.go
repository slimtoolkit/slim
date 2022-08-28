package openapi3

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

type RequestBodies map[string]*RequestBodyRef

var _ jsonpointer.JSONPointable = (*RequestBodyRef)(nil)

func (r RequestBodies) JSONLookup(token string) (interface{}, error) {
	ref, ok := r[token]
	if ok == false {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref != nil && ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

// RequestBody is specified by OpenAPI/Swagger 3.0 standard.
type RequestBody struct {
	ExtensionProps
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool    `json:"required,omitempty" yaml:"required,omitempty"`
	Content     Content `json:"content,omitempty" yaml:"content,omitempty"`
}

func NewRequestBody() *RequestBody {
	return &RequestBody{}
}

func (requestBody *RequestBody) WithDescription(value string) *RequestBody {
	requestBody.Description = value
	return requestBody
}

func (requestBody *RequestBody) WithRequired(value bool) *RequestBody {
	requestBody.Required = value
	return requestBody
}

func (requestBody *RequestBody) WithContent(content Content) *RequestBody {
	requestBody.Content = content
	return requestBody
}

func (requestBody *RequestBody) WithSchemaRef(value *SchemaRef, consumes []string) *RequestBody {
	requestBody.Content = NewContentWithSchemaRef(value, consumes)
	return requestBody
}

func (requestBody *RequestBody) WithSchema(value *Schema, consumes []string) *RequestBody {
	requestBody.Content = NewContentWithSchema(value, consumes)
	return requestBody
}

func (requestBody *RequestBody) WithJSONSchemaRef(value *SchemaRef) *RequestBody {
	requestBody.Content = NewContentWithJSONSchemaRef(value)
	return requestBody
}

func (requestBody *RequestBody) WithJSONSchema(value *Schema) *RequestBody {
	requestBody.Content = NewContentWithJSONSchema(value)
	return requestBody
}

func (requestBody *RequestBody) WithFormDataSchemaRef(value *SchemaRef) *RequestBody {
	requestBody.Content = NewContentWithFormDataSchemaRef(value)
	return requestBody
}

func (requestBody *RequestBody) WithFormDataSchema(value *Schema) *RequestBody {
	requestBody.Content = NewContentWithFormDataSchema(value)
	return requestBody
}

func (requestBody *RequestBody) GetMediaType(mediaType string) *MediaType {
	m := requestBody.Content
	if m == nil {
		return nil
	}
	return m[mediaType]
}

func (requestBody *RequestBody) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(requestBody)
}

func (requestBody *RequestBody) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, requestBody)
}

func (value *RequestBody) Validate(ctx context.Context) error {
	if v := value.Content; v != nil {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}
