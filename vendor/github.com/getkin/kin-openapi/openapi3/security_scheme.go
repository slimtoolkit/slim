package openapi3

import (
	"context"
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
)

type SecurityScheme struct {
	ExtensionProps

	Type         string      `json:"type,omitempty" yaml:"type,omitempty"`
	Description  string      `json:"description,omitempty" yaml:"description,omitempty"`
	Name         string      `json:"name,omitempty" yaml:"name,omitempty"`
	In           string      `json:"in,omitempty" yaml:"in,omitempty"`
	Scheme       string      `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	BearerFormat string      `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
	Flows        *OAuthFlows `json:"flows,omitempty" yaml:"flows,omitempty"`
}

func NewSecurityScheme() *SecurityScheme {
	return &SecurityScheme{}
}

func NewCSRFSecurityScheme() *SecurityScheme {
	return &SecurityScheme{
		Type: "apiKey",
		In:   "header",
		Name: "X-XSRF-TOKEN",
	}
}

func NewJWTSecurityScheme() *SecurityScheme {
	return &SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
	}
}

func (ss *SecurityScheme) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(ss)
}

func (ss *SecurityScheme) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, ss)
}

func (ss *SecurityScheme) WithType(value string) *SecurityScheme {
	ss.Type = value
	return ss
}

func (ss *SecurityScheme) WithDescription(value string) *SecurityScheme {
	ss.Description = value
	return ss
}

func (ss *SecurityScheme) WithName(value string) *SecurityScheme {
	ss.Name = value
	return ss
}

func (ss *SecurityScheme) WithIn(value string) *SecurityScheme {
	ss.In = value
	return ss
}

func (ss *SecurityScheme) WithScheme(value string) *SecurityScheme {
	ss.Scheme = value
	return ss
}

func (ss *SecurityScheme) WithBearerFormat(value string) *SecurityScheme {
	ss.BearerFormat = value
	return ss
}

func (ss *SecurityScheme) Validate(c context.Context) error {
	hasIn := false
	hasBearerFormat := false
	hasFlow := false
	switch ss.Type {
	case "apiKey":
		hasIn = true
	case "http":
		scheme := ss.Scheme
		switch scheme {
		case "bearer":
			hasBearerFormat = true
		case "basic":
		default:
			return fmt.Errorf("Security scheme of type 'http' has invalid 'scheme' value '%s'", scheme)
		}
	case "oauth2":
		hasFlow = true
	case "openIdConnect":
		return fmt.Errorf("Support for security schemes with type '%v' has not been implemented", ss.Type)
	default:
		return fmt.Errorf("Security scheme 'type' can't be '%v'", ss.Type)
	}

	// Validate "in" and "name"
	if hasIn {
		switch ss.In {
		case "query", "header", "cookie":
		default:
			return fmt.Errorf("Security scheme of type 'apiKey' should have 'in'. It can be 'query', 'header' or 'cookie', not '%s'", ss.In)
		}
		if ss.Name == "" {
			return errors.New("Security scheme of type 'apiKey' should have 'name'")
		}
	} else if len(ss.In) > 0 {
		return fmt.Errorf("Security scheme of type '%s' can't have 'in'", ss.Type)
	} else if len(ss.Name) > 0 {
		return errors.New("Security scheme of type 'apiKey' can't have 'name'")
	}

	// Validate "format"
	// "bearerFormat" is an arbitrary string so we only check if the scheme supports it
	if !hasBearerFormat && len(ss.BearerFormat) > 0 {
		return fmt.Errorf("Security scheme of type '%v' can't have 'bearerFormat'", ss.Type)
	}

	// Validate "flow"
	if hasFlow {
		flow := ss.Flows
		if flow == nil {
			return fmt.Errorf("Security scheme of type '%v' should have 'flows'", ss.Type)
		}
		if err := flow.Validate(c); err != nil {
			return fmt.Errorf("Security scheme 'flow' is invalid: %v", err)
		}
	} else if ss.Flows != nil {
		return fmt.Errorf("Security scheme of type '%s' can't have 'flows'", ss.Type)
	}
	return nil
}

type OAuthFlows struct {
	ExtensionProps
	Implicit          *OAuthFlow `json:"implicit,omitempty" yaml:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty" yaml:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty" yaml:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty" yaml:"authorizationCode,omitempty"`
}

type oAuthFlowType int

const (
	oAuthFlowTypeImplicit oAuthFlowType = iota
	oAuthFlowTypePassword
	oAuthFlowTypeClientCredentials
	oAuthFlowAuthorizationCode
)

func (flows *OAuthFlows) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(flows)
}

func (flows *OAuthFlows) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, flows)
}

func (flows *OAuthFlows) Validate(c context.Context) error {
	if v := flows.Implicit; v != nil {
		return v.Validate(c, oAuthFlowTypeImplicit)
	}
	if v := flows.Password; v != nil {
		return v.Validate(c, oAuthFlowTypePassword)
	}
	if v := flows.ClientCredentials; v != nil {
		return v.Validate(c, oAuthFlowTypeClientCredentials)
	}
	if v := flows.AuthorizationCode; v != nil {
		return v.Validate(c, oAuthFlowAuthorizationCode)
	}
	return errors.New("No OAuth flow is defined")
}

type OAuthFlow struct {
	ExtensionProps
	AuthorizationURL string            `json:"authorizationUrl,omitempty" yaml:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty" yaml:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes" yaml:"scopes"`
}

func (flow *OAuthFlow) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(flow)
}

func (flow *OAuthFlow) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, flow)
}

func (flow *OAuthFlow) Validate(c context.Context, typ oAuthFlowType) error {
	if typ == oAuthFlowAuthorizationCode || typ == oAuthFlowTypeImplicit {
		if v := flow.AuthorizationURL; v == "" {
			return errors.New("An OAuth flow is missing 'authorizationUrl in authorizationCode or implicit '")
		}
	}
	if typ != oAuthFlowTypeImplicit {
		if v := flow.TokenURL; v == "" {
			return errors.New("An OAuth flow is missing 'tokenUrl in not implicit'")
		}
	}
	if v := flow.Scopes; v == nil {
		return errors.New("An OAuth flow is missing 'scopes'")
	}
	return nil
}
