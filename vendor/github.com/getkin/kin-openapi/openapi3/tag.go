package openapi3

import "github.com/getkin/kin-openapi/jsoninfo"

// Tags is specified by OpenAPI/Swagger 3.0 standard.
type Tags []*Tag

func (tags Tags) Get(name string) *Tag {
	for _, tag := range tags {
		if tag.Name == name {
			return tag
		}
	}
	return nil
}

// Tag is specified by OpenAPI/Swagger 3.0 standard.
type Tag struct {
	ExtensionProps
	Name         string        `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string        `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
}

func (t *Tag) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(t)
}

func (t *Tag) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, t)
}
