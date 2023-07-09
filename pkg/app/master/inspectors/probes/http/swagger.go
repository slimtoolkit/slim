package http

import (
	"bytes"
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
				"endpoints": len(spec.Paths),
			})
	}

	httpClient, err := getHTTPClient(proto)
	if err != nil {
		p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
		return
	}

	for apiPath, pathInfo := range spec.Paths {
		//very primitive way to set the path params (will break for numeric values)
		if strings.Contains(apiPath, "{") {
			apiPath = strings.ReplaceAll(apiPath, "{", "")

			if strings.Contains(apiPath, "}") {
				apiPath = strings.ReplaceAll(apiPath, "}", "")
			}
		}

		endpoint := fmt.Sprintf("%s%s%s", addr, prefix, apiPath)
		ops := pathOps(pathInfo)
		for apiMethod := range ops {
			//make a call (no params for now)
			p.apiSpecEndpointCall(httpClient, endpoint, apiMethod)
		}
	}
}

func (p *CustomProbe) apiSpecEndpointCall(client *http.Client, endpoint, method string) {
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
		req, err := http.NewRequest(method, endpoint, nil)
		if err != nil {
			p.xc.Out.Error("HTTP probe - construct request error - %v", err.Error())
			// Break since the same args are passed to NewRequest() on each loop.
			break
		}
		//no body, no request headers and no credentials for now
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
