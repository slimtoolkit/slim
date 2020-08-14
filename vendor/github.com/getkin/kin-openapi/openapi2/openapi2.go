// Package openapi2 parses and writes OpenAPI 2 specifications.
//
// Does not cover all elements of OpenAPI 2.
// When OpenAPI version 3 is backwards-compatible with version 2, version 3 elements have been used.
//
// The specification:
// https://github.com/OAI/OpenAPI-Specification/blob/master/versions/2.0.md
package openapi2

import (
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/getkin/kin-openapi/openapi3"
)

type Swagger struct {
	openapi3.ExtensionProps
	Swagger             string                         `json:"swagger"`
	Info                openapi3.Info                  `json:"info"`
	ExternalDocs        *openapi3.ExternalDocs         `json:"externalDocs,omitempty"`
	Schemes             []string                       `json:"schemes,omitempty"`
	Host                string                         `json:"host,omitempty"`
	BasePath            string                         `json:"basePath,omitempty"`
	Paths               map[string]*PathItem           `json:"paths,omitempty"`
	Definitions         map[string]*openapi3.SchemaRef `json:"definitions,omitempty,noref"`
	Parameters          map[string]*Parameter          `json:"parameters,omitempty,noref"`
	Responses           map[string]*Response           `json:"responses,omitempty,noref"`
	SecurityDefinitions map[string]*SecurityScheme     `json:"securityDefinitions,omitempty"`
	Security            SecurityRequirements           `json:"security,omitempty"`
	Tags                openapi3.Tags                  `json:"tags,omitempty"`
}

func (swagger *Swagger) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(swagger)
}

func (swagger *Swagger) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, swagger)
}

func (swagger *Swagger) AddOperation(path string, method string, operation *Operation) {
	paths := swagger.Paths
	if paths == nil {
		paths = make(map[string]*PathItem, 8)
		swagger.Paths = paths
	}
	pathItem := paths[path]
	if pathItem == nil {
		pathItem = &PathItem{}
		paths[path] = pathItem
	}
	pathItem.SetOperation(method, operation)
}

type PathItem struct {
	openapi3.ExtensionProps
	Ref        string     `json:"$ref,omitempty"`
	Delete     *Operation `json:"delete,omitempty"`
	Get        *Operation `json:"get,omitempty"`
	Head       *Operation `json:"head,omitempty"`
	Options    *Operation `json:"options,omitempty"`
	Patch      *Operation `json:"patch,omitempty"`
	Post       *Operation `json:"post,omitempty"`
	Put        *Operation `json:"put,omitempty"`
	Parameters Parameters `json:"parameters,omitempty"`
}

func (pathItem *PathItem) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(pathItem)
}

func (pathItem *PathItem) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, pathItem)
}

func (pathItem *PathItem) Operations() map[string]*Operation {
	operations := make(map[string]*Operation, 8)
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
	return operations
}

func (pathItem *PathItem) GetOperation(method string) *Operation {
	switch method {
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
	default:
		panic(fmt.Errorf("Unsupported HTTP method '%s'", method))
	}
}

func (pathItem *PathItem) SetOperation(method string, operation *Operation) {
	switch method {
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
	default:
		panic(fmt.Errorf("Unsupported HTTP method '%s'", method))
	}
}

type Operation struct {
	openapi3.ExtensionProps
	Summary      string                 `json:"summary,omitempty"`
	Description  string                 `json:"description,omitempty"`
	ExternalDocs *openapi3.ExternalDocs `json:"externalDocs,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	OperationID  string                 `json:"operationId,omitempty"`
	Parameters   Parameters             `json:"parameters,omitempty"`
	Responses    map[string]*Response   `json:"responses"`
	Consumes     []string               `json:"consumes,omitempty"`
	Produces     []string               `json:"produces,omitempty"`
	Security     *SecurityRequirements  `json:"security,omitempty"`
}

func (operation *Operation) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(operation)
}

func (operation *Operation) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, operation)
}

type Parameters []*Parameter

type Parameter struct {
	openapi3.ExtensionProps
	Ref              string              `json:"$ref,omitempty"`
	In               string              `json:"in,omitempty"`
	Name             string              `json:"name,omitempty"`
	Description      string              `json:"description,omitempty"`
	CollectionFormat string              `json:"collectionFormat,omitempty"`
	Type             string              `json:"type,omitempty"`
	Format           string              `json:"format,omitempty"`
	Pattern          string              `json:"pattern,omitempty"`
	AllowEmptyValue  bool                `json:"allowEmptyValue,omitempty"`
	Required         bool                `json:"required,omitempty"`
	UniqueItems      bool                `json:"uniqueItems,omitempty"`
	ExclusiveMin     bool                `json:"exclusiveMinimum,omitempty"`
	ExclusiveMax     bool                `json:"exclusiveMaximum,omitempty"`
	Schema           *openapi3.SchemaRef `json:"schema,omitempty"`
	Items            *openapi3.SchemaRef `json:"items,omitempty"`
	Enum             []interface{}       `json:"enum,omitempty"`
	MultipleOf       *float64            `json:"multipleOf,omitempty"`
	Minimum          *float64            `json:"minimum,omitempty"`
	Maximum          *float64            `json:"maximum,omitempty"`
	MaxLength        *uint64             `json:"maxLength,omitempty"`
	MaxItems         *uint64             `json:"maxItems,omitempty"`
	MinLength        uint64              `json:"minLength,omitempty"`
	MinItems         uint64              `json:"minItems,omitempty"`
	Default          interface{}         `json:"default,omitempty"`
}

func (parameter *Parameter) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(parameter)
}

func (parameter *Parameter) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, parameter)
}

type Response struct {
	openapi3.ExtensionProps
	Ref         string                 `json:"$ref,omitempty"`
	Description string                 `json:"description,omitempty"`
	Schema      *openapi3.SchemaRef    `json:"schema,omitempty"`
	Headers     map[string]*Header     `json:"headers,omitempty"`
	Examples    map[string]interface{} `json:"examples,omitempty"`
}

func (response *Response) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(response)
}

func (response *Response) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, response)
}

type Header struct {
	Ref         string `json:"$ref,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

type SecurityRequirements []map[string][]string

type SecurityScheme struct {
	openapi3.ExtensionProps
	Ref              string            `json:"$ref,omitempty"`
	Description      string            `json:"description,omitempty"`
	Type             string            `json:"type,omitempty"`
	In               string            `json:"in,omitempty"`
	Name             string            `json:"name,omitempty"`
	Flow             string            `json:"flow,omitempty"`
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	Scopes           map[string]string `json:"scopes,omitempty"`
	Tags             openapi3.Tags     `json:"tags,omitempty"`
}

func (securityScheme *SecurityScheme) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(securityScheme)
}

func (securityScheme *SecurityScheme) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, securityScheme)
}
