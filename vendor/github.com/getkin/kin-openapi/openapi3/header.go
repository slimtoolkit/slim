package openapi3

import (
	"context"
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

type Headers map[string]*HeaderRef

var _ jsonpointer.JSONPointable = (*Headers)(nil)

func (h Headers) JSONLookup(token string) (interface{}, error) {
	ref, ok := h[token]
	if ref == nil || !ok {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

// Header is specified by OpenAPI/Swagger 3.0 standard.
// See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md#headerObject
type Header struct {
	Parameter
}

var _ jsonpointer.JSONPointable = (*Header)(nil)

func (value *Header) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, value)
}

// SerializationMethod returns a header's serialization method.
func (value *Header) SerializationMethod() (*SerializationMethod, error) {
	style := value.Style
	if style == "" {
		style = SerializationSimple
	}
	explode := false
	if value.Explode != nil {
		explode = *value.Explode
	}
	return &SerializationMethod{Style: style, Explode: explode}, nil
}

func (value *Header) Validate(ctx context.Context) error {
	if value.Name != "" {
		return errors.New("header 'name' MUST NOT be specified, it is given in the corresponding headers map")
	}
	if value.In != "" {
		return errors.New("header 'in' MUST NOT be specified, it is implicitly in header")
	}

	// Validate a parameter's serialization method.
	sm, err := value.SerializationMethod()
	if err != nil {
		return err
	}
	if smSupported := false ||
		sm.Style == SerializationSimple && !sm.Explode ||
		sm.Style == SerializationSimple && sm.Explode; !smSupported {
		e := fmt.Errorf("serialization method with style=%q and explode=%v is not supported by a header parameter", sm.Style, sm.Explode)
		return fmt.Errorf("header schema is invalid: %v", e)
	}

	if (value.Schema == nil) == (value.Content == nil) {
		e := fmt.Errorf("parameter must contain exactly one of content and schema: %v", value)
		return fmt.Errorf("header schema is invalid: %v", e)
	}
	if schema := value.Schema; schema != nil {
		if err := schema.Validate(ctx); err != nil {
			return fmt.Errorf("header schema is invalid: %v", err)
		}
	}

	if content := value.Content; content != nil {
		if err := content.Validate(ctx); err != nil {
			return fmt.Errorf("header content is invalid: %v", err)
		}
	}
	return nil
}

func (value Header) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "schema":
		if value.Schema != nil {
			if value.Schema.Ref != "" {
				return &Ref{Ref: value.Schema.Ref}, nil
			}
			return value.Schema.Value, nil
		}
	case "name":
		return value.Name, nil
	case "in":
		return value.In, nil
	case "description":
		return value.Description, nil
	case "style":
		return value.Style, nil
	case "explode":
		return value.Explode, nil
	case "allowEmptyValue":
		return value.AllowEmptyValue, nil
	case "allowReserved":
		return value.AllowReserved, nil
	case "deprecated":
		return value.Deprecated, nil
	case "required":
		return value.Required, nil
	case "example":
		return value.Example, nil
	case "examples":
		return value.Examples, nil
	case "content":
		return value.Content, nil
	}

	v, _, err := jsonpointer.GetForToken(value.ExtensionProps, token)
	return v, err
}
