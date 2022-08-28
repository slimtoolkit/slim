package openapi3

import (
	"context"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/jsoninfo"
)

type PathItem struct {
	ExtensionProps
	Ref         string     `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Summary     string     `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Connect     *Operation `json:"connect,omitempty" yaml:"connect,omitempty"`
	Delete      *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Get         *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Head        *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Options     *Operation `json:"options,omitempty" yaml:"options,omitempty"`
	Patch       *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Post        *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put         *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Trace       *Operation `json:"trace,omitempty" yaml:"trace,omitempty"`
	Servers     Servers    `json:"servers,omitempty" yaml:"servers,omitempty"`
	Parameters  Parameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

func (pathItem *PathItem) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(pathItem)
}

func (pathItem *PathItem) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, pathItem)
}

func (pathItem *PathItem) Operations() map[string]*Operation {
	operations := make(map[string]*Operation, 4)
	if v := pathItem.Connect; v != nil {
		operations[http.MethodConnect] = v
	}
	if v := pathItem.Delete; v != nil {
		operations[http.MethodDelete] = v
	}
	if v := pathItem.Get; v != nil {
		operations[http.MethodGet] = v
	}
	if v := pathItem.Head; v != nil {
		operations[http.MethodHead] = v
	}
	if v := pathItem.Options; v != nil {
		operations[http.MethodOptions] = v
	}
	if v := pathItem.Patch; v != nil {
		operations[http.MethodPatch] = v
	}
	if v := pathItem.Post; v != nil {
		operations[http.MethodPost] = v
	}
	if v := pathItem.Put; v != nil {
		operations[http.MethodPut] = v
	}
	if v := pathItem.Trace; v != nil {
		operations[http.MethodTrace] = v
	}
	return operations
}

func (pathItem *PathItem) GetOperation(method string) *Operation {
	switch method {
	case http.MethodConnect:
		return pathItem.Connect
	case http.MethodDelete:
		return pathItem.Delete
	case http.MethodGet:
		return pathItem.Get
	case http.MethodHead:
		return pathItem.Head
	case http.MethodOptions:
		return pathItem.Options
	case http.MethodPatch:
		return pathItem.Patch
	case http.MethodPost:
		return pathItem.Post
	case http.MethodPut:
		return pathItem.Put
	case http.MethodTrace:
		return pathItem.Trace
	default:
		panic(fmt.Errorf("unsupported HTTP method %q", method))
	}
}

func (pathItem *PathItem) SetOperation(method string, operation *Operation) {
	switch method {
	case http.MethodConnect:
		pathItem.Connect = operation
	case http.MethodDelete:
		pathItem.Delete = operation
	case http.MethodGet:
		pathItem.Get = operation
	case http.MethodHead:
		pathItem.Head = operation
	case http.MethodOptions:
		pathItem.Options = operation
	case http.MethodPatch:
		pathItem.Patch = operation
	case http.MethodPost:
		pathItem.Post = operation
	case http.MethodPut:
		pathItem.Put = operation
	case http.MethodTrace:
		pathItem.Trace = operation
	default:
		panic(fmt.Errorf("unsupported HTTP method %q", method))
	}
}

func (value *PathItem) Validate(ctx context.Context) error {
	for _, operation := range value.Operations() {
		if err := operation.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}
