package http

import (
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSubstitutePathParams(t *testing.T) {
	apiPath := "/pets/{id}/owners/{ownerId}"

	params := []*openapi3.Parameter{
		{
			Name: "id",
			In:   "path",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{openapi3.TypeInteger},
			}},
		},
		{
			Name: "ownerId",
			In:   "path",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{openapi3.TypeString},
			}},
		},
	}

	got := substitutePathParams(apiPath, params)
	want := "/pets/1/owners/x"
	if got != want {
		t.Fatalf("substitutePathParams() = %q; want %q", got, want)
	}
}

func TestBuildQueryAndHeaders(t *testing.T) {
	params := []*openapi3.Parameter{
		{
			Name: "q",
			In:   "query",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{openapi3.TypeString},
			}},
		},
		{
			Name: "count",
			In:   "query",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{openapi3.TypeInteger},
			}},
		},
		{
			Name: "X-Token",
			In:   "header",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{openapi3.TypeString},
			}},
		},
		{
			Name: "meta",
			In:   "query",
			Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{openapi3.TypeObject},
				Properties: openapi3.Schemas{
					"x": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeString}}},
				},
			}},
		},
	}

	qs, headers := buildQueryAndHeaders(params)
	values, err := url.ParseQuery(qs)
	if err != nil {
		t.Fatalf("failed to parse query string: %v", err)
	}
	if values.Get("q") != "x" {
		t.Fatalf("query param q = %q; want %q", values.Get("q"), "x")
	}
	if values.Get("count") != "1" {
		t.Fatalf("query param count = %q; want %q", values.Get("count"), "1")
	}
	if headers["X-Token"] != "x" {
		t.Fatalf("header X-Token = %q; want %q", headers["X-Token"], "x")
	}

	// object param should be JSON-stringified
	if values.Get("meta") == "" {
		t.Fatalf("expected meta query param to be present")
	}
	if !strings.HasPrefix(values.Get("meta"), "{") {
		t.Fatalf("expected meta to be JSON string, got: %q", values.Get("meta"))
	}
}

func TestDummyRequestBody_JSON(t *testing.T) {
	op := &openapi3.Operation{
		RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
						Type: &openapi3.Types{openapi3.TypeObject},
						Properties: openapi3.Schemas{
							"name": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeString}}},
							"age":  &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeInteger}}},
						},
					}},
				},
			},
		}},
	}

	body, ct := dummyRequestBody(op)
	if ct != "application/json" {
		t.Fatalf("content type = %q; want application/json", ct)
	}
	if body == nil {
		t.Fatalf("expected non-nil body")
	}

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal body error: %v; raw=%s", err, strings.TrimSpace(string(data)))
	}
	if _, ok := obj["name"]; !ok {
		t.Fatalf("missing 'name' in body: %#v", obj)
	}
	if _, ok := obj["age"]; !ok {
		t.Fatalf("missing 'age' in body: %#v", obj)
	}
}
