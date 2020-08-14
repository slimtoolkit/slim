package openapi3

import "context"

// Callback is specified by OpenAPI/Swagger standard version 3.0.
type Callback map[string]*PathItem

func (value Callback) Validate(c context.Context) error {
	for _, v := range value {
		if err := v.Validate(c); err != nil {
			return err
		}
	}
	return nil
}
