package openapi3

import (
	"context"
	"errors"
	"strconv"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

// Operation represents "operation" specified by" OpenAPI/Swagger 3.0 standard.
type Operation struct {
	ExtensionProps

	// Optional tags for documentation.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// Optional short summary.
	Summary string `json:"summary,omitempty" yaml:"summary,omitempty"`

	// Optional description. Should use CommonMark syntax.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Optional operation ID.
	OperationID string `json:"operationId,omitempty" yaml:"operationId,omitempty"`

	// Optional parameters.
	Parameters Parameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// Optional body parameter.
	RequestBody *RequestBodyRef `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`

	// Responses.
	Responses Responses `json:"responses" yaml:"responses"` // Required

	// Optional callbacks
	Callbacks Callbacks `json:"callbacks,omitempty" yaml:"callbacks,omitempty"`

	Deprecated bool `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`

	// Optional security requirements that overrides top-level security.
	Security *SecurityRequirements `json:"security,omitempty" yaml:"security,omitempty"`

	// Optional servers that overrides top-level servers.
	Servers *Servers `json:"servers,omitempty" yaml:"servers,omitempty"`

	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
}

var _ jsonpointer.JSONPointable = (*Operation)(nil)

func NewOperation() *Operation {
	return &Operation{}
}

func (operation *Operation) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(operation)
}

func (operation *Operation) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, operation)
}

func (operation Operation) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "requestBody":
		if operation.RequestBody != nil {
			if operation.RequestBody.Ref != "" {
				return &Ref{Ref: operation.RequestBody.Ref}, nil
			}
			return operation.RequestBody.Value, nil
		}
	case "tags":
		return operation.Tags, nil
	case "summary":
		return operation.Summary, nil
	case "description":
		return operation.Description, nil
	case "operationID":
		return operation.OperationID, nil
	case "parameters":
		return operation.Parameters, nil
	case "responses":
		return operation.Responses, nil
	case "callbacks":
		return operation.Callbacks, nil
	case "deprecated":
		return operation.Deprecated, nil
	case "security":
		return operation.Security, nil
	case "servers":
		return operation.Servers, nil
	case "externalDocs":
		return operation.ExternalDocs, nil
	}

	v, _, err := jsonpointer.GetForToken(operation.ExtensionProps, token)
	return v, err
}

func (operation *Operation) AddParameter(p *Parameter) {
	operation.Parameters = append(operation.Parameters, &ParameterRef{
		Value: p,
	})
}

func (operation *Operation) AddResponse(status int, response *Response) {
	responses := operation.Responses
	if responses == nil {
		responses = NewResponses()
		operation.Responses = responses
	}
	code := "default"
	if status != 0 {
		code = strconv.FormatInt(int64(status), 10)
	}
	responses[code] = &ResponseRef{
		Value: response,
	}
}

func (value *Operation) Validate(ctx context.Context) error {
	if v := value.Parameters; v != nil {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	if v := value.RequestBody; v != nil {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	if v := value.Responses; v != nil {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	} else {
		return errors.New("value of responses must be an object")
	}
	return nil
}
