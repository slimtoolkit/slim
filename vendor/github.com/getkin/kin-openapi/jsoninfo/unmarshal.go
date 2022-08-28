package jsoninfo

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// UnmarshalStrictStruct function:
//   * Unmarshals struct fields, ignoring UnmarshalJSON(...) and fields without 'json' tag.
//   * Correctly handles StrictStruct
func UnmarshalStrictStruct(data []byte, value StrictStruct) error {
	decoder, err := NewObjectDecoder(data)
	if err != nil {
		return err
	}
	return value.DecodeWith(decoder, value)
}

type ObjectDecoder struct {
	Data            []byte
	remainingFields map[string]json.RawMessage
}

func NewObjectDecoder(data []byte) (*ObjectDecoder, error) {
	var remainingFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &remainingFields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extension properties: %v (%s)", err, data)
	}
	return &ObjectDecoder{
		Data:            data,
		remainingFields: remainingFields,
	}, nil
}

// DecodeExtensionMap returns all properties that were not decoded previously.
func (decoder *ObjectDecoder) DecodeExtensionMap() map[string]json.RawMessage {
	return decoder.remainingFields
}

func (decoder *ObjectDecoder) DecodeStructFieldsAndExtensions(value interface{}) error {
	reflection := reflect.ValueOf(value)
	if reflection.Kind() != reflect.Ptr {
		panic(fmt.Errorf("value %T is not a pointer", value))
	}
	if reflection.IsNil() {
		panic(fmt.Errorf("value %T is nil", value))
	}
	reflection = reflection.Elem()
	for (reflection.Kind() == reflect.Interface || reflection.Kind() == reflect.Ptr) && !reflection.IsNil() {
		reflection = reflection.Elem()
	}
	reflectionType := reflection.Type()
	if reflectionType.Kind() != reflect.Struct {
		panic(fmt.Errorf("value %T is not a struct", value))
	}
	typeInfo := GetTypeInfo(reflectionType)

	// Supported fields
	fields := typeInfo.Fields
	remainingFields := decoder.remainingFields
	for fieldIndex, field := range fields {
		// Fields without JSON tag are ignored
		if !field.HasJSONTag {
			continue
		}

		// Get data
		fieldData, exists := remainingFields[field.JSONName]
		if !exists {
			continue
		}

		// Unmarshal
		if field.TypeIsUnmarshaller {
			fieldType := field.Type
			isPtr := false
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
				isPtr = true
			}
			fieldValue := reflect.New(fieldType)
			if err := fieldValue.Interface().(json.Unmarshaler).UnmarshalJSON(fieldData); err != nil {
				if field.MultipleFields {
					i := fieldIndex + 1
					if i < len(fields) && fields[i].JSONName == field.JSONName {
						continue
					}
				}
				return fmt.Errorf("failed to unmarshal property %q (%s): %v",
					field.JSONName, fieldValue.Type().String(), err)
			}
			if !isPtr {
				fieldValue = fieldValue.Elem()
			}
			reflection.FieldByIndex(field.Index).Set(fieldValue)

			// Remove the field from remaining fields
			delete(remainingFields, field.JSONName)
		} else {
			fieldPtr := reflection.FieldByIndex(field.Index)
			if fieldPtr.Kind() != reflect.Ptr || fieldPtr.IsNil() {
				fieldPtr = fieldPtr.Addr()
			}
			if err := json.Unmarshal(fieldData, fieldPtr.Interface()); err != nil {
				if field.MultipleFields {
					i := fieldIndex + 1
					if i < len(fields) && fields[i].JSONName == field.JSONName {
						continue
					}
				}
				return fmt.Errorf("failed to unmarshal property %q (%s): %v",
					field.JSONName, fieldPtr.Type().String(), err)
			}

			// Remove the field from remaining fields
			delete(remainingFields, field.JSONName)
		}
	}
	return nil
}
