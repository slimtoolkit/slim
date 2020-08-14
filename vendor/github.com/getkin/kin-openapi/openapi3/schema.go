package openapi3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"unicode/utf16"

	"github.com/getkin/kin-openapi/jsoninfo"
)

var (
	// SchemaErrorDetailsDisabled disables printing of details about schema errors.
	SchemaErrorDetailsDisabled = false

	//SchemaFormatValidationDisabled disables validation of schema type formats.
	SchemaFormatValidationDisabled = false

	errSchema = errors.New("Input does not match the schema")

	ErrSchemaInputNaN = errors.New("NaN is not allowed")
	ErrSchemaInputInf = errors.New("Inf is not allowed")
)

// Float64Ptr is a helper for defining OpenAPI schemas.
func Float64Ptr(value float64) *float64 {
	return &value
}

// BoolPtr is a helper for defining OpenAPI schemas.
func BoolPtr(value bool) *bool {
	return &value
}

// Int64Ptr is a helper for defining OpenAPI schemas.
func Int64Ptr(value int64) *int64 {
	return &value
}

// Uint64Ptr is a helper for defining OpenAPI schemas.
func Uint64Ptr(value uint64) *uint64 {
	return &value
}

// Schema is specified by OpenAPI/Swagger 3.0 standard.
type Schema struct {
	ExtensionProps

	OneOf        []*SchemaRef  `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	AnyOf        []*SchemaRef  `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf        []*SchemaRef  `json:"allOf,omitempty" yaml:"allOf,omitempty"`
	Not          *SchemaRef    `json:"not,omitempty" yaml:"not,omitempty"`
	Type         string        `json:"type,omitempty" yaml:"type,omitempty"`
	Title        string        `json:"title,omitempty" yaml:"title,omitempty"`
	Format       string        `json:"format,omitempty" yaml:"format,omitempty"`
	Description  string        `json:"description,omitempty" yaml:"description,omitempty"`
	Enum         []interface{} `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default      interface{}   `json:"default,omitempty" yaml:"default,omitempty"`
	Example      interface{}   `json:"example,omitempty" yaml:"example,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	// Object-related, here for struct compactness
	AdditionalPropertiesAllowed *bool `json:"-" multijson:"additionalProperties,omitempty" yaml:"-"`
	// Array-related, here for struct compactness
	UniqueItems bool `json:"uniqueItems,omitempty" yaml:"uniqueItems,omitempty"`
	// Number-related, here for struct compactness
	ExclusiveMin bool `json:"exclusiveMinimum,omitempty" yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMax bool `json:"exclusiveMaximum,omitempty" yaml:"exclusiveMaximum,omitempty"`
	// Properties
	Nullable        bool        `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	ReadOnly        bool        `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	WriteOnly       bool        `json:"writeOnly,omitempty" yaml:"writeOnly,omitempty"`
	AllowEmptyValue bool        `json:"allowEmptyValue,omitempty" yaml:"allowEmptyValue,omitempty"`
	XML             interface{} `json:"xml,omitempty" yaml:"xml,omitempty"`

	// Number
	Min        *float64 `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Max        *float64 `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MultipleOf *float64 `json:"multipleOf,omitempty" yaml:"multipleOf,omitempty"`

	// String
	MinLength       uint64  `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength       *uint64 `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern         string  `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	compiledPattern *compiledPattern

	// Array
	MinItems uint64     `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems *uint64    `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	Items    *SchemaRef `json:"items,omitempty" yaml:"items,omitempty"`

	// Object
	Required             []string              `json:"required,omitempty" yaml:"required,omitempty"`
	Properties           map[string]*SchemaRef `json:"properties,omitempty" yaml:"properties,omitempty"`
	MinProps             uint64                `json:"minProperties,omitempty" yaml:"minProperties,omitempty"`
	MaxProps             *uint64               `json:"maxProperties,omitempty" yaml:"maxProperties,omitempty"`
	AdditionalProperties *SchemaRef            `json:"-" multijson:"additionalProperties,omitempty" yaml:"-"`
	Discriminator        *Discriminator        `json:"discriminator,omitempty" yaml:"discriminator,omitempty"`
}

func NewSchema() *Schema {
	return &Schema{}
}

func (schema *Schema) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(schema)
}

func (schema *Schema) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, schema)
}

func (schema *Schema) NewRef() *SchemaRef {
	return &SchemaRef{
		Value: schema,
	}
}

func NewOneOfSchema(schemas ...*Schema) *Schema {
	refs := make([]*SchemaRef, 0, len(schemas))
	for _, schema := range schemas {
		refs = append(refs, &SchemaRef{Value: schema})
	}
	return &Schema{
		OneOf: refs,
	}
}

func NewAnyOfSchema(schemas ...*Schema) *Schema {
	refs := make([]*SchemaRef, 0, len(schemas))
	for _, schema := range schemas {
		refs = append(refs, &SchemaRef{Value: schema})
	}
	return &Schema{
		AnyOf: refs,
	}
}

func NewAllOfSchema(schemas ...*Schema) *Schema {
	refs := make([]*SchemaRef, 0, len(schemas))
	for _, schema := range schemas {
		refs = append(refs, &SchemaRef{Value: schema})
	}
	return &Schema{
		AllOf: refs,
	}
}

func NewBoolSchema() *Schema {
	return &Schema{
		Type: "boolean",
	}
}

func NewFloat64Schema() *Schema {
	return &Schema{
		Type: "number",
	}
}

func NewIntegerSchema() *Schema {
	return &Schema{
		Type: "integer",
	}
}

func NewInt32Schema() *Schema {
	return &Schema{
		Type:   "integer",
		Format: "int32",
	}
}

func NewInt64Schema() *Schema {
	return &Schema{
		Type:   "integer",
		Format: "int64",
	}
}

func NewStringSchema() *Schema {
	return &Schema{
		Type: "string",
	}
}

func NewDateTimeSchema() *Schema {
	return &Schema{
		Type:   "string",
		Format: "date-time",
	}
}

func NewUUIDSchema() *Schema {
	return &Schema{
		Type:   "string",
		Format: "uuid",
	}
}

func NewBytesSchema() *Schema {
	return &Schema{
		Type:   "string",
		Format: "byte",
	}
}

func NewArraySchema() *Schema {
	return &Schema{
		Type: "array",
	}
}

func NewObjectSchema() *Schema {
	return &Schema{
		Type:       "object",
		Properties: make(map[string]*SchemaRef),
	}
}

type compiledPattern struct {
	Regexp    *regexp.Regexp
	ErrReason string
}

func (schema *Schema) WithNullable() *Schema {
	schema.Nullable = true
	return schema
}

func (schema *Schema) WithMin(value float64) *Schema {
	schema.Min = &value
	return schema
}

func (schema *Schema) WithMax(value float64) *Schema {
	schema.Max = &value
	return schema
}
func (schema *Schema) WithExclusiveMin(value bool) *Schema {
	schema.ExclusiveMin = value
	return schema
}

func (schema *Schema) WithExclusiveMax(value bool) *Schema {
	schema.ExclusiveMax = value
	return schema
}

func (schema *Schema) WithEnum(values ...interface{}) *Schema {
	schema.Enum = values
	return schema
}

func (schema *Schema) WithDefault(defaultValue interface{}) *Schema {
	schema.Default = defaultValue
	return schema
}

func (schema *Schema) WithFormat(value string) *Schema {
	schema.Format = value
	return schema
}

func (schema *Schema) WithLength(i int64) *Schema {
	n := uint64(i)
	schema.MinLength = n
	schema.MaxLength = &n
	return schema
}

func (schema *Schema) WithMinLength(i int64) *Schema {
	n := uint64(i)
	schema.MinLength = n
	return schema
}

func (schema *Schema) WithMaxLength(i int64) *Schema {
	n := uint64(i)
	schema.MaxLength = &n
	return schema
}

func (schema *Schema) WithLengthDecodedBase64(i int64) *Schema {
	n := uint64(i)
	v := (n*8 + 5) / 6
	schema.MinLength = v
	schema.MaxLength = &v
	return schema
}

func (schema *Schema) WithMinLengthDecodedBase64(i int64) *Schema {
	n := uint64(i)
	schema.MinLength = (n*8 + 5) / 6
	return schema
}

func (schema *Schema) WithMaxLengthDecodedBase64(i int64) *Schema {
	n := uint64(i)
	schema.MinLength = (n*8 + 5) / 6
	return schema
}

func (schema *Schema) WithPattern(pattern string) *Schema {
	schema.Pattern = pattern
	return schema
}

func (schema *Schema) WithItems(value *Schema) *Schema {
	schema.Items = &SchemaRef{
		Value: value,
	}
	return schema
}

func (schema *Schema) WithMinItems(i int64) *Schema {
	n := uint64(i)
	schema.MinItems = n
	return schema
}

func (schema *Schema) WithMaxItems(i int64) *Schema {
	n := uint64(i)
	schema.MaxItems = &n
	return schema
}

func (schema *Schema) WithUniqueItems(unique bool) *Schema {
	schema.UniqueItems = unique
	return schema
}

func (schema *Schema) WithProperty(name string, propertySchema *Schema) *Schema {
	return schema.WithPropertyRef(name, &SchemaRef{
		Value: propertySchema,
	})
}

func (schema *Schema) WithPropertyRef(name string, ref *SchemaRef) *Schema {
	properties := schema.Properties
	if properties == nil {
		properties = make(map[string]*SchemaRef)
		schema.Properties = properties
	}
	properties[name] = ref
	return schema
}

func (schema *Schema) WithProperties(properties map[string]*Schema) *Schema {
	result := make(map[string]*SchemaRef, len(properties))
	for k, v := range properties {
		result[k] = &SchemaRef{
			Value: v,
		}
	}
	schema.Properties = result
	return schema
}

func (schema *Schema) WithMinProperties(i int64) *Schema {
	n := uint64(i)
	schema.MinProps = n
	return schema
}

func (schema *Schema) WithMaxProperties(i int64) *Schema {
	n := uint64(i)
	schema.MaxProps = &n
	return schema
}

func (schema *Schema) WithAnyAdditionalProperties() *Schema {
	schema.AdditionalProperties = nil
	t := true
	schema.AdditionalPropertiesAllowed = &t
	return schema
}

func (schema *Schema) WithAdditionalProperties(v *Schema) *Schema {
	if v == nil {
		schema.AdditionalProperties = nil
	} else {
		schema.AdditionalProperties = &SchemaRef{
			Value: v,
		}
	}
	return schema
}

func (schema *Schema) IsEmpty() bool {
	if schema.Type != "" || schema.Format != "" || len(schema.Enum) != 0 ||
		schema.UniqueItems || schema.ExclusiveMin || schema.ExclusiveMax ||
		schema.Nullable ||
		schema.Min != nil || schema.Max != nil || schema.MultipleOf != nil ||
		schema.MinLength != 0 || schema.MaxLength != nil || schema.Pattern != "" ||
		schema.MinItems != 0 || schema.MaxItems != nil ||
		len(schema.Required) != 0 ||
		schema.MinProps != 0 || schema.MaxProps != nil {
		return false
	}
	if n := schema.Not; n != nil && !n.Value.IsEmpty() {
		return false
	}
	if ap := schema.AdditionalProperties; ap != nil && !ap.Value.IsEmpty() {
		return false
	}
	if apa := schema.AdditionalPropertiesAllowed; apa != nil && !*apa {
		return false
	}
	if items := schema.Items; items != nil && !items.Value.IsEmpty() {
		return false
	}
	for _, s := range schema.Properties {
		if !s.Value.IsEmpty() {
			return false
		}
	}
	for _, s := range schema.OneOf {
		if !s.Value.IsEmpty() {
			return false
		}
	}
	for _, s := range schema.AnyOf {
		if !s.Value.IsEmpty() {
			return false
		}
	}
	for _, s := range schema.AllOf {
		if !s.Value.IsEmpty() {
			return false
		}
	}
	return true
}

func (schema *Schema) Validate(c context.Context) error {
	return schema.validate(c, []*Schema{})
}

func (schema *Schema) validate(c context.Context, stack []*Schema) (err error) {
	for _, existing := range stack {
		if existing == schema {
			return
		}
	}
	stack = append(stack, schema)

	for _, item := range schema.OneOf {
		v := item.Value
		if v == nil {
			return foundUnresolvedRef(item.Ref)
		}
		if err = v.validate(c, stack); err == nil {
			return
		}
	}

	for _, item := range schema.AnyOf {
		v := item.Value
		if v == nil {
			return foundUnresolvedRef(item.Ref)
		}
		if err = v.validate(c, stack); err != nil {
			return
		}
	}

	for _, item := range schema.AllOf {
		v := item.Value
		if v == nil {
			return foundUnresolvedRef(item.Ref)
		}
		if err = v.validate(c, stack); err != nil {
			return
		}
	}

	if ref := schema.Not; ref != nil {
		v := ref.Value
		if v == nil {
			return foundUnresolvedRef(ref.Ref)
		}
		if err = v.validate(c, stack); err != nil {
			return
		}
	}

	schemaType := schema.Type
	switch schemaType {
	case "":
	case "boolean":
	case "number":
		if format := schema.Format; len(format) > 0 {
			switch format {
			case "float", "double":
			default:
				if !SchemaFormatValidationDisabled {
					return unsupportedFormat(format)
				}
			}
		}
	case "integer":
		if format := schema.Format; len(format) > 0 {
			switch format {
			case "int32", "int64":
			default:
				if !SchemaFormatValidationDisabled {
					return unsupportedFormat(format)
				}
			}
		}
	case "string":
		if format := schema.Format; len(format) > 0 {
			switch format {
			// Supported by OpenAPIv3.0.1:
			case "byte", "binary", "date", "date-time", "password":
				// In JSON Draft-07 (not validated yet though):
			case "regex":
			case "time", "email", "idn-email":
			case "hostname", "idn-hostname", "ipv4", "ipv6":
			case "uri", "uri-reference", "iri", "iri-reference", "uri-template":
			case "json-pointer", "relative-json-pointer":
			default:
				// Try to check for custom defined formats
				if _, ok := SchemaStringFormats[format]; !ok && !SchemaFormatValidationDisabled {
					return unsupportedFormat(format)
				}
			}
		}
	case "array":
		if schema.Items == nil {
			return errors.New("When schema type is 'array', schema 'items' must be non-null")
		}
	case "object":
	default:
		return fmt.Errorf("Unsupported 'type' value '%s'", schemaType)
	}

	if ref := schema.Items; ref != nil {
		v := ref.Value
		if v == nil {
			return foundUnresolvedRef(ref.Ref)
		}
		if err = v.validate(c, stack); err != nil {
			return
		}
	}

	for _, ref := range schema.Properties {
		v := ref.Value
		if v == nil {
			return foundUnresolvedRef(ref.Ref)
		}
		if err = v.validate(c, stack); err != nil {
			return
		}
	}

	if ref := schema.AdditionalProperties; ref != nil {
		v := ref.Value
		if v == nil {
			return foundUnresolvedRef(ref.Ref)
		}
		if err = v.validate(c, stack); err != nil {
			return
		}
	}

	return
}

func (schema *Schema) IsMatching(value interface{}) bool {
	return schema.visitJSON(value, true) == nil
}

func (schema *Schema) IsMatchingJSONBoolean(value bool) bool {
	return schema.visitJSON(value, true) == nil
}

func (schema *Schema) IsMatchingJSONNumber(value float64) bool {
	return schema.visitJSON(value, true) == nil
}

func (schema *Schema) IsMatchingJSONString(value string) bool {
	return schema.visitJSON(value, true) == nil
}

func (schema *Schema) IsMatchingJSONArray(value []interface{}) bool {
	return schema.visitJSON(value, true) == nil
}

func (schema *Schema) IsMatchingJSONObject(value map[string]interface{}) bool {
	return schema.visitJSON(value, true) == nil
}

func (schema *Schema) VisitJSON(value interface{}) error {
	return schema.visitJSON(value, false)
}

func (schema *Schema) visitJSON(value interface{}, fast bool) (err error) {
	switch value := value.(type) {
	case nil:
		return schema.visitJSONNull(fast)
	case float64:
		if math.IsNaN(value) {
			return ErrSchemaInputNaN
		}
		if math.IsInf(value, 0) {
			return ErrSchemaInputInf
		}
	}

	if schema.IsEmpty() {
		return
	}
	if err = schema.visitSetOperations(value, fast); err != nil {
		return
	}

	switch value := value.(type) {
	case nil:
		return schema.visitJSONNull(fast)
	case bool:
		return schema.visitJSONBoolean(value, fast)
	case float64:
		return schema.visitJSONNumber(value, fast)
	case string:
		return schema.visitJSONString(value, fast)
	case []interface{}:
		return schema.visitJSONArray(value, fast)
	case map[string]interface{}:
		return schema.visitJSONObject(value, fast)
	default:
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "type",
			Reason:      fmt.Sprintf("Not a JSON value: %T", value),
		}
	}
}

func (schema *Schema) visitSetOperations(value interface{}, fast bool) (err error) {
	if enum := schema.Enum; len(enum) != 0 {
		for _, v := range enum {
			if value == v {
				return
			}
		}
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "enum",
			Reason:      "JSON value is not one of the allowed values",
		}
	}

	if ref := schema.Not; ref != nil {
		v := ref.Value
		if v == nil {
			return foundUnresolvedRef(ref.Ref)
		}
		if err := v.visitJSON(value, true); err == nil {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "not",
			}
		}
	}

	if v := schema.OneOf; len(v) > 0 {
		ok := 0
		for _, item := range v {
			v := item.Value
			if v == nil {
				return foundUnresolvedRef(item.Ref)
			}
			if err := v.visitJSON(value, true); err == nil {
				ok++
			}
		}
		if ok != 1 {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "oneOf",
			}
		}
	}

	if v := schema.AnyOf; len(v) > 0 {
		ok := false
		for _, item := range v {
			v := item.Value
			if v == nil {
				return foundUnresolvedRef(item.Ref)
			}
			if err := v.visitJSON(value, true); err == nil {
				ok = true
				break
			}
		}
		if !ok {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "anyOf",
			}
		}
	}

	for _, item := range schema.AllOf {
		v := item.Value
		if v == nil {
			return foundUnresolvedRef(item.Ref)
		}
		if err := v.visitJSON(value, false); err != nil {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "allOf",
				Origin:      err,
			}
		}
	}
	return
}

func (schema *Schema) visitJSONNull(fast bool) (err error) {
	if schema.Nullable {
		return
	}
	if fast {
		return errSchema
	}
	return &SchemaError{
		Value:       nil,
		Schema:      schema,
		SchemaField: "nullable",
		Reason:      "Value is not nullable",
	}
}

func (schema *Schema) VisitJSONBoolean(value bool) error {
	return schema.visitJSONBoolean(value, false)
}

func (schema *Schema) visitJSONBoolean(value bool, fast bool) (err error) {
	if schemaType := schema.Type; schemaType != "" && schemaType != "boolean" {
		return schema.expectedType("boolean", fast)
	}
	return
}

func (schema *Schema) VisitJSONNumber(value float64) error {
	return schema.visitJSONNumber(value, false)
}

func (schema *Schema) visitJSONNumber(value float64, fast bool) (err error) {
	schemaType := schema.Type
	if schemaType == "integer" {
		if bigFloat := big.NewFloat(value); !bigFloat.IsInt() {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "type",
				Reason:      "Value must be an integer",
			}
		}
	} else if schemaType != "" && schemaType != "number" {
		return schema.expectedType("number, integer", fast)
	}

	// "exclusiveMinimum"
	if v := schema.ExclusiveMin; v && !(*schema.Min < value) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "exclusiveMinimum",
			Reason:      fmt.Sprintf("Number must be more than %g", *schema.Min),
		}
	}

	// "exclusiveMaximum"
	if v := schema.ExclusiveMax; v && !(*schema.Max > value) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "exclusiveMaximum",
			Reason:      fmt.Sprintf("Number must be less than %g", *schema.Max),
		}
	}

	// "minimum"
	if v := schema.Min; v != nil && !(*v <= value) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "minimum",
			Reason:      fmt.Sprintf("Number must be at least %g", *v),
		}
	}

	// "maximum"
	if v := schema.Max; v != nil && !(*v >= value) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "maximum",
			Reason:      fmt.Sprintf("Number must be most %g", *v),
		}
	}

	// "multipleOf"
	if v := schema.MultipleOf; v != nil {
		// "A numeric instance is valid only if division by this keyword's
		//    value results in an integer."
		if bigFloat := big.NewFloat(value / *v); !bigFloat.IsInt() {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "multipleOf",
			}
		}
	}
	return
}

func (schema *Schema) VisitJSONString(value string) error {
	return schema.visitJSONString(value, false)
}

func (schema *Schema) visitJSONString(value string, fast bool) (err error) {
	if schemaType := schema.Type; schemaType != "" && schemaType != "string" {
		return schema.expectedType("string", fast)
	}

	// "minLength" and "maxLength"
	minLength := schema.MinLength
	maxLength := schema.MaxLength
	if minLength != 0 || maxLength != nil {
		// JSON schema string lengths are UTF-16, not UTF-8!
		length := int64(0)
		for _, r := range value {
			if utf16.IsSurrogate(r) {
				length += 2
			} else {
				length++
			}
		}
		if minLength != 0 && length < int64(minLength) {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "minLength",
				Reason:      fmt.Sprintf("Minimum string length is %d", minLength),
			}
		}
		if maxLength != nil && length > int64(*maxLength) {
			if fast {
				return errSchema
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "maxLength",
				Reason:      fmt.Sprintf("Maximum string length is %d", *maxLength),
			}
		}
	}

	// "format" and "pattern"
	cp := schema.compiledPattern
	if cp == nil {
		pattern := schema.Pattern
		if v := schema.Pattern; len(v) > 0 {
			// Pattern
			re, err := regexp.Compile(v)
			if err != nil {
				return fmt.Errorf("Error while compiling regular expression '%s': %v", pattern, err)
			}
			cp = &compiledPattern{
				Regexp:    re,
				ErrReason: "JSON string doesn't match the regular expression '" + v + "'",
			}
			schema.compiledPattern = cp
		} else if v := schema.Format; len(v) > 0 {
			// No pattern, but does have a format
			re := SchemaStringFormats[v]
			if re != nil {
				cp = &compiledPattern{
					Regexp:    re,
					ErrReason: "JSON string doesn't match the format '" + v + " (regular expression `" + re.String() + "`)'",
				}
				schema.compiledPattern = cp
			}
		}
	}
	if cp != nil {
		if !cp.Regexp.MatchString(value) {
			field := "format"
			if schema.Pattern != "" {
				field = "pattern"
			}
			return &SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: field,
				Reason:      cp.ErrReason,
			}
		}
	}
	return
}

func (schema *Schema) VisitJSONArray(value []interface{}) error {
	return schema.visitJSONArray(value, false)
}

func (schema *Schema) visitJSONArray(value []interface{}, fast bool) (err error) {
	if schemaType := schema.Type; schemaType != "" && schemaType != "array" {
		return schema.expectedType("array", fast)
	}

	lenValue := int64(len(value))

	// "minItems"
	if v := schema.MinItems; v != 0 && lenValue < int64(v) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "minItems",
			Reason:      fmt.Sprintf("Minimum number of items is %d", v),
		}
	}

	// "maxItems"
	if v := schema.MaxItems; v != nil && lenValue > int64(*v) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "maxItems",
			Reason:      fmt.Sprintf("Maximum number of items is %d", *v),
		}
	}

	// "uniqueItems"
	if v := schema.UniqueItems; v && !sliceUniqueItemsChecker(value) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "uniqueItems",
			Reason:      fmt.Sprintf("Duplicate items found"),
		}
	}

	// "items"
	if itemSchemaRef := schema.Items; itemSchemaRef != nil {
		itemSchema := itemSchemaRef.Value
		if itemSchema == nil {
			return foundUnresolvedRef(itemSchemaRef.Ref)
		}
		for i, item := range value {
			if err := itemSchema.VisitJSON(item); err != nil {
				return markSchemaErrorIndex(err, i)
			}
		}
	}
	return
}

func (schema *Schema) VisitJSONObject(value map[string]interface{}) error {
	return schema.visitJSONObject(value, false)
}

func (schema *Schema) visitJSONObject(value map[string]interface{}, fast bool) (err error) {
	if schemaType := schema.Type; schemaType != "" && schemaType != "object" {
		return schema.expectedType("object", fast)
	}

	// "properties"
	properties := schema.Properties
	lenValue := int64(len(value))

	// "minProperties"
	if v := schema.MinProps; v != 0 && lenValue < int64(v) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "minProperties",
			Reason:      fmt.Sprintf("There must be at least %d properties", v),
		}
	}

	// "maxProperties"
	if v := schema.MaxProps; v != nil && lenValue > int64(*v) {
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "maxProperties",
			Reason:      fmt.Sprintf("There must be at most %d properties", *v),
		}
	}

	// "additionalProperties"
	var additionalProperties *Schema
	if ref := schema.AdditionalProperties; ref != nil {
		additionalProperties = ref.Value
	}
	for k, v := range value {
		if properties != nil {
			propertyRef := properties[k]
			if propertyRef != nil {
				p := propertyRef.Value
				if p == nil {
					return foundUnresolvedRef(propertyRef.Ref)
				}
				if err := p.VisitJSON(v); err != nil {
					if fast {
						return errSchema
					}
					return markSchemaErrorKey(err, k)
				}
				continue
			}
		}
		allowed := schema.AdditionalPropertiesAllowed
		if additionalProperties != nil || allowed == nil || (allowed != nil && *allowed) {
			if additionalProperties != nil {
				if err := additionalProperties.VisitJSON(v); err != nil {
					if fast {
						return errSchema
					}
					return markSchemaErrorKey(err, k)
				}
			}
			continue
		}
		if fast {
			return errSchema
		}
		return &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: "properties",
			Reason:      fmt.Sprintf("Property '%s' is unsupported", k),
		}
	}
	for _, k := range schema.Required {
		if _, ok := value[k]; !ok {
			if fast {
				return errSchema
			}
			return markSchemaErrorKey(&SchemaError{
				Value:       value,
				Schema:      schema,
				SchemaField: "required",
				Reason:      fmt.Sprintf("Property '%s' is missing", k),
			}, k)
		}
	}
	return
}

func (schema *Schema) expectedType(typ string, fast bool) error {
	if fast {
		return errSchema
	}
	return &SchemaError{
		Value:       typ,
		Schema:      schema,
		SchemaField: "type",
		Reason:      "Field must be set to " + schema.Type + " or not be present",
	}
}

type SchemaError struct {
	Value       interface{}
	reversePath []string
	Schema      *Schema
	SchemaField string
	Reason      string
	Origin      error
}

func markSchemaErrorKey(err error, key string) error {
	if v, ok := err.(*SchemaError); ok {
		v.reversePath = append(v.reversePath, key)
		return v
	}
	return err
}

func markSchemaErrorIndex(err error, index int) error {
	if v, ok := err.(*SchemaError); ok {
		v.reversePath = append(v.reversePath, strconv.FormatInt(int64(index), 10))
		return v
	}
	return err
}

func (err *SchemaError) JSONPointer() []string {
	reversePath := err.reversePath
	path := append([]string(nil), reversePath...)
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func (err *SchemaError) Error() string {
	if err.Origin != nil {
		return err.Origin.Error()
	}

	buf := bytes.NewBuffer(make([]byte, 0, 256))
	if len(err.reversePath) > 0 {
		buf.WriteString(`Error at "`)
		reversePath := err.reversePath
		for i := len(reversePath) - 1; i >= 0; i-- {
			buf.WriteByte('/')
			buf.WriteString(reversePath[i])
		}
		buf.WriteString(`":`)
	}
	reason := err.Reason
	if reason == "" {
		buf.WriteString(`Doesn't match schema "`)
		buf.WriteString(err.SchemaField)
		buf.WriteString(`"`)
	} else {
		buf.WriteString(reason)
	}
	if !SchemaErrorDetailsDisabled {
		buf.WriteString("\nSchema:\n  ")
		encoder := json.NewEncoder(buf)
		encoder.SetIndent("  ", "  ")
		if err := encoder.Encode(err.Schema); err != nil {
			panic(err)
		}
		buf.WriteString("\nValue:\n  ")
		if err := encoder.Encode(err.Value); err != nil {
			panic(err)
		}
	}
	return buf.String()
}

func isSliceOfUniqueItems(xs []interface{}) bool {
	s := len(xs)
	m := make(map[string]struct{}, s)
	for _, x := range xs {
		// The input slice is coverted from a JSON string, there shall
		// have no error when covert it back.
		key, _ := json.Marshal(&x)
		m[string(key)] = struct{}{}
	}
	return s == len(m)
}

// SliceUniqueItemsChecker is an function used to check if an given slice
// have unique items.
type SliceUniqueItemsChecker func(items []interface{}) bool

// By default using predefined func isSliceOfUniqueItems which make use of
// json.Marshal to generate a key for map used to check if a given slice
// have unique items.
var sliceUniqueItemsChecker SliceUniqueItemsChecker = isSliceOfUniqueItems

// RegisterArrayUniqueItemsChecker is used to register a customized function
// used to check if JSON array have unique items.
func RegisterArrayUniqueItemsChecker(fn SliceUniqueItemsChecker) {
	sliceUniqueItemsChecker = fn
}

func unsupportedFormat(format string) error {
	return fmt.Errorf("Unsupported 'format' value '%s'", format)
}
