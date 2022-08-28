package openapi2

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/getkin/kin-openapi/openapi3"
)

// T is the root of an OpenAPI v2 document
type T struct {
	openapi3.ExtensionProps
	Swagger             string                         `json:"swagger" yaml:"swagger"`
	Info                openapi3.Info                  `json:"info" yaml:"info"`
	ExternalDocs        *openapi3.ExternalDocs         `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	Schemes             []string                       `json:"schemes,omitempty" yaml:"schemes,omitempty"`
	Consumes            []string                       `json:"consumes,omitempty" yaml:"consumes,omitempty"`
	Host                string                         `json:"host,omitempty" yaml:"host,omitempty"`
	BasePath            string                         `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	Paths               map[string]*PathItem           `json:"paths,omitempty" yaml:"paths,omitempty"`
	Definitions         map[string]*openapi3.SchemaRef `json:"definitions,omitempty,noref" yaml:"definitions,omitempty,noref"`
	Parameters          map[string]*Parameter          `json:"parameters,omitempty,noref" yaml:"parameters,omitempty,noref"`
	Responses           map[string]*Response           `json:"responses,omitempty,noref" yaml:"responses,omitempty,noref"`
	SecurityDefinitions map[string]*SecurityScheme     `json:"securityDefinitions,omitempty" yaml:"securityDefinitions,omitempty"`
	Security            SecurityRequirements           `json:"security,omitempty" yaml:"security,omitempty"`
	Tags                openapi3.Tags                  `json:"tags,omitempty" yaml:"tags,omitempty"`
}

func (doc *T) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(doc)
}

func (doc *T) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, doc)
}

func (doc *T) AddOperation(path string, method string, operation *Operation) {
	paths := doc.Paths
	if paths == nil {
		paths = make(map[string]*PathItem, 8)
		doc.Paths = paths
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
	Ref        string     `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Delete     *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Get        *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Head       *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Options    *Operation `json:"options,omitempty" yaml:"options,omitempty"`
	Patch      *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Post       *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put        *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Parameters Parameters `json:"parameters,omitempty" yaml:"parameters,omitempty"`
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
		panic(fmt.Errorf("unsupported HTTP method %q", method))
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
		panic(fmt.Errorf("unsupported HTTP method %q", method))
	}
}

type Operation struct {
	openapi3.ExtensionProps
	Summary      string                 `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalDocs *openapi3.ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	Tags         []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	OperationID  string                 `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Parameters   Parameters             `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Responses    map[string]*Response   `json:"responses" yaml:"responses"`
	Consumes     []string               `json:"consumes,omitempty" yaml:"consumes,omitempty"`
	Produces     []string               `json:"produces,omitempty" yaml:"produces,omitempty"`
	Security     *SecurityRequirements  `json:"security,omitempty" yaml:"security,omitempty"`
}

func (operation *Operation) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(operation)
}

func (operation *Operation) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, operation)
}

type Parameters []*Parameter

var _ sort.Interface = Parameters{}

func (ps Parameters) Len() int      { return len(ps) }
func (ps Parameters) Swap(i, j int) { ps[i], ps[j] = ps[j], ps[i] }
func (ps Parameters) Less(i, j int) bool {
	if ps[i].Name != ps[j].Name {
		return ps[i].Name < ps[j].Name
	}
	if ps[i].In != ps[j].In {
		return ps[i].In < ps[j].In
	}
	return ps[i].Ref < ps[j].Ref
}

type Parameter struct {
	openapi3.ExtensionProps
	Ref              string              `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	In               string              `json:"in,omitempty" yaml:"in,omitempty"`
	Name             string              `json:"name,omitempty" yaml:"name,omitempty"`
	Description      string              `json:"description,omitempty" yaml:"description,omitempty"`
	CollectionFormat string              `json:"collectionFormat,omitempty" yaml:"collectionFormat,omitempty"`
	Type             string              `json:"type,omitempty" yaml:"type,omitempty"`
	Format           string              `json:"format,omitempty" yaml:"format,omitempty"`
	Pattern          string              `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	AllowEmptyValue  bool                `json:"allowEmptyValue,omitempty" yaml:"allowEmptyValue,omitempty"`
	Required         bool                `json:"required,omitempty" yaml:"required,omitempty"`
	UniqueItems      bool                `json:"uniqueItems,omitempty" yaml:"uniqueItems,omitempty"`
	ExclusiveMin     bool                `json:"exclusiveMinimum,omitempty" yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMax     bool                `json:"exclusiveMaximum,omitempty" yaml:"exclusiveMaximum,omitempty"`
	Schema           *openapi3.SchemaRef `json:"schema,omitempty" yaml:"schema,omitempty"`
	Items            *openapi3.SchemaRef `json:"items,omitempty" yaml:"items,omitempty"`
	Enum             []interface{}       `json:"enum,omitempty" yaml:"enum,omitempty"`
	MultipleOf       *float64            `json:"multipleOf,omitempty" yaml:"multipleOf,omitempty"`
	Minimum          *float64            `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum          *float64            `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MaxLength        *uint64             `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	MaxItems         *uint64             `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	MinLength        uint64              `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MinItems         uint64              `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	Default          interface{}         `json:"default,omitempty" yaml:"default,omitempty"`
}

func (parameter *Parameter) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(parameter)
}

func (parameter *Parameter) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, parameter)
}

type Response struct {
	openapi3.ExtensionProps
	Ref         string                 `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Schema      *openapi3.SchemaRef    `json:"schema,omitempty" yaml:"schema,omitempty"`
	Headers     map[string]*Header     `json:"headers,omitempty" yaml:"headers,omitempty"`
	Examples    map[string]interface{} `json:"examples,omitempty" yaml:"examples,omitempty"`
}

func (response *Response) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(response)
}

func (response *Response) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, response)
}

type Header struct {
	openapi3.ExtensionProps
	Ref         string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
}

func (header *Header) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(header)
}

func (header *Header) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, header)
}

type SecurityRequirements []map[string][]string

type SecurityScheme struct {
	openapi3.ExtensionProps
	Ref              string            `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
	Type             string            `json:"type,omitempty" yaml:"type,omitempty"`
	In               string            `json:"in,omitempty" yaml:"in,omitempty"`
	Name             string            `json:"name,omitempty" yaml:"name,omitempty"`
	Flow             string            `json:"flow,omitempty" yaml:"flow,omitempty"`
	AuthorizationURL string            `json:"authorizationUrl,omitempty" yaml:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	Scopes           map[string]string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Tags             openapi3.Tags     `json:"tags,omitempty" yaml:"tags,omitempty"`
}

func (securityScheme *SecurityScheme) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(securityScheme)
}

func (securityScheme *SecurityScheme) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, securityScheme)
}
