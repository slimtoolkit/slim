package openapi3

import (
	"context"
	"strings"
)

// Content is specified by OpenAPI/Swagger 3.0 standard.
type Content map[string]*MediaType

func NewContent() Content {
	return make(map[string]*MediaType, 4)
}

func NewContentWithSchema(schema *Schema, consumes []string) Content {
	if len(consumes) == 0 {
		return Content{
			"*/*": NewMediaType().WithSchema(schema),
		}
	}
	content := make(map[string]*MediaType, len(consumes))
	for _, mediaType := range consumes {
		content[mediaType] = NewMediaType().WithSchema(schema)
	}
	return content
}

func NewContentWithSchemaRef(schema *SchemaRef, consumes []string) Content {
	if len(consumes) == 0 {
		return Content{
			"*/*": NewMediaType().WithSchemaRef(schema),
		}
	}
	content := make(map[string]*MediaType, len(consumes))
	for _, mediaType := range consumes {
		content[mediaType] = NewMediaType().WithSchemaRef(schema)
	}
	return content
}

func NewContentWithJSONSchema(schema *Schema) Content {
	return Content{
		"application/json": NewMediaType().WithSchema(schema),
	}
}
func NewContentWithJSONSchemaRef(schema *SchemaRef) Content {
	return Content{
		"application/json": NewMediaType().WithSchemaRef(schema),
	}
}

func NewContentWithFormDataSchema(schema *Schema) Content {
	return Content{
		"multipart/form-data": NewMediaType().WithSchema(schema),
	}
}

func NewContentWithFormDataSchemaRef(schema *SchemaRef) Content {
	return Content{
		"multipart/form-data": NewMediaType().WithSchemaRef(schema),
	}
}

func (content Content) Get(mime string) *MediaType {
	// If the mime is empty then short-circuit to the wildcard.
	// We do this here so that we catch only the specific case of
	// and empty mime rather than a present, but invalid, mime type.
	if mime == "" {
		return content["*/*"]
	}
	// Start by making the most specific match possible
	// by using the mime type in full.
	if v := content[mime]; v != nil {
		return v
	}
	// If an exact match is not found then we strip all
	// metadata from the mime type and only use the x/y
	// portion.
	i := strings.IndexByte(mime, ';')
	if i < 0 {
		// If there is no metadata then preserve the full mime type
		// string for later wildcard searches.
		i = len(mime)
	}
	mime = mime[:i]
	if v := content[mime]; v != nil {
		return v
	}
	// If the x/y pattern has no specific match then we
	// try the x/* pattern.
	i = strings.IndexByte(mime, '/')
	if i < 0 {
		// In the case that the given mime type is not valid because it is
		// missing the subtype we return nil so that this does not accidentally
		// resolve with the wildcard.
		return nil
	}
	mime = mime[:i] + "/*"
	if v := content[mime]; v != nil {
		return v
	}
	// Finally, the most generic match of */* is returned
	// as a catch-all.
	return content["*/*"]
}

func (value Content) Validate(ctx context.Context) error {
	for _, v := range value {
		// Validate MediaType
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}
