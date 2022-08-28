package openapi3

// SchemaValidationOption describes options a user has when validating request / response bodies.
type SchemaValidationOption func(*schemaValidationSettings)

type schemaValidationSettings struct {
	failfast     bool
	multiError   bool
	asreq, asrep bool // exclusive (XOR) fields
}

// FailFast returns schema validation errors quicker.
func FailFast() SchemaValidationOption {
	return func(s *schemaValidationSettings) { s.failfast = true }
}

func MultiErrors() SchemaValidationOption {
	return func(s *schemaValidationSettings) { s.multiError = true }
}

func VisitAsRequest() SchemaValidationOption {
	return func(s *schemaValidationSettings) { s.asreq, s.asrep = true, false }
}
func VisitAsResponse() SchemaValidationOption {
	return func(s *schemaValidationSettings) { s.asreq, s.asrep = false, true }
}

func newSchemaValidationSettings(opts ...SchemaValidationOption) *schemaValidationSettings {
	settings := &schemaValidationSettings{}
	for _, opt := range opts {
		opt(settings)
	}
	return settings
}
