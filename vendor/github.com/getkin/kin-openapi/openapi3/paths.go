package openapi3

import (
	"context"
	"fmt"
	"strings"
)

// Paths is specified by OpenAPI/Swagger standard version 3.0.
type Paths map[string]*PathItem

func (paths Paths) Validate(c context.Context) error {
	normalizedPaths := make(map[string]string)
	for path, pathItem := range paths {
		if path == "" || path[0] != '/' {
			return fmt.Errorf("path %q does not start with a forward slash (/)", path)
		}

		normalizedPath, pathParamsCount := normalizeTemplatedPath(path)
		if oldPath, ok := normalizedPaths[normalizedPath]; ok {
			return fmt.Errorf("conflicting paths %q and %q", path, oldPath)
		}
		normalizedPaths[path] = path

		var globalCount uint
		for _, parameterRef := range pathItem.Parameters {
			if parameterRef != nil {
				if parameter := parameterRef.Value; parameter != nil && parameter.In == ParameterInPath {
					globalCount++
				}
			}
		}
		for method, operation := range pathItem.Operations() {
			var count uint
			for _, parameterRef := range operation.Parameters {
				if parameterRef != nil {
					if parameter := parameterRef.Value; parameter != nil && parameter.In == ParameterInPath {
						count++
					}
				}
			}
			if count+globalCount != pathParamsCount {
				return fmt.Errorf("operation %s %s must define exactly all path parameters", method, path)
			}
		}

		if err := pathItem.Validate(c); err != nil {
			return err
		}
	}
	return nil
}

// Find returns a path that matches the key.
//
// The method ignores differences in template variable names (except possible "*" suffix).
//
// For example:
//
//   paths := openapi3.Paths {
//     "/person/{personName}": &openapi3.PathItem{},
//   }
//   pathItem := path.Find("/person/{name}")
//
// would return the correct path item.
func (paths Paths) Find(key string) *PathItem {
	// Try directly access the map
	pathItem := paths[key]
	if pathItem != nil {
		return pathItem
	}

	normalizedPath, expected := normalizeTemplatedPath(key)
	for path, pathItem := range paths {
		pathNormalized, got := normalizeTemplatedPath(path)
		if got == expected && pathNormalized == normalizedPath {
			return pathItem
		}
	}
	return nil
}

func normalizeTemplatedPath(path string) (string, uint) {
	if strings.IndexByte(path, '{') < 0 {
		return path, 0
	}

	var buf strings.Builder
	buf.Grow(len(path))

	var (
		cc         rune
		count      uint
		isVariable bool
	)
	for i, c := range path {
		if isVariable {
			if c == '}' {
				// End path variables
				// First append possible '*' before this character
				// The character '}' will be appended
				if i > 0 && cc == '*' {
					buf.WriteRune(cc)
				}
				isVariable = false
			} else {
				// Skip this character
				continue
			}
		} else if c == '{' {
			// Begin path variable
			// The character '{' will be appended
			isVariable = true
			count++
		}

		// Append the character
		buf.WriteRune(c)
		cc = c
	}
	return buf.String(), count
}
