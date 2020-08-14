package openapi3

import (
	"github.com/getkin/kin-openapi/jsoninfo"
)

// ExternalDocs is specified by OpenAPI/Swagger standard version 3.0.
type ExternalDocs struct {
	ExtensionProps

	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

func (e *ExternalDocs) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(e)
}

func (e *ExternalDocs) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, e)
}
