package openapi3

import (
	"github.com/getkin/kin-openapi/jsoninfo"
)

// ExtensionProps provides support for OpenAPI extensions.
// It reads/writes all properties that begin with "x-".
type ExtensionProps struct {
	Extensions map[string]interface{} `json:"-" yaml:"-"`
}

// Assert that the type implements the interface
var _ jsoninfo.StrictStruct = &ExtensionProps{}

// EncodeWith will be invoked by package "jsoninfo"
func (props *ExtensionProps) EncodeWith(encoder *jsoninfo.ObjectEncoder, value interface{}) error {
	for k, v := range props.Extensions {
		if err := encoder.EncodeExtension(k, v); err != nil {
			return err
		}
	}
	return encoder.EncodeStructFieldsAndExtensions(value)
}

// DecodeWith will be invoked by package "jsoninfo"
func (props *ExtensionProps) DecodeWith(decoder *jsoninfo.ObjectDecoder, value interface{}) error {
	if err := decoder.DecodeStructFieldsAndExtensions(value); err != nil {
		return err
	}
	source := decoder.DecodeExtensionMap()
	result := make(map[string]interface{}, len(source))
	for k, v := range source {
		result[k] = v
	}
	props.Extensions = result
	return nil
}
