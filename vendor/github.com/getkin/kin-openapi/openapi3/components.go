package openapi3

import (
	"context"
	"fmt"
	"regexp"

	"github.com/getkin/kin-openapi/jsoninfo"
)

// Components is specified by OpenAPI/Swagger standard version 3.0.
type Components struct {
	ExtensionProps
	Schemas         Schemas         `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	Parameters      ParametersMap   `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Headers         Headers         `json:"headers,omitempty" yaml:"headers,omitempty"`
	RequestBodies   RequestBodies   `json:"requestBodies,omitempty" yaml:"requestBodies,omitempty"`
	Responses       Responses       `json:"responses,omitempty" yaml:"responses,omitempty"`
	SecuritySchemes SecuritySchemes `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
	Examples        Examples        `json:"examples,omitempty" yaml:"examples,omitempty"`
	Links           Links           `json:"links,omitempty" yaml:"links,omitempty"`
	Callbacks       Callbacks       `json:"callbacks,omitempty" yaml:"callbacks,omitempty"`
}

func NewComponents() Components {
	return Components{}
}

func (components *Components) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(components)
}

func (components *Components) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, components)
}

func (components *Components) Validate(ctx context.Context) (err error) {
	for k, v := range components.Schemas {
		if err = ValidateIdentifier(k); err != nil {
			return
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}

	for k, v := range components.Parameters {
		if err = ValidateIdentifier(k); err != nil {
			return
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}

	for k, v := range components.RequestBodies {
		if err = ValidateIdentifier(k); err != nil {
			return
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}

	for k, v := range components.Responses {
		if err = ValidateIdentifier(k); err != nil {
			return
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}

	for k, v := range components.Headers {
		if err = ValidateIdentifier(k); err != nil {
			return
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}

	for k, v := range components.SecuritySchemes {
		if err = ValidateIdentifier(k); err != nil {
			return
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}

	return
}

const identifierPattern = `^[a-zA-Z0-9._-]+$`

// IdentifierRegExp verifies whether Component object key matches 'identifierPattern' pattern, according to OapiAPI v3.x.0.
// Hovever, to be able supporting legacy OpenAPI v2.x, there is a need to customize above pattern in orde not to fail
// converted v2-v3 validation
var IdentifierRegExp = regexp.MustCompile(identifierPattern)

func ValidateIdentifier(value string) error {
	if IdentifierRegExp.MatchString(value) {
		return nil
	}
	return fmt.Errorf("identifier %q is not supported by OpenAPIv3 standard (regexp: %q)", value, identifierPattern)
}
