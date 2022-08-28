package openapi3

import (
	"context"
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/go-openapi/jsonpointer"
)

type SecuritySchemes map[string]*SecuritySchemeRef

func (s SecuritySchemes) JSONLookup(token string) (interface{}, error) {
	ref, ok := s[token]
	if ref == nil || ok == false {
		return nil, fmt.Errorf("object has no field %q", token)
	}

	if ref.Ref != "" {
		return &Ref{Ref: ref.Ref}, nil
	}
	return ref.Value, nil
}

var _ jsonpointer.JSONPointable = (*SecuritySchemes)(nil)

type SecurityScheme struct {
	ExtensionProps

	Type             string      `json:"type,omitempty" yaml:"type,omitempty"`
	Description      string      `json:"description,omitempty" yaml:"description,omitempty"`
	Name             string      `json:"name,omitempty" yaml:"name,omitempty"`
	In               string      `json:"in,omitempty" yaml:"in,omitempty"`
	Scheme           string      `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	BearerFormat     string      `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
	Flows            *OAuthFlows `json:"flows,omitempty" yaml:"flows,omitempty"`
	OpenIdConnectUrl string      `json:"openIdConnectUrl,omitempty" yaml:"openIdConnectUrl,omitempty"`
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

func NewOIDCSecurityScheme(oidcUrl string) *SecurityScheme {
	return &SecurityScheme{
		Type:             "openIdConnect",
		OpenIdConnectUrl: oidcUrl,
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

func (value *SecurityScheme) Validate(ctx context.Context) error {
	hasIn := false
	hasBearerFormat := false
	hasFlow := false
	switch value.Type {
	case "apiKey":
		hasIn = true
	case "http":
		scheme := value.Scheme
		switch scheme {
		case "bearer":
			hasBearerFormat = true
		case "basic", "negotiate", "digest":
		default:
			return fmt.Errorf("security scheme of type 'http' has invalid 'scheme' value %q", scheme)
		}
	case "oauth2":
		hasFlow = true
	case "openIdConnect":
		if value.OpenIdConnectUrl == "" {
			return fmt.Errorf("no OIDC URL found for openIdConnect security scheme %q", value.Name)
		}
	default:
		return fmt.Errorf("security scheme 'type' can't be %q", value.Type)
	}

	// Validate "in" and "name"
	if hasIn {
		switch value.In {
		case "query", "header", "cookie":
		default:
			return fmt.Errorf("security scheme of type 'apiKey' should have 'in'. It can be 'query', 'header' or 'cookie', not %q", value.In)
		}
		if value.Name == "" {
			return errors.New("security scheme of type 'apiKey' should have 'name'")
		}
	} else if len(value.In) > 0 {
		return fmt.Errorf("security scheme of type %q can't have 'in'", value.Type)
	} else if len(value.Name) > 0 {
		return errors.New("security scheme of type 'apiKey' can't have 'name'")
	}

	// Validate "format"
	// "bearerFormat" is an arbitrary string so we only check if the scheme supports it
	if !hasBearerFormat && len(value.BearerFormat) > 0 {
		return fmt.Errorf("security scheme of type %q can't have 'bearerFormat'", value.Type)
	}

	// Validate "flow"
	if hasFlow {
		flow := value.Flows
		if flow == nil {
			return fmt.Errorf("security scheme of type %q should have 'flows'", value.Type)
		}
		if err := flow.Validate(ctx); err != nil {
			return fmt.Errorf("security scheme 'flow' is invalid: %v", err)
		}
	} else if value.Flows != nil {
		return fmt.Errorf("security scheme of type %q can't have 'flows'", value.Type)
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

func (flows *OAuthFlows) Validate(ctx context.Context) error {
	if v := flows.Implicit; v != nil {
		return v.Validate(ctx, oAuthFlowTypeImplicit)
	}
	if v := flows.Password; v != nil {
		return v.Validate(ctx, oAuthFlowTypePassword)
	}
	if v := flows.ClientCredentials; v != nil {
		return v.Validate(ctx, oAuthFlowTypeClientCredentials)
	}
	if v := flows.AuthorizationCode; v != nil {
		return v.Validate(ctx, oAuthFlowAuthorizationCode)
	}
	return errors.New("no OAuth flow is defined")
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

func (flow *OAuthFlow) Validate(ctx context.Context, typ oAuthFlowType) error {
	if typ == oAuthFlowAuthorizationCode || typ == oAuthFlowTypeImplicit {
		if v := flow.AuthorizationURL; v == "" {
			return errors.New("an OAuth flow is missing 'authorizationUrl in authorizationCode or implicit '")
		}
	}
	if typ != oAuthFlowTypeImplicit {
		if v := flow.TokenURL; v == "" {
			return errors.New("an OAuth flow is missing 'tokenUrl in not implicit'")
		}
	}
	if v := flow.Scopes; v == nil {
		return errors.New("an OAuth flow is missing 'scopes'")
	}
	return nil
}
