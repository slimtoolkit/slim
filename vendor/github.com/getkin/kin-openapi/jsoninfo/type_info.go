package jsoninfo

import (
	"reflect"
	"sort"
	"sync"
)

var (
	typeInfos      = map[reflect.Type]*TypeInfo{}
	typeInfosMutex sync.RWMutex
)

// TypeInfo contains information about JSON serialization of a type
type TypeInfo struct {
	Type   reflect.Type
	Fields []FieldInfo
}

func GetTypeInfoForValue(value interface{}) *TypeInfo {
	return GetTypeInfo(reflect.TypeOf(value))
}

// GetTypeInfo returns TypeInfo for the given type.
func GetTypeInfo(t reflect.Type) *TypeInfo {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	typeInfosMutex.RLock()
	typeInfo, exists := typeInfos[t]
	typeInfosMutex.RUnlock()
	if exists {
		return typeInfo
	}
	if t.Kind() != reflect.Struct {
		typeInfo = &TypeInfo{
			Type: t,
		}
	} else {
		// Allocate
		typeInfo = &TypeInfo{
			Type:   t,
			Fields: make([]FieldInfo, 0, 16),
		}

		// Add fields
		typeInfo.Fields = AppendFields(nil, nil, t)

		// Sort fields
		sort.Sort(sortableFieldInfos(typeInfo.Fields))
	}

	// Publish
	typeInfosMutex.Lock()
	typeInfos[t] = typeInfo
	typeInfosMutex.Unlock()
	return typeInfo
}

// FieldNames returns all field names
func (typeInfo *TypeInfo) FieldNames() []string {
	fields := typeInfo.Fields
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.JSONName)
	}
	return names
}
