package openapi3

import (
	"context"
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

type Links map[string]*LinkRef

func (l Links) JSONLookup(token string) (interface{}, error) {
	ref, ok := l[token]
	if ok == false {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref != nil && ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

var _ jsonpointer.JSONPointable = (*Links)(nil)

// Link is specified by OpenAPI/Swagger standard version 3.0.
type Link struct {
	ExtensionProps
	OperationID  string                 `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	OperationRef string                 `json:"operationRef,omitempty" yaml:"operationRef,omitempty"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Server       *Server                `json:"server,omitempty" yaml:"server,omitempty"`
	RequestBody  interface{}            `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
}

func (value *Link) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(value)
}

func (value *Link) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, value)
}

func (value *Link) Validate(ctx context.Context) error {
	if value.OperationID == "" && value.OperationRef == "" {
		return errors.New("missing operationId or operationRef on link")
	}
	if value.OperationID != "" && value.OperationRef != "" {
		return fmt.Errorf("operationId %q and operationRef %q are mutually exclusive", value.OperationID, value.OperationRef)
	}
	return nil
}
