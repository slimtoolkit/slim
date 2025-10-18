package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

type apiSpecInfo struct {
	spec           *openapi3.T
	prefixOverride string
}

func (p *CustomProbe) loadAPISpecFiles() {
	for _, info := range p.opts.APISpecFiles {
		fileName := info
		prefixOverride := ""
		if strings.Contains(info, ":") {
			parts := strings.SplitN(info, ":", 2)
			fileName = parts[0]
			prefixOverride = parts[1]
		}

		spec, err := loadAPISpecFromFile(fileName)
		if err != nil {
			p.xc.Out.Info("http.probe.apispec.error",
				ovars{
					"message": "error loading api spec file",
					"error":   err,
				})
			continue
		}

		if spec == nil {
			p.xc.Out.Info("http.probe.apispec",
				ovars{
					"message": "unsupported spec type",
				})
			continue
		}

		info := apiSpecInfo{
			spec:           spec,
			prefixOverride: prefixOverride,
		}

		p.APISpecProbes = append(p.APISpecProbes, info)
	}
}

func parseAPISpec(rdata []byte) (*openapi3.T, error) {
	if isOpenAPI(rdata) {
		log.Debug("http.CustomProbe.parseAPISpec - is openapi")
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		spec, err := loader.LoadFromData(rdata)
		if err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.LoadFromData - error=%v", err)
			return nil, err
		}

		return spec, nil
	}

	if isSwagger(rdata) {
		log.Debug("http.CustomProbe.parseAPISpec - is swagger")
		spec2 := &openapi2.T{}
		if err := yaml.Unmarshal(rdata, spec2); err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.yaml.Unmarshal - error=%v", err)
			return nil, err
		}

		spec, err := openapi2conv.ToV3(spec2)
		if err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.ToV3 - error=%v", err)
			return nil, err
		}

		return spec, nil
	}

	log.Debugf("http.CustomProbe.parseAPISpec - unsupported api spec type (%d): %s", len(rdata), string(rdata))
	return nil, nil
}

func loadAPISpecFromEndpoint(client *http.Client, endpoint string) (*openapi3.T, error) {
	log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint(%s)", endpoint)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.http.NewRequest - error=%v", err)
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.httpClient.Do - error=%v", err)
		return nil, err
	}

	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if res.Body != nil {
		rdata, err := io.ReadAll(res.Body)
		if err != nil {
			log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.response.read - error=%v", err)
			return nil, err
		}

		return parseAPISpec(rdata)
	}

	log.Debug("http.CustomProbe.loadAPISpecFromEndpoint.response - no body")
	return nil, nil
}

func loadAPISpecFromFile(name string) (*openapi3.T, error) {
	rdata, err := os.ReadFile(name)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromFile.ReadFile - error=%v", err)
		return nil, err
	}

	return parseAPISpec(rdata)
}

func isSwagger(data []byte) bool {
	if (bytes.Contains(data, []byte(`"swagger":`)) ||
		bytes.Contains(data, []byte(`swagger:`))) &&
		bytes.Contains(data, []byte(`paths`)) {
		return true
	}

	return false
}

func isOpenAPI(data []byte) bool {
	if (bytes.Contains(data, []byte(`"openapi":`)) ||
		bytes.Contains(data, []byte(`openapi:`))) &&
		bytes.Contains(data, []byte(`paths`)) {
		return true
	}

	return false
}

func apiSpecPrefix(spec *openapi3.T) (string, error) {
	//for now get the api prefix from the first server struc
	//later, support multiple prefixes if there's more than one server struct
	var prefix string
	for _, sinfo := range spec.Servers {
		xurl := sinfo.URL
		if strings.Contains(xurl, "{") {
			for k, vinfo := range sinfo.Variables {
				varStr := fmt.Sprintf("{%s}", k)
				if strings.Contains(xurl, varStr) {
					valStr := "var"
					if vinfo.Default != "" {
						valStr = fmt.Sprintf("%v", vinfo.Default)
					} else if len(vinfo.Enum) > 0 {
						valStr = fmt.Sprintf("%v", vinfo.Enum[0])
					}

					xurl = strings.ReplaceAll(xurl, varStr, valStr)
				}
			}
		}

		if strings.Contains(xurl, "{") {
			xurl = strings.ReplaceAll(xurl, "{", "")

			if strings.Contains(xurl, "}") {
				xurl = strings.ReplaceAll(xurl, "}", "")
			}
		}

		parsed, err := url.Parse(xurl)
		if err != nil {
			return "", err
		}

		if parsed.Path != "" && parsed.Path != "/" {
			prefix = parsed.Path
		}
	}

	return prefix, nil
}

func (p *CustomProbe) loadAPISpecs(proto, targetHost, port string) {

	baseAddr := getHTTPAddr(proto, targetHost, port)
	client, err := getHTTPClient(proto)
	if err != nil {
		p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
		return
	}

	//TODO:
	//Need to support user provided target port for the spec,
	//but these need to be mapped to the actual port at runtime
	//Need to support user provided target proto for the spec
	for _, info := range p.opts.APISpecs {
		specPath := info
		prefixOverride := ""
		if strings.Contains(info, ":") {
			parts := strings.SplitN(info, ":", 2)
			specPath = parts[0]
			prefixOverride = parts[1]
		}

		addr := fmt.Sprintf("%s%s", baseAddr, specPath)
		spec, err := loadAPISpecFromEndpoint(client, addr)
		if err != nil {
			p.xc.Out.Info("http.probe.apispec.error",
				ovars{
					"message": "error loading api spec from endpoint",
					"error":   err,
				})

			continue
		}

		if spec == nil {
			p.xc.Out.Info("http.probe.apispec",
				ovars{
					"message": "unsupported spec type",
				})

			continue
		}

		info := apiSpecInfo{
			spec:           spec,
			prefixOverride: prefixOverride,
		}

		p.APISpecProbes = append(p.APISpecProbes, info)
	}
}

func pathOps(pinfo *openapi3.PathItem) map[string]*openapi3.Operation {
	ops := map[string]*openapi3.Operation{}
	addPathOp(&ops, pinfo.Connect, "connect")
	addPathOp(&ops, pinfo.Delete, "delete")
	addPathOp(&ops, pinfo.Get, "get")
	addPathOp(&ops, pinfo.Head, "head")
	addPathOp(&ops, pinfo.Options, "options")
	addPathOp(&ops, pinfo.Patch, "patch")
	addPathOp(&ops, pinfo.Post, "post")
	addPathOp(&ops, pinfo.Put, "put")
	addPathOp(&ops, pinfo.Trace, "trace")
	return ops
}

func addPathOp(m *map[string]*openapi3.Operation, op *openapi3.Operation, name string) {
	if op != nil {
		(*m)[name] = op
	}
}

func (p *CustomProbe) probeAPISpecEndpoints(proto, targetHost, port, prefix string, spec *openapi3.T) {
	addr := getHTTPAddr(proto, targetHost, port)

	if p.printState {
		p.xc.Out.State("http.probe.api-spec.probe.endpoint.starting",
			ovars{
				"addr":      addr,
				"prefix":    prefix,
				"endpoints": spec.Paths.Len(),
			})
	}

	httpClient, err := getHTTPClient(proto)
	if err != nil {
		p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
		return
	}

	for rawAPIPath, pathInfo := range spec.Paths.Map() {
		ops := pathOps(pathInfo)
		for apiMethod, op := range ops {
			params := collectParameters(pathInfo, op)
			finalPath := substitutePathParams(rawAPIPath, params)
			queryString, headers := buildQueryAndHeaders(params)
			endpoint := fmt.Sprintf("%s%s%s", addr, prefix, finalPath)
			if queryString != "" {
				if strings.Contains(endpoint, "?") {
					endpoint = endpoint + "&" + queryString
				} else {
					endpoint = endpoint + "?" + queryString
				}
			}

			var bodyReader io.Reader
			var contentType string
			if br, ct := dummyRequestBody(op); br != nil {
				bodyReader = br
				contentType = ct
				if headers == nil {
					headers = map[string]string{}
				}
				if contentType != "" {
					headers["Content-Type"] = contentType
				}
			}

			// Only send body for methods that typically support it
			methodUpper := strings.ToUpper(apiMethod)
			if !(methodUpper == http.MethodPost || methodUpper == http.MethodPut || methodUpper == http.MethodPatch || methodUpper == http.MethodDelete) {
				bodyReader = nil
			}

			p.apiSpecEndpointCall(httpClient, endpoint, apiMethod, bodyReader, headers)
		}
	}
}

func (p *CustomProbe) apiSpecEndpointCall(client *http.Client, endpoint, method string, body io.Reader, headers map[string]string) {
	maxRetryCount := probeRetryCount
	if p.opts.RetryCount > 0 {
		maxRetryCount = p.opts.RetryCount
	}

	notReadyErrorWait := time.Duration(16)
	webErrorWait := time.Duration(8)
	otherErrorWait := time.Duration(4)
	if p.opts.RetryWait > 0 {
		webErrorWait = time.Duration(p.opts.RetryWait)
		notReadyErrorWait = time.Duration(p.opts.RetryWait * 2)
		otherErrorWait = time.Duration(p.opts.RetryWait / 2)
	}

	method = strings.ToUpper(method)
	for i := 0; i < maxRetryCount; i++ {
		req, err := http.NewRequest(method, endpoint, body)
		if err != nil {
			p.xc.Out.Error("HTTP probe - construct request error - %v", err.Error())
			// Break since the same args are passed to NewRequest() on each loop.
			break
		}
		for hname, hvalue := range headers {
			req.Header.Add(hname, hvalue)
		}
		//no credentials for now
		res, err := client.Do(req)
		p.CallCount++

		if res != nil {
			if res.Body != nil {
				io.Copy(io.Discard, res.Body)
			}

			defer res.Body.Close()
		}

		statusCode := "error"
		callErrorStr := "none"
		if err == nil {
			statusCode = fmt.Sprintf("%v", res.StatusCode)
		} else {
			callErrorStr = err.Error()
		}

		if p.printState {
			p.xc.Out.Info("http.probe.api-spec.probe.endpoint.call",
				ovars{
					"status":   statusCode,
					"method":   method,
					"endpoint": endpoint,
					"attempt":  i + 1,
					"error":    callErrorStr,
					"time":     time.Now().UTC().Format(time.RFC3339),
				})
		}

		if err == nil {
			p.OkCount++
			break
		} else {
			p.ErrCount++

			if urlErr, ok := err.(*url.Error); ok {
				if urlErr.Err == io.EOF {
					log.Debugf("HTTP probe - target not ready yet (retry again later)...")
					time.Sleep(notReadyErrorWait * time.Second)
				} else {
					log.Debugf("HTTP probe - web error... retry again later...")
					time.Sleep(webErrorWait * time.Second)

				}
			} else {
				log.Debugf("HTTP probe - other error... retry again later...")
				time.Sleep(otherErrorWait * time.Second)
			}
		}

	}
}

// collectParameters merges path-level and operation-level parameters,
// operation-level parameters take precedence on conflicts (same in+name).
func collectParameters(pathItem *openapi3.PathItem, op *openapi3.Operation) []*openapi3.Parameter {
	var result []*openapi3.Parameter
	// index to handle overrides
	key := func(p *openapi3.Parameter) string { return p.In + "\x00" + p.Name }
	seen := map[string]bool{}

	if pathItem != nil {
		for _, pref := range pathItem.Parameters {
			if pref == nil || pref.Value == nil {
				continue
			}
			p := pref.Value
			result = append(result, p)
			seen[key(p)] = true
		}
	}

	if op != nil {
		for _, pref := range op.Parameters {
			if pref == nil || pref.Value == nil {
				continue
			}
			p := pref.Value
			k := key(p)
			if seen[k] {
				// override by replacing prior entry
				for i := range result {
					if key(result[i]) == k {
						result[i] = p
						// OpenAPI params are unique per operation by (in,name). An op-level param
						// overrides at most one path-level entry, so replace once and stop.
						break
					}
				}
			} else {
				result = append(result, p)
				seen[k] = true
			}
		}
	}

	return result
}

func substitutePathParams(apiPath string, params []*openapi3.Parameter) string {
	if !strings.Contains(apiPath, "{") {
		return apiPath
	}

	for _, p := range params {
		if p == nil || p.In != "path" {
			continue
		}
		placeholder := "{" + p.Name + "}"
		if strings.Contains(apiPath, placeholder) {
			apiPath = strings.ReplaceAll(apiPath, placeholder, url.PathEscape(paramStringForSchema(p.Schema)))
		}
	}

	// fallback: strip any remaining braces
	if strings.Contains(apiPath, "{") {
		apiPath = strings.ReplaceAll(apiPath, "{", "")
		apiPath = strings.ReplaceAll(apiPath, "}", "")
	}

	return apiPath
}

func buildQueryAndHeaders(params []*openapi3.Parameter) (string, map[string]string) {
	var parts []string
	headers := make(map[string]string)
	for _, p := range params {
		if p == nil {
			continue
		}
		// derive a string value for param; for object schemas, JSON-stringify a sample
		getStringValue := func(pref *openapi3.SchemaRef) string {
			if pref != nil && pref.Value != nil && pref.Value.Type != nil && pref.Value.Type.Is(openapi3.TypeObject) {
				sample := jsonSampleForSchema(pref)
				if data, err := json.Marshal(sample); err == nil {
					return string(data)
				}
				return "{}"
			}
			return paramStringForSchema(pref)
		}
		switch p.In {
		case "query":
			v := getStringValue(p.Schema)
			parts = append(parts, url.QueryEscape(p.Name)+"="+url.QueryEscape(v))
		case "header":
			v := getStringValue(p.Schema)
			headers[p.Name] = v
		}
	}
	return strings.Join(parts, "&"), headers
}

func dummyRequestBody(op *openapi3.Operation) (io.Reader, string) {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil, ""
	}
	rb := op.RequestBody.Value
	if len(rb.Content) == 0 {
		return nil, ""
	}

	// prefer application/json
	var ct string
	if _, ok := rb.Content["application/json"]; ok {
		ct = "application/json"
	} else {
		for k := range rb.Content {
			ct = k
			break
		}
	}

	mt := rb.Content[ct]
	if mt == nil || mt.Schema == nil || mt.Schema.Value == nil {
		return nil, ""
	}

	// For JSON, build a sample payload from the schema
	if strings.Contains(ct, "json") {
		sample := jsonSampleForSchema(mt.Schema)
		data, err := json.Marshal(sample)
		if err != nil {
			return nil, ""
		}
		return bytes.NewReader(data), ct
	}

	return nil, ""
}

func paramStringForSchema(sref *openapi3.SchemaRef) string {
	if sref == nil || sref.Value == nil {
		return "x"
	}
	s := sref.Value

	if len(s.Enum) > 0 {
		if v, ok := s.Enum[0].(string); ok {
			return v
		}
		return "1"
	}

	if s.Type != nil && s.Type.Is(openapi3.TypeInteger) {
		return "1"
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeNumber) {
		return "1"
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeBoolean) {
		return "true"
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeArray) {
		// represent as a single element list in query/header contexts
		return paramStringForSchema(s.Items)
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeObject) {
		return "x"
	}
	return "x"
}

func jsonSampleForSchema(sref *openapi3.SchemaRef) interface{} {
	if sref == nil || sref.Value == nil {
		return map[string]interface{}{}
	}

	s := sref.Value

	if len(s.Enum) > 0 {
		return s.Enum[0]
	}

	if s.Type != nil && s.Type.Is(openapi3.TypeInteger) {
		return 1
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeNumber) {
		return 1
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeBoolean) {
		return true
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeArray) {
		return []interface{}{jsonSampleForSchema(s.Items)}
	}
	if s.Type != nil && s.Type.Is(openapi3.TypeObject) {
		obj := map[string]interface{}{}
		if len(s.Properties) > 0 {
			for pname, pref := range s.Properties {
				obj[pname] = jsonSampleForSchema(pref)
			}
		}
		return obj
	}
	return "string"
}
