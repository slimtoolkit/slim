package openapi3

import (
	"context"
	"errors"
	"math"
	"net/url"
	"strings"
)

// Servers is specified by OpenAPI/Swagger standard version 3.0.
type Servers []*Server

func (servers Servers) Validate(c context.Context) error {
	for _, v := range servers {
		if err := v.Validate(c); err != nil {
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
	URL         string                     `json:"url" yaml:"url"`
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Variables   map[string]*ServerVariable `json:"variables,omitempty" yaml:"variables,omitempty"`
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
			return nil, errors.New("Missing '}'")
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

func (server *Server) Validate(c context.Context) (err error) {
	if server.URL == "" {
		return errors.New("value of url must be a non-empty JSON string")
	}
	for _, v := range server.Variables {
		if err = v.Validate(c); err != nil {
			return
		}
	}
	return
}

// ServerVariable is specified by OpenAPI/Swagger standard version 3.0.
type ServerVariable struct {
	Enum        []interface{} `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     interface{}   `json:"default,omitempty" yaml:"default,omitempty"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
}

func (serverVariable *ServerVariable) Validate(c context.Context) error {
	switch serverVariable.Default.(type) {
	case float64, string:
	default:
		return errors.New("value of default must be either JSON number or JSON string")
	}
	for _, item := range serverVariable.Enum {
		switch item.(type) {
		case float64, string:
		default:
			return errors.New("Every variable 'enum' item must be number of string")
		}
	}
	return nil
}
