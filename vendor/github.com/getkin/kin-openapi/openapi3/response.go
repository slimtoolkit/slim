package openapi3

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

// Responses is specified by OpenAPI/Swagger 3.0 standard.
type Responses map[string]*ResponseRef

var _ jsonpointer.JSONPointable = (*Responses)(nil)

func NewResponses() Responses {
	r := make(Responses)
	r["default"] = &ResponseRef{Value: NewResponse().WithDescription("")}
	return r
}

func (responses Responses) Default() *ResponseRef {
	return responses["default"]
}

func (responses Responses) Get(status int) *ResponseRef {
	return responses[strconv.FormatInt(int64(status), 10)]
}

func (value Responses) Validate(ctx context.Context) error {
	if len(value) == 0 {
		return errors.New("the responses object MUST contain at least one response code")
	}
	for _, v := range value {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (responses Responses) JSONLookup(token string) (interface{}, error) {
	ref, ok := responses[token]
	if ok == false {
		return nil, fmt.Errorf("invalid token reference: %q", token)
	}

	if ref != nil && ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

// Response is specified by OpenAPI/Swagger 3.0 standard.
type Response struct {
	ExtensionProps
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	Headers     Headers `json:"headers,omitempty" yaml:"headers,omitempty"`
	Content     Content `json:"content,omitempty" yaml:"content,omitempty"`
	Links       Links   `json:"links,omitempty" yaml:"links,omitempty"`
}

func NewResponse() *Response {
	return &Response{}
}

func (response *Response) WithDescription(value string) *Response {
	response.Description = &value
	return response
}

func (response *Response) WithContent(content Content) *Response {
	response.Content = content
	return response
}

func (response *Response) WithJSONSchema(schema *Schema) *Response {
	response.Content = NewContentWithJSONSchema(schema)
	return response
}

func (response *Response) WithJSONSchemaRef(schema *SchemaRef) *Response {
	response.Content = NewContentWithJSONSchemaRef(schema)
	return response
}

func (response *Response) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(response)
}

func (response *Response) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, response)
}

func (value *Response) Validate(ctx context.Context) error {
	if value.Description == nil {
		return errors.New("a short description of the response is required")
	}

	if content := value.Content; content != nil {
		if err := content.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}
