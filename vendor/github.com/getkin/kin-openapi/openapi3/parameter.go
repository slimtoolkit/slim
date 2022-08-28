package openapi3

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

type ParametersMap map[string]*ParameterRef

var _ jsonpointer.JSONPointable = (*ParametersMap)(nil)

func (p ParametersMap) JSONLookup(token string) (interface{}, error) {
	ref, ok := p[token]
	if ref == nil || ok == false {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

// Parameters is specified by OpenAPI/Swagger 3.0 standard.
type Parameters []*ParameterRef

var _ jsonpointer.JSONPointable = (*Parameters)(nil)

func (p Parameters) JSONLookup(token string) (interface{}, error) {
	index, err := strconv.Atoi(token)
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(p) {
		return nil, fmt.Errorf("index %d out of bounds of array of length %d", index, len(p))
	}

	ref := p[index]

	if ref != nil && ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

func NewParameters() Parameters {
	return make(Parameters, 0, 4)
}

func (parameters Parameters) GetByInAndName(in string, name string) *Parameter {
	for _, item := range parameters {
		if v := item.Value; v != nil {
			if v.Name == name && v.In == in {
				return v
			}
		}
	}
	return nil
}

func (value Parameters) Validate(ctx context.Context) error {
	dupes := make(map[string]struct{})
	for _, item := range value {
		if v := item.Value; v != nil {
			key := v.In + ":" + v.Name
			if _, ok := dupes[key]; ok {
				return fmt.Errorf("more than one %q parameter has name %q", v.In, v.Name)
			}
			dupes[key] = struct{}{}
		}

		if err := item.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Parameter is specified by OpenAPI/Swagger 3.0 standard.
// See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md#parameterObject
type Parameter struct {
	ExtensionProps
	Name            string      `json:"name,omitempty" yaml:"name,omitempty"`
	In              string      `json:"in,omitempty" yaml:"in,omitempty"`
	Description     string      `json:"description,omitempty" yaml:"description,omitempty"`
	Style           string      `json:"style,omitempty" yaml:"style,omitempty"`
	Explode         *bool       `json:"explode,omitempty" yaml:"explode,omitempty"`
	AllowEmptyValue bool        `json:"allowEmptyValue,omitempty" yaml:"allowEmptyValue,omitempty"`
	AllowReserved   bool        `json:"allowReserved,omitempty" yaml:"allowReserved,omitempty"`
	Deprecated      bool        `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Required        bool        `json:"required,omitempty" yaml:"required,omitempty"`
	Schema          *SchemaRef  `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example         interface{} `json:"example,omitempty" yaml:"example,omitempty"`
	Examples        Examples    `json:"examples,omitempty" yaml:"examples,omitempty"`
	Content         Content     `json:"content,omitempty" yaml:"content,omitempty"`
}

var _ jsonpointer.JSONPointable = (*Parameter)(nil)

const (
	ParameterInPath   = "path"
	ParameterInQuery  = "query"
	ParameterInHeader = "header"
	ParameterInCookie = "cookie"
)

func NewPathParameter(name string) *Parameter {
	return &Parameter{
		Name:     name,
		In:       ParameterInPath,
		Required: true,
	}
}

func NewQueryParameter(name string) *Parameter {
	return &Parameter{
		Name: name,
		In:   ParameterInQuery,
	}
}

func NewHeaderParameter(name string) *Parameter {
	return &Parameter{
		Name: name,
		In:   ParameterInHeader,
	}
}

func NewCookieParameter(name string) *Parameter {
	return &Parameter{
		Name: name,
		In:   ParameterInCookie,
	}
}

func (parameter *Parameter) WithDescription(value string) *Parameter {
	parameter.Description = value
	return parameter
}

func (parameter *Parameter) WithRequired(value bool) *Parameter {
	parameter.Required = value
	return parameter
}

func (parameter *Parameter) WithSchema(value *Schema) *Parameter {
	if value == nil {
		parameter.Schema = nil
	} else {
		parameter.Schema = &SchemaRef{
			Value: value,
		}
	}
	return parameter
}

func (parameter *Parameter) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(parameter)
}

func (parameter *Parameter) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, parameter)
}

func (value Parameter) JSONLookup(token string) (interface{}, error) {
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

// SerializationMethod returns a parameter's serialization method.
// When a parameter's serialization method is not defined the method returns
// the default serialization method corresponding to a parameter's location.
func (parameter *Parameter) SerializationMethod() (*SerializationMethod, error) {
	switch parameter.In {
	case ParameterInPath, ParameterInHeader:
		style := parameter.Style
		if style == "" {
			style = SerializationSimple
		}
		explode := false
		if parameter.Explode != nil {
			explode = *parameter.Explode
		}
		return &SerializationMethod{Style: style, Explode: explode}, nil
	case ParameterInQuery, ParameterInCookie:
		style := parameter.Style
		if style == "" {
			style = SerializationForm
		}
		explode := true
		if parameter.Explode != nil {
			explode = *parameter.Explode
		}
		return &SerializationMethod{Style: style, Explode: explode}, nil
	default:
		return nil, fmt.Errorf("unexpected parameter's 'in': %q", parameter.In)
	}
}

func (value *Parameter) Validate(ctx context.Context) error {
	if value.Name == "" {
		return errors.New("parameter name can't be blank")
	}
	in := value.In
	switch in {
	case
		ParameterInPath,
		ParameterInQuery,
		ParameterInHeader,
		ParameterInCookie:
	default:
		return fmt.Errorf("parameter can't have 'in' value %q", value.In)
	}

	// Validate a parameter's serialization method.
	sm, err := value.SerializationMethod()
	if err != nil {
		return err
	}
	var smSupported bool
	switch {
	case value.In == ParameterInPath && sm.Style == SerializationSimple && !sm.Explode,
		value.In == ParameterInPath && sm.Style == SerializationSimple && sm.Explode,
		value.In == ParameterInPath && sm.Style == SerializationLabel && !sm.Explode,
		value.In == ParameterInPath && sm.Style == SerializationLabel && sm.Explode,
		value.In == ParameterInPath && sm.Style == SerializationMatrix && !sm.Explode,
		value.In == ParameterInPath && sm.Style == SerializationMatrix && sm.Explode,

		value.In == ParameterInQuery && sm.Style == SerializationForm && sm.Explode,
		value.In == ParameterInQuery && sm.Style == SerializationForm && !sm.Explode,
		value.In == ParameterInQuery && sm.Style == SerializationSpaceDelimited && sm.Explode,
		value.In == ParameterInQuery && sm.Style == SerializationSpaceDelimited && !sm.Explode,
		value.In == ParameterInQuery && sm.Style == SerializationPipeDelimited && sm.Explode,
		value.In == ParameterInQuery && sm.Style == SerializationPipeDelimited && !sm.Explode,
		value.In == ParameterInQuery && sm.Style == SerializationDeepObject && sm.Explode,

		value.In == ParameterInHeader && sm.Style == SerializationSimple && !sm.Explode,
		value.In == ParameterInHeader && sm.Style == SerializationSimple && sm.Explode,

		value.In == ParameterInCookie && sm.Style == SerializationForm && !sm.Explode,
		value.In == ParameterInCookie && sm.Style == SerializationForm && sm.Explode:
		smSupported = true
	}
	if !smSupported {
		e := fmt.Errorf("serialization method with style=%q and explode=%v is not supported by a %s parameter", sm.Style, sm.Explode, in)
		return fmt.Errorf("parameter %q schema is invalid: %v", value.Name, e)
	}

	if (value.Schema == nil) == (value.Content == nil) {
		e := errors.New("parameter must contain exactly one of content and schema")
		return fmt.Errorf("parameter %q schema is invalid: %v", value.Name, e)
	}
	if schema := value.Schema; schema != nil {
		if err := schema.Validate(ctx); err != nil {
			return fmt.Errorf("parameter %q schema is invalid: %v", value.Name, err)
		}
	}

	if content := value.Content; content != nil {
		if err := content.Validate(ctx); err != nil {
			return fmt.Errorf("parameter %q content is invalid: %v", value.Name, err)
		}
	}
	return nil
}
