package openapi3

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
)

// Encoding is specified by OpenAPI/Swagger 3.0 standard.
type Encoding struct {
	ExtensionProps

	ContentType   string  `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Headers       Headers `json:"headers,omitempty" yaml:"headers,omitempty"`
	Style         string  `json:"style,omitempty" yaml:"style,omitempty"`
	Explode       *bool   `json:"explode,omitempty" yaml:"explode,omitempty"`
	AllowReserved bool    `json:"allowReserved,omitempty" yaml:"allowReserved,omitempty"`
}

func NewEncoding() *Encoding {
	return &Encoding{}
}

func (encoding *Encoding) WithHeader(name string, header *Header) *Encoding {
	return encoding.WithHeaderRef(name, &HeaderRef{
		Value: header,
	})
}

func (encoding *Encoding) WithHeaderRef(name string, ref *HeaderRef) *Encoding {
	headers := encoding.Headers
	if headers == nil {
		headers = make(map[string]*HeaderRef)
		encoding.Headers = headers
	}
	headers[name] = ref
	return encoding
}

func (encoding *Encoding) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(encoding)
}

func (encoding *Encoding) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, encoding)
}

// SerializationMethod returns a serialization method of request body.
// When serialization method is not defined the method returns the default serialization method.
func (encoding *Encoding) SerializationMethod() *SerializationMethod {
	sm := &SerializationMethod{Style: SerializationForm, Explode: true}
	if encoding != nil {
		if encoding.Style != "" {
			sm.Style = encoding.Style
		}
		if encoding.Explode != nil {
			sm.Explode = *encoding.Explode
		}
	}
	return sm
}

func (value *Encoding) Validate(ctx context.Context) error {
	if value == nil {
		return nil
	}
	for k, v := range value.Headers {
		if err := ValidateIdentifier(k); err != nil {
			return nil
		}
		if err := v.Validate(ctx); err != nil {
			return nil
		}
	}

	// Validate a media types's serialization method.
	sm := value.SerializationMethod()
	switch {
	case sm.Style == SerializationForm && sm.Explode,
		sm.Style == SerializationForm && !sm.Explode,
		sm.Style == SerializationSpaceDelimited && sm.Explode,
		sm.Style == SerializationSpaceDelimited && !sm.Explode,
		sm.Style == SerializationPipeDelimited && sm.Explode,
		sm.Style == SerializationPipeDelimited && !sm.Explode,
		sm.Style == SerializationDeepObject && sm.Explode:
		// it is a valid
	default:
		return fmt.Errorf("serialization method with style=%q and explode=%v is not supported by media type", sm.Style, sm.Explode)
	}

	return nil
}
