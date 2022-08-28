package openapi3

import (
	"context"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

// Ref is specified by OpenAPI/Swagger 3.0 standard.
type Ref struct {
	Ref string `json:"$ref" yaml:"$ref"`
}

type CallbackRef struct {
	Ref   string
	Value *Callback
}

var _ jsonpointer.JSONPointable = (*CallbackRef)(nil)

func (value *CallbackRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *CallbackRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *CallbackRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value CallbackRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type ExampleRef struct {
	Ref   string
	Value *Example
}

var _ jsonpointer.JSONPointable = (*ExampleRef)(nil)

func (value *ExampleRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *ExampleRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *ExampleRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value ExampleRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type HeaderRef struct {
	Ref   string
	Value *Header
}

var _ jsonpointer.JSONPointable = (*HeaderRef)(nil)

func (value *HeaderRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *HeaderRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *HeaderRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value HeaderRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type LinkRef struct {
	Ref   string
	Value *Link
}

func (value *LinkRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *LinkRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *LinkRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

type ParameterRef struct {
	Ref   string
	Value *Parameter
}

var _ jsonpointer.JSONPointable = (*ParameterRef)(nil)

func (value *ParameterRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *ParameterRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *ParameterRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value ParameterRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type ResponseRef struct {
	Ref   string
	Value *Response
}

var _ jsonpointer.JSONPointable = (*ResponseRef)(nil)

func (value *ResponseRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *ResponseRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *ResponseRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value ResponseRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type RequestBodyRef struct {
	Ref   string
	Value *RequestBody
}

var _ jsonpointer.JSONPointable = (*RequestBodyRef)(nil)

func (value *RequestBodyRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *RequestBodyRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *RequestBodyRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value RequestBodyRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type SchemaRef struct {
	Ref   string
	Value *Schema
}

var _ jsonpointer.JSONPointable = (*SchemaRef)(nil)

func NewSchemaRef(ref string, value *Schema) *SchemaRef {
	return &SchemaRef{
		Ref:   ref,
		Value: value,
	}
}

func (value *SchemaRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *SchemaRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *SchemaRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value SchemaRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}

type SecuritySchemeRef struct {
	Ref   string
	Value *SecurityScheme
}

var _ jsonpointer.JSONPointable = (*SecuritySchemeRef)(nil)

func (value *SecuritySchemeRef) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalRef(value.Ref, value.Value)
}

func (value *SecuritySchemeRef) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalRef(data, &value.Ref, &value.Value)
}

func (value *SecuritySchemeRef) Validate(ctx context.Context) error {
	if v := value.Value; v != nil {
		return v.Validate(ctx)
	}
	return foundUnresolvedRef(value.Ref)
}

func (value SecuritySchemeRef) JSONLookup(token string) (interface{}, error) {
	if token == "$ref" {
		return value.Ref, nil
	}

	ptr, _, err := jsonpointer.GetForToken(value.Value, token)
	return ptr, err
}
