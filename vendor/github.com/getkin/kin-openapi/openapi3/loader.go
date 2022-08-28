package openapi3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
)

func foundUnresolvedRef(ref string) error {
	return fmt.Errorf("found unresolved ref: %q", ref)
}

func failedToResolveRefFragmentPart(value, what string) error {
	return fmt.Errorf("failed to resolve %q in fragment in URI: %q", what, value)
}

// Loader helps deserialize an OpenAPIv3 document
type Loader struct {
	// IsExternalRefsAllowed enables visiting other files
	IsExternalRefsAllowed bool

	// ReadFromURIFunc allows overriding the any file/URL reading func
	ReadFromURIFunc func(loader *Loader, url *url.URL) ([]byte, error)

	Context context.Context

	rootDir string

	visitedPathItemRefs map[string]struct{}

	visitedDocuments map[string]*T

	visitedExample        map[*Example]struct{}
	visitedHeader         map[*Header]struct{}
	visitedLink           map[*Link]struct{}
	visitedParameter      map[*Parameter]struct{}
	visitedRequestBody    map[*RequestBody]struct{}
	visitedResponse       map[*Response]struct{}
	visitedSchema         map[*Schema]struct{}
	visitedSecurityScheme map[*SecurityScheme]struct{}
}

// NewLoader returns an empty Loader
func NewLoader() *Loader {
	return &Loader{}
}

func (loader *Loader) resetVisitedPathItemRefs() {
	loader.visitedPathItemRefs = make(map[string]struct{})
}

// LoadFromURI loads a spec from a remote URL
func (loader *Loader) LoadFromURI(location *url.URL) (*T, error) {
	loader.resetVisitedPathItemRefs()
	return loader.loadFromURIInternal(location)
}

// LoadFromFile loads a spec from a local file path
func (loader *Loader) LoadFromFile(location string) (*T, error) {
	loader.rootDir = path.Dir(location)
	return loader.LoadFromURI(&url.URL{Path: filepath.ToSlash(location)})
}

func (loader *Loader) loadFromURIInternal(location *url.URL) (*T, error) {
	data, err := loader.readURL(location)
	if err != nil {
		return nil, err
	}
	return loader.loadFromDataWithPathInternal(data, location)
}

func (loader *Loader) allowsExternalRefs(ref string) (err error) {
	if !loader.IsExternalRefsAllowed {
		err = fmt.Errorf("encountered disallowed external reference: %q", ref)
	}
	return
}

// loadSingleElementFromURI reads the data from ref and unmarshals to the passed element.
func (loader *Loader) loadSingleElementFromURI(ref string, rootPath *url.URL, element interface{}) (*url.URL, error) {
	if err := loader.allowsExternalRefs(ref); err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	if fragment := parsedURL.Fragment; fragment != "" {
		return nil, fmt.Errorf("unexpected ref fragment %q", fragment)
	}

	resolvedPath, err := resolvePath(rootPath, parsedURL)
	if err != nil {
		return nil, fmt.Errorf("could not resolve path: %v", err)
	}

	data, err := loader.readURL(resolvedPath)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, element); err != nil {
		return nil, err
	}

	return resolvedPath, nil
}

func (loader *Loader) readURL(location *url.URL) ([]byte, error) {
	if f := loader.ReadFromURIFunc; f != nil {
		return f(loader, location)
	}

	if location.Scheme != "" && location.Host != "" {
		resp, err := http.Get(location.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode > 399 {
			return nil, fmt.Errorf("error loading %q: request returned status code %d", location.String(), resp.StatusCode)
		}
		return ioutil.ReadAll(resp.Body)
	}
	if location.Scheme != "" || location.Host != "" || location.RawQuery != "" {
		return nil, fmt.Errorf("unsupported URI: %q", location.String())
	}
	return ioutil.ReadFile(location.Path)
}

// LoadFromData loads a spec from a byte array
func (loader *Loader) LoadFromData(data []byte) (*T, error) {
	loader.resetVisitedPathItemRefs()
	doc := &T{}
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, err
	}
	if err := loader.ResolveRefsIn(doc, nil); err != nil {
		return nil, err
	}
	return doc, nil
}

// LoadFromDataWithPath takes the OpenAPI document data in bytes and a path where the resolver can find referred
// elements and returns a *T with all resolved data or an error if unable to load data or resolve refs.
func (loader *Loader) LoadFromDataWithPath(data []byte, location *url.URL) (*T, error) {
	loader.resetVisitedPathItemRefs()
	return loader.loadFromDataWithPathInternal(data, location)
}

func (loader *Loader) loadFromDataWithPathInternal(data []byte, location *url.URL) (*T, error) {
	if loader.visitedDocuments == nil {
		loader.visitedDocuments = make(map[string]*T)
	}
	uri := location.String()
	if doc, ok := loader.visitedDocuments[uri]; ok {
		return doc, nil
	}

	doc := &T{}
	loader.visitedDocuments[uri] = doc

	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, err
	}
	if err := loader.ResolveRefsIn(doc, location); err != nil {
		return nil, err
	}

	return doc, nil
}

// ResolveRefsIn expands references if for instance spec was just unmarshalled
func (loader *Loader) ResolveRefsIn(doc *T, location *url.URL) (err error) {
	if loader.visitedPathItemRefs == nil {
		loader.resetVisitedPathItemRefs()
	}

	// Visit all components
	components := doc.Components
	for _, component := range components.Headers {
		if err = loader.resolveHeaderRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.Parameters {
		if err = loader.resolveParameterRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.RequestBodies {
		if err = loader.resolveRequestBodyRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.Responses {
		if err = loader.resolveResponseRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.Schemas {
		if err = loader.resolveSchemaRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.SecuritySchemes {
		if err = loader.resolveSecuritySchemeRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.Examples {
		if err = loader.resolveExampleRef(doc, component, location); err != nil {
			return
		}
	}
	for _, component := range components.Callbacks {
		if err = loader.resolveCallbackRef(doc, component, location); err != nil {
			return
		}
	}

	// Visit all operations
	for entrypoint, pathItem := range doc.Paths {
		if pathItem == nil {
			continue
		}
		if err = loader.resolvePathItemRef(doc, entrypoint, pathItem, location); err != nil {
			return
		}
	}

	return
}

func join(basePath *url.URL, relativePath *url.URL) (*url.URL, error) {
	if basePath == nil {
		return relativePath, nil
	}
	newPath, err := url.Parse(basePath.String())
	if err != nil {
		return nil, fmt.Errorf("cannot copy path: %q", basePath.String())
	}
	newPath.Path = path.Join(path.Dir(newPath.Path), relativePath.Path)
	return newPath, nil
}

func resolvePath(basePath *url.URL, componentPath *url.URL) (*url.URL, error) {
	if componentPath.Scheme == "" && componentPath.Host == "" {
		// support absolute paths
		if componentPath.Path[0] == '/' {
			return componentPath, nil
		}
		return join(basePath, componentPath)
	}
	return componentPath, nil
}

func isSingleRefElement(ref string) bool {
	return !strings.Contains(ref, "#")
}

func (loader *Loader) resolveComponent(
	doc *T,
	ref string,
	path *url.URL,
	resolved interface{},
) (
	componentPath *url.URL,
	err error,
) {
	if doc, ref, componentPath, err = loader.resolveRef(doc, ref, path); err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("cannot parse reference: %q: %v", ref, parsedURL)
	}
	fragment := parsedURL.Fragment
	if !strings.HasPrefix(fragment, "/") {
		return nil, fmt.Errorf("expected fragment prefix '#/' in URI %q", ref)
	}

	drill := func(cursor interface{}) (interface{}, error) {
		for _, pathPart := range strings.Split(fragment[1:], "/") {
			pathPart = unescapeRefString(pathPart)

			if cursor, err = drillIntoField(cursor, pathPart); err != nil {
				e := failedToResolveRefFragmentPart(ref, pathPart)
				return nil, fmt.Errorf("%s: %s", e.Error(), err.Error())
			}
			if cursor == nil {
				return nil, failedToResolveRefFragmentPart(ref, pathPart)
			}
		}
		return cursor, nil
	}
	var cursor interface{}
	if cursor, err = drill(doc); err != nil {
		if path == nil {
			return nil, err
		}
		var err2 error
		data, err2 := loader.readURL(path)
		if err2 != nil {
			return nil, err
		}
		if err2 = yaml.Unmarshal(data, &cursor); err2 != nil {
			return nil, err
		}
		if cursor, err2 = drill(cursor); err2 != nil || cursor == nil {
			return nil, err
		}
		err = nil
	}

	switch {
	case reflect.TypeOf(cursor) == reflect.TypeOf(resolved):
		reflect.ValueOf(resolved).Elem().Set(reflect.ValueOf(cursor).Elem())
		return componentPath, nil

	case reflect.TypeOf(cursor) == reflect.TypeOf(map[string]interface{}{}):
		codec := func(got, expect interface{}) error {
			enc, err := json.Marshal(got)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(enc, expect); err != nil {
				return err
			}
			return nil
		}
		if err := codec(cursor, resolved); err != nil {
			return nil, fmt.Errorf("bad data in %q", ref)
		}
		return componentPath, nil

	default:
		return nil, fmt.Errorf("bad data in %q", ref)
	}
}

func drillIntoField(cursor interface{}, fieldName string) (interface{}, error) {
	// Special case due to multijson
	if s, ok := cursor.(*SchemaRef); ok && fieldName == "additionalProperties" {
		if ap := s.Value.AdditionalPropertiesAllowed; ap != nil {
			return *ap, nil
		}
		return s.Value.AdditionalProperties, nil
	}

	switch val := reflect.Indirect(reflect.ValueOf(cursor)); val.Kind() {
	case reflect.Map:
		elementValue := val.MapIndex(reflect.ValueOf(fieldName))
		if !elementValue.IsValid() {
			return nil, fmt.Errorf("map key %q not found", fieldName)
		}
		return elementValue.Interface(), nil

	case reflect.Slice:
		i, err := strconv.ParseUint(fieldName, 10, 32)
		if err != nil {
			return nil, err
		}
		index := int(i)
		if 0 > index || index >= val.Len() {
			return nil, errors.New("slice index out of bounds")
		}
		return val.Index(index).Interface(), nil

	case reflect.Struct:
		hasFields := false
		for i := 0; i < val.NumField(); i++ {
			hasFields = true
			field := val.Type().Field(i)
			tagValue := field.Tag.Get("yaml")
			yamlKey := strings.Split(tagValue, ",")[0]
			if yamlKey == "-" {
				tagValue := field.Tag.Get("multijson")
				yamlKey = strings.Split(tagValue, ",")[0]
			}
			if yamlKey == fieldName {
				return val.Field(i).Interface(), nil
			}
		}
		// if cursor is a "ref wrapper" struct (e.g. RequestBodyRef),
		if _, ok := val.Type().FieldByName("Value"); ok {
			// try digging into its Value field
			return drillIntoField(val.FieldByName("Value").Interface(), fieldName)
		}
		if hasFields {
			if ff := val.Type().Field(0); ff.PkgPath == "" && ff.Name == "ExtensionProps" {
				extensions := val.Field(0).Interface().(ExtensionProps).Extensions
				if enc, ok := extensions[fieldName]; ok {
					var dec interface{}
					if err := json.Unmarshal(enc.(json.RawMessage), &dec); err != nil {
						return nil, err
					}
					return dec, nil
				}
			}
		}
		return nil, fmt.Errorf("struct field %q not found", fieldName)

	default:
		return nil, errors.New("not a map, slice nor struct")
	}
}

func (loader *Loader) documentPathForRecursiveRef(current *url.URL, resolvedRef string) *url.URL {
	if loader.rootDir == "" {
		return current
	}
	return &url.URL{Path: path.Join(loader.rootDir, resolvedRef)}

}

func (loader *Loader) resolveRef(doc *T, ref string, path *url.URL) (*T, string, *url.URL, error) {
	if ref != "" && ref[0] == '#' {
		return doc, ref, path, nil
	}

	if err := loader.allowsExternalRefs(ref); err != nil {
		return nil, "", nil, err
	}

	parsedURL, err := url.Parse(ref)
	if err != nil {
		return nil, "", nil, fmt.Errorf("cannot parse reference: %q: %v", ref, parsedURL)
	}
	fragment := parsedURL.Fragment
	parsedURL.Fragment = ""

	var resolvedPath *url.URL
	if resolvedPath, err = resolvePath(path, parsedURL); err != nil {
		return nil, "", nil, fmt.Errorf("error resolving path: %v", err)
	}

	if doc, err = loader.loadFromURIInternal(resolvedPath); err != nil {
		return nil, "", nil, fmt.Errorf("error resolving reference %q: %v", ref, err)
	}

	return doc, "#" + fragment, resolvedPath, nil
}

func (loader *Loader) resolveHeaderRef(doc *T, component *HeaderRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedHeader == nil {
			loader.visitedHeader = make(map[*Header]struct{})
		}
		if _, ok := loader.visitedHeader[component.Value]; ok {
			return nil
		}
		loader.visitedHeader[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid header: value MUST be an object")
	}
	if ref := component.Ref; ref != "" {
		if isSingleRefElement(ref) {
			var header Header
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &header); err != nil {
				return err
			}
			component.Value = &header
		} else {
			var resolved HeaderRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveHeaderRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			documentPath = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	value := component.Value
	if value == nil {
		return nil
	}

	if schema := value.Schema; schema != nil {
		if err := loader.resolveSchemaRef(doc, schema, documentPath); err != nil {
			return err
		}
	}
	return nil
}

func (loader *Loader) resolveParameterRef(doc *T, component *ParameterRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedParameter == nil {
			loader.visitedParameter = make(map[*Parameter]struct{})
		}
		if _, ok := loader.visitedParameter[component.Value]; ok {
			return nil
		}
		loader.visitedParameter[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid parameter: value MUST be an object")
	}
	ref := component.Ref
	if ref != "" {
		if isSingleRefElement(ref) {
			var param Parameter
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &param); err != nil {
				return err
			}
			component.Value = &param
		} else {
			var resolved ParameterRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveParameterRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			documentPath = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	value := component.Value
	if value == nil {
		return nil
	}

	if value.Content != nil && value.Schema != nil {
		return errors.New("cannot contain both schema and content in a parameter")
	}
	for _, contentType := range value.Content {
		if schema := contentType.Schema; schema != nil {
			if err := loader.resolveSchemaRef(doc, schema, documentPath); err != nil {
				return err
			}
		}
	}
	if schema := value.Schema; schema != nil {
		if err := loader.resolveSchemaRef(doc, schema, documentPath); err != nil {
			return err
		}
	}
	return nil
}

func (loader *Loader) resolveRequestBodyRef(doc *T, component *RequestBodyRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedRequestBody == nil {
			loader.visitedRequestBody = make(map[*RequestBody]struct{})
		}
		if _, ok := loader.visitedRequestBody[component.Value]; ok {
			return nil
		}
		loader.visitedRequestBody[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid requestBody: value MUST be an object")
	}
	if ref := component.Ref; ref != "" {
		if isSingleRefElement(ref) {
			var requestBody RequestBody
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &requestBody); err != nil {
				return err
			}
			component.Value = &requestBody
		} else {
			var resolved RequestBodyRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err = loader.resolveRequestBodyRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			documentPath = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	value := component.Value
	if value == nil {
		return nil
	}

	for _, contentType := range value.Content {
		for name, example := range contentType.Examples {
			if err := loader.resolveExampleRef(doc, example, documentPath); err != nil {
				return err
			}
			contentType.Examples[name] = example
		}
		if schema := contentType.Schema; schema != nil {
			if err := loader.resolveSchemaRef(doc, schema, documentPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (loader *Loader) resolveResponseRef(doc *T, component *ResponseRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedResponse == nil {
			loader.visitedResponse = make(map[*Response]struct{})
		}
		if _, ok := loader.visitedResponse[component.Value]; ok {
			return nil
		}
		loader.visitedResponse[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid response: value MUST be an object")
	}
	ref := component.Ref
	if ref != "" {
		if isSingleRefElement(ref) {
			var resp Response
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &resp); err != nil {
				return err
			}
			component.Value = &resp
		} else {
			var resolved ResponseRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveResponseRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			documentPath = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	value := component.Value
	if value == nil {
		return nil
	}

	for _, header := range value.Headers {
		if err := loader.resolveHeaderRef(doc, header, documentPath); err != nil {
			return err
		}
	}
	for _, contentType := range value.Content {
		if contentType == nil {
			continue
		}
		for name, example := range contentType.Examples {
			if err := loader.resolveExampleRef(doc, example, documentPath); err != nil {
				return err
			}
			contentType.Examples[name] = example
		}
		if schema := contentType.Schema; schema != nil {
			if err := loader.resolveSchemaRef(doc, schema, documentPath); err != nil {
				return err
			}
			contentType.Schema = schema
		}
	}
	for _, link := range value.Links {
		if err := loader.resolveLinkRef(doc, link, documentPath); err != nil {
			return err
		}
	}
	return nil
}

func (loader *Loader) resolveSchemaRef(doc *T, component *SchemaRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedSchema == nil {
			loader.visitedSchema = make(map[*Schema]struct{})
		}
		if _, ok := loader.visitedSchema[component.Value]; ok {
			return nil
		}
		loader.visitedSchema[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid schema: value MUST be an object")
	}
	ref := component.Ref
	if ref != "" {
		if isSingleRefElement(ref) {
			var schema Schema
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &schema); err != nil {
				return err
			}
			component.Value = &schema
		} else {
			var resolved SchemaRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveSchemaRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			documentPath = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	value := component.Value
	if value == nil {
		return nil
	}

	// ResolveRefs referred schemas
	if v := value.Items; v != nil {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	for _, v := range value.Properties {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	if v := value.AdditionalProperties; v != nil {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	if v := value.Not; v != nil {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	for _, v := range value.AllOf {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	for _, v := range value.AnyOf {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	for _, v := range value.OneOf {
		if err := loader.resolveSchemaRef(doc, v, documentPath); err != nil {
			return err
		}
	}
	return nil
}

func (loader *Loader) resolveSecuritySchemeRef(doc *T, component *SecuritySchemeRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedSecurityScheme == nil {
			loader.visitedSecurityScheme = make(map[*SecurityScheme]struct{})
		}
		if _, ok := loader.visitedSecurityScheme[component.Value]; ok {
			return nil
		}
		loader.visitedSecurityScheme[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid securityScheme: value MUST be an object")
	}
	if ref := component.Ref; ref != "" {
		if isSingleRefElement(ref) {
			var scheme SecurityScheme
			if _, err = loader.loadSingleElementFromURI(ref, documentPath, &scheme); err != nil {
				return err
			}
			component.Value = &scheme
		} else {
			var resolved SecuritySchemeRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveSecuritySchemeRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			_ = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	return nil
}

func (loader *Loader) resolveExampleRef(doc *T, component *ExampleRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedExample == nil {
			loader.visitedExample = make(map[*Example]struct{})
		}
		if _, ok := loader.visitedExample[component.Value]; ok {
			return nil
		}
		loader.visitedExample[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid example: value MUST be an object")
	}
	if ref := component.Ref; ref != "" {
		if isSingleRefElement(ref) {
			var example Example
			if _, err = loader.loadSingleElementFromURI(ref, documentPath, &example); err != nil {
				return err
			}
			component.Value = &example
		} else {
			var resolved ExampleRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveExampleRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			_ = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	return nil
}

func (loader *Loader) resolveCallbackRef(doc *T, component *CallbackRef, documentPath *url.URL) (err error) {

	if component == nil {
		return errors.New("invalid callback: value MUST be an object")
	}
	if ref := component.Ref; ref != "" {
		if isSingleRefElement(ref) {
			var resolved Callback
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &resolved); err != nil {
				return err
			}
			component.Value = &resolved
		} else {
			var resolved CallbackRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveCallbackRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			documentPath = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	value := component.Value
	if value == nil {
		return nil
	}

	for entrypoint, pathItem := range *value {
		entrypoint, pathItem := entrypoint, pathItem
		err = func() (err error) {
			key := "-"
			if documentPath != nil {
				key = documentPath.EscapedPath()
			}
			key += entrypoint
			if _, ok := loader.visitedPathItemRefs[key]; ok {
				return nil
			}
			loader.visitedPathItemRefs[key] = struct{}{}

			if pathItem == nil {
				return errors.New("invalid path item: value MUST be an object")
			}
			ref := pathItem.Ref
			if ref != "" {
				if isSingleRefElement(ref) {
					var p PathItem
					if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &p); err != nil {
						return err
					}
					*pathItem = p
				} else {
					if doc, ref, documentPath, err = loader.resolveRef(doc, ref, documentPath); err != nil {
						return
					}

					rest := strings.TrimPrefix(ref, "#/components/callbacks/")
					if rest == ref {
						return fmt.Errorf(`expected prefix "#/components/callbacks/" in URI %q`, ref)
					}
					id := unescapeRefString(rest)

					definitions := doc.Components.Callbacks
					if definitions == nil {
						return failedToResolveRefFragmentPart(ref, "callbacks")
					}
					resolved := definitions[id]
					if resolved == nil {
						return failedToResolveRefFragmentPart(ref, id)
					}

					for _, p := range *resolved.Value {
						*pathItem = *p
						break
					}
				}
			}
			return loader.resolvePathItemRefContinued(doc, pathItem, documentPath)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (loader *Loader) resolveLinkRef(doc *T, component *LinkRef, documentPath *url.URL) (err error) {
	if component != nil && component.Value != nil {
		if loader.visitedLink == nil {
			loader.visitedLink = make(map[*Link]struct{})
		}
		if _, ok := loader.visitedLink[component.Value]; ok {
			return nil
		}
		loader.visitedLink[component.Value] = struct{}{}
	}

	if component == nil {
		return errors.New("invalid link: value MUST be an object")
	}
	if ref := component.Ref; ref != "" {
		if isSingleRefElement(ref) {
			var link Link
			if _, err = loader.loadSingleElementFromURI(ref, documentPath, &link); err != nil {
				return err
			}
			component.Value = &link
		} else {
			var resolved LinkRef
			componentPath, err := loader.resolveComponent(doc, ref, documentPath, &resolved)
			if err != nil {
				return err
			}
			if err := loader.resolveLinkRef(doc, &resolved, componentPath); err != nil {
				return err
			}
			component.Value = resolved.Value
			_ = loader.documentPathForRecursiveRef(documentPath, resolved.Ref)
		}
	}
	return nil
}

func (loader *Loader) resolvePathItemRef(doc *T, entrypoint string, pathItem *PathItem, documentPath *url.URL) (err error) {
	key := "_"
	if documentPath != nil {
		key = documentPath.EscapedPath()
	}
	key += entrypoint
	if _, ok := loader.visitedPathItemRefs[key]; ok {
		return nil
	}
	loader.visitedPathItemRefs[key] = struct{}{}

	if pathItem == nil {
		return errors.New("invalid path item: value MUST be an object")
	}
	ref := pathItem.Ref
	if ref != "" {
		if isSingleRefElement(ref) {
			var p PathItem
			if documentPath, err = loader.loadSingleElementFromURI(ref, documentPath, &p); err != nil {
				return err
			}
			*pathItem = p
		} else {
			if doc, ref, documentPath, err = loader.resolveRef(doc, ref, documentPath); err != nil {
				return
			}

			rest := strings.TrimPrefix(ref, "#/paths/")
			if rest == ref {
				return fmt.Errorf(`expected prefix "#/paths/" in URI %q`, ref)
			}
			id := unescapeRefString(rest)

			definitions := doc.Paths
			if definitions == nil {
				return failedToResolveRefFragmentPart(ref, "paths")
			}
			resolved := definitions[id]
			if resolved == nil {
				return failedToResolveRefFragmentPart(ref, id)
			}

			*pathItem = *resolved
		}
	}
	return loader.resolvePathItemRefContinued(doc, pathItem, documentPath)
}

func (loader *Loader) resolvePathItemRefContinued(doc *T, pathItem *PathItem, documentPath *url.URL) (err error) {
	for _, parameter := range pathItem.Parameters {
		if err = loader.resolveParameterRef(doc, parameter, documentPath); err != nil {
			return
		}
	}
	for _, operation := range pathItem.Operations() {
		for _, parameter := range operation.Parameters {
			if err = loader.resolveParameterRef(doc, parameter, documentPath); err != nil {
				return
			}
		}
		if requestBody := operation.RequestBody; requestBody != nil {
			if err = loader.resolveRequestBodyRef(doc, requestBody, documentPath); err != nil {
				return
			}
		}
		for _, response := range operation.Responses {
			if err = loader.resolveResponseRef(doc, response, documentPath); err != nil {
				return
			}
		}
		for _, callback := range operation.Callbacks {
			if err = loader.resolveCallbackRef(doc, callback, documentPath); err != nil {
				return
			}
		}
	}
	return
}

func unescapeRefString(ref string) string {
	return strings.Replace(strings.Replace(ref, "~1", "/", -1), "~0", "~", -1)
}
