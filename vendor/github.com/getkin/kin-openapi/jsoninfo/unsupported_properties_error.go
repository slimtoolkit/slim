package jsoninfo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// UnsupportedPropertiesError is a helper for extensions that want to refuse
// unsupported JSON object properties.
//
// It produces a helpful error message.
type UnsupportedPropertiesError struct {
	Value                 interface{}
	UnsupportedProperties map[string]json.RawMessage
}

func NewUnsupportedPropertiesError(v interface{}, m map[string]json.RawMessage) error {
	return &UnsupportedPropertiesError{
		Value:                 v,
		UnsupportedProperties: m,
	}
}

func (err *UnsupportedPropertiesError) Error() string {
	m := err.UnsupportedProperties
	typeInfo := GetTypeInfoForValue(err.Value)
	if m == nil || typeInfo == nil {
		return "Invalid UnsupportedPropertiesError"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	supported := typeInfo.FieldNames()
	if len(supported) == 0 {
		return fmt.Sprintf("Type '%T' doesn't take any properties. Unsupported properties: '%s'\n",
			err.Value, strings.Join(keys, "', '"))
	}
	return fmt.Sprintf("Unsupported properties: '%s'\nSupported properties are: '%s'",
		strings.Join(keys, "', '"),
		strings.Join(supported, "', '"))
}
