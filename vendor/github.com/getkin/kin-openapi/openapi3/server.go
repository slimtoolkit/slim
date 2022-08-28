package openapi3

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/jsoninfo"
)

// Servers is specified by OpenAPI/Swagger standard version 3.0.
type Servers []*Server

// Validate ensures servers are per the OpenAPIv3 specification.
func (value Servers) Validate(ctx context.Context) error {
	for _, v := range value {
		if err := v.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (servers Servers) MatchURL(parsedURL *url.URL) (*Server, []string, string) {
	rawURL := parsedURL.String()
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		rawURL = rawURL[:i]
	}
	for _, server := range servers {
		pathParams, remaining, ok := server.MatchRawURL(rawURL)
		if ok {
			return server, pathParams, remaining
		}
	}
	return nil, nil, ""
}

// Server is specified by OpenAPI/Swagger standard version 3.0.
type Server struct {
	ExtensionProps
	URL         string                     `json:"url" yaml:"url"`
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Variables   map[string]*ServerVariable `json:"variables,omitempty" yaml:"variables,omitempty"`
}

func (server *Server) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(server)
}

func (server *Server) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, server)
}

func (server Server) ParameterNames() ([]string, error) {
	pattern := server.URL
	var params []string
	for len(pattern) > 0 {
		i := strings.IndexByte(pattern, '{')
		if i < 0 {
			break
		}
		pattern = pattern[i+1:]
		i = strings.IndexByte(pattern, '}')
		if i < 0 {
			return nil, errors.New("missing '}'")
		}
		params = append(params, strings.TrimSpace(pattern[:i]))
		pattern = pattern[i+1:]
	}
	return params, nil
}

func (server Server) MatchRawURL(input string) ([]string, string, bool) {
	pattern := server.URL
	var params []string
	for len(pattern) > 0 {
		c := pattern[0]
		if len(pattern) == 1 && c == '/' {
			break
		}
		if c == '{' {
			// Find end of pattern
			i := strings.IndexByte(pattern, '}')
			if i < 0 {
				return nil, "", false
			}
			pattern = pattern[i+1:]

			// Find next matching pattern character or next '/' whichever comes first
			np := -1
			if len(pattern) > 0 {
				np = strings.IndexByte(input, pattern[0])
			}
			ns := strings.IndexByte(input, '/')

			if np < 0 {
				i = ns
			} else if ns < 0 {
				i = np
			} else {
				i = int(math.Min(float64(np), float64(ns)))
			}
			if i < 0 {
				i = len(input)
			}
			params = append(params, input[:i])
			input = input[i:]
			continue
		}
		if len(input) == 0 || input[0] != c {
			return nil, "", false
		}
		pattern = pattern[1:]
		input = input[1:]
	}
	if input == "" {
		input = "/"
	}
	if input[0] != '/' {
		return nil, "", false
	}
	return params, input, true
}

func (value *Server) Validate(ctx context.Context) (err error) {
	if value.URL == "" {
		return errors.New("value of url must be a non-empty string")
	}
	opening, closing := strings.Count(value.URL, "{"), strings.Count(value.URL, "}")
	if opening != closing {
		return errors.New("server URL has mismatched { and }")
	}
	if opening != len(value.Variables) {
		return errors.New("server has undeclared variables")
	}
	for name, v := range value.Variables {
		if !strings.Contains(value.URL, fmt.Sprintf("{%s}", name)) {
			return errors.New("server has undeclared variables")
		}
		if err = v.Validate(ctx); err != nil {
			return
		}
	}
	return
}

// ServerVariable is specified by OpenAPI/Swagger standard version 3.0.
type ServerVariable struct {
	ExtensionProps
	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}

func (serverVariable *ServerVariable) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(serverVariable)
}

func (serverVariable *ServerVariable) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, serverVariable)
}

func (value *ServerVariable) Validate(ctx context.Context) error {
	if value.Default == "" {
		data, err := value.MarshalJSON()
		if err != nil {
			return err
		}
		return fmt.Errorf("field default is required in %s", data)
	}
	return nil
}
