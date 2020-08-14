package jsoninfo

import (
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FieldInfo contains information about JSON serialization of a field.
type FieldInfo struct {
	MultipleFields     bool // Whether multiple Go fields share this JSON name
	HasJSONTag         bool
	TypeIsMarshaller   bool
	TypeIsUnmarshaller bool
	JSONOmitEmpty      bool
	JSONString         bool
	Index              []int
	Type               reflect.Type
	JSONName           string
}

func AppendFields(fields []FieldInfo, parentIndex []int, t reflect.Type) []FieldInfo {
	// For each field
	numField := t.NumField()
iteration:
	for i := 0; i < numField; i++ {
		f := t.Field(i)
		index := make([]int, 0, len(parentIndex)+1)
		index = append(index, parentIndex...)
		index = append(index, i)

		// See whether this is an embedded field
		if f.Anonymous {
			if f.Tag.Get("json") == "-" {
				continue
			}
			fields = AppendFields(fields, index, f.Type)
			continue iteration
		}

		// Ignore certain types
		switch f.Type.Kind() {
		case reflect.Func, reflect.Chan:
			continue iteration
		}

		// Is it a private (lowercase) field?
		firstRune, _ := utf8.DecodeRuneInString(f.Name)
		if unicode.IsLower(firstRune) {
			continue iteration
		}

		// Declare a field
		field := FieldInfo{
			Index:    index,
			Type:     f.Type,
			JSONName: f.Name,
		}

		// Read "json" tag
		jsonTag := f.Tag.Get("json")

		// Read our custom "multijson" tag that
		// allows multiple fields with the same name.
		if v := f.Tag.Get("multijson"); len(v) > 0 {
			field.MultipleFields = true
			jsonTag = v
		}

		// Handle "-"
		if jsonTag == "-" {
			continue
		}

		// Parse the tag
		if len(jsonTag) > 0 {
			field.HasJSONTag = true
			for i, part := range strings.Split(jsonTag, ",") {
				if i == 0 {
					if len(part) > 0 {
						field.JSONName = part
					}
				} else {
					switch part {
					case "omitempty":
						field.JSONOmitEmpty = true
					case "string":
						field.JSONString = true
					}
				}
			}
		}

		if _, ok := field.Type.MethodByName("MarshalJSON"); ok {
			field.TypeIsMarshaller = true
		}
		if _, ok := field.Type.MethodByName("UnmarshalJSON"); ok {
			field.TypeIsUnmarshaller = true
		}

		// Field is done
		fields = append(fields, field)
	}

	return fields
}

type sortableFieldInfos []FieldInfo

func (list sortableFieldInfos) Len() int {
	return len(list)
}

func (list sortableFieldInfos) Less(i, j int) bool {
	return list[i].JSONName < list[j].JSONName
}

func (list sortableFieldInfos) Swap(i, j int) {
	a, b := list[i], list[j]
	list[i], list[j] = b, a
}
