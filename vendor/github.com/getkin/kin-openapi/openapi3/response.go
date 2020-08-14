package openapi3

import (
	"context"
	"errors"
	"strconv"

	"github.com/getkin/kin-openapi/jsoninfo"
)

// Responses is specified by OpenAPI/Swagger 3.0 standard.
type Responses map[string]*ResponseRef

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

func (responses Responses) Validate(c context.Context) error {
	if len(responses) == 0 {
		return errors.New("the responses object MUST contain at least one response code")
	}
	for _, v := range responses {
		if err := v.Validate(c); err != nil {
			return err
		}
	}
	return nil
}

// Response is specified by OpenAPI/Swagger 3.0 standard.
type Response struct {
	ExtensionProps
	Description *string               `json:"description,omitempty" yaml:"description,omitempty"`
	Headers     map[string]*HeaderRef `json:"headers,omitempty" yaml:"headers,omitempty"`
	Content     Content               `json:"content,omitempty" yaml:"content,omitempty"`
	Links       map[string]*LinkRef   `json:"links,omitempty" yaml:"links,omitempty"`
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

func (response *Response) Validate(c context.Context) error {
	if response.Description == nil {
		return errors.New("a short description of the response is required")
	}

	if content := response.Content; content != nil {
		if err := content.Validate(c); err != nil {
			return err
		}
	}
	return nil
}
