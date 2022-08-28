package openapi3

import (
	"context"
	"fmt"
	"strings"
)

// Paths is specified by OpenAPI/Swagger standard version 3.0.
type Paths map[string]*PathItem

func (value Paths) Validate(ctx context.Context) error {
	normalizedPaths := make(map[string]string)
	for path, pathItem := range value {
		if path == "" || path[0] != '/' {
			return fmt.Errorf("path %q does not start with a forward slash (/)", path)
		}

		if pathItem == nil {
			value[path] = &PathItem{}
			pathItem = value[path]
		}

		normalizedPath, _, varsInPath := normalizeTemplatedPath(path)
		if oldPath, ok := normalizedPaths[normalizedPath]; ok {
			return fmt.Errorf("conflicting paths %q and %q", path, oldPath)
		}
		normalizedPaths[path] = path

		var commonParams []string
		for _, parameterRef := range pathItem.Parameters {
			if parameterRef != nil {
				if parameter := parameterRef.Value; parameter != nil && parameter.In == ParameterInPath {
					commonParams = append(commonParams, parameter.Name)
				}
			}
		}
		for method, operation := range pathItem.Operations() {
			var setParams []string
			for _, parameterRef := range operation.Parameters {
				if parameterRef != nil {
					if parameter := parameterRef.Value; parameter != nil && parameter.In == ParameterInPath {
						setParams = append(setParams, parameter.Name)
					}
				}
			}
			if expected := len(setParams) + len(commonParams); expected != len(varsInPath) {
				expected -= len(varsInPath)
				if expected < 0 {
					expected *= -1
				}
				missing := make(map[string]struct{}, expected)
				definedParams := append(setParams, commonParams...)
				for _, name := range definedParams {
					if _, ok := varsInPath[name]; !ok {
						missing[name] = struct{}{}
					}
				}
				for name := range varsInPath {
					got := false
					for _, othername := range definedParams {
						if othername == name {
							got = true
							break
						}
					}
					if !got {
						missing[name] = struct{}{}
					}
				}
				if len(missing) != 0 {
					missings := make([]string, 0, len(missing))
					for name := range missing {
						missings = append(missings, name)
					}
					return fmt.Errorf("operation %s %s must define exactly all path parameters (missing: %v)", method, path, missings)
				}
			}
		}

		if err := pathItem.Validate(ctx); err != nil {
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

	normalizedPath, expected, _ := normalizeTemplatedPath(key)
	for path, pathItem := range paths {
		pathNormalized, got, _ := normalizeTemplatedPath(path)
		if got == expected && pathNormalized == normalizedPath {
			return pathItem
		}
	}
	return nil
}

func normalizeTemplatedPath(path string) (string, uint, map[string]struct{}) {
	if strings.IndexByte(path, '{') < 0 {
		return path, 0, nil
	}

	var buffTpl strings.Builder
	buffTpl.Grow(len(path))

	var (
		cc         rune
		count      uint
		isVariable bool
		vars       = make(map[string]struct{})
		buffVar    strings.Builder
	)
	for i, c := range path {
		if isVariable {
			if c == '}' {
				// End path variable
				isVariable = false

				vars[buffVar.String()] = struct{}{}
				buffVar = strings.Builder{}

				// First append possible '*' before this character
				// The character '}' will be appended
				if i > 0 && cc == '*' {
					buffTpl.WriteRune(cc)
				}
			} else {
				buffVar.WriteRune(c)
				continue
			}

		} else if c == '{' {
			// Begin path variable
			isVariable = true

			// The character '{' will be appended
			count++
		}

		// Append the character
		buffTpl.WriteRune(c)
		cc = c
	}
	return buffTpl.String(), count, vars
}
