package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	"github.com/gocolly/colly/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container"
)

const (
	probeRetryCount = 5
	httpPortStr     = "80"
	httpsPortStr    = "443"
)

// CustomProbe is a custom HTTP probe
type CustomProbe struct {
	PrintState            bool
	PrintPrefix           string
	Ports                 []string
	Cmds                  []config.HTTPProbeCmd
	RetryCount            int
	RetryWait             int
	TargetPorts           []uint16
	ProbeFull             bool
	ProbeExitOnFailure    bool
	APISpecs              []string
	APISpecFiles          []string
	APISpecProbes         []apiSpecInfo
	ContainerInspector    *container.Inspector
	CallCount             uint64
	ErrCount              uint64
	OkCount               uint64
	doneChan              chan struct{}
	workers               sync.WaitGroup
	crawlMaxDepth         int
	crawlMaxPageCount     int
	crawlConcurrency      int
	maxConcurrentCrawlers int
	concurrentCrawlers    chan struct{}
}

type apiSpecInfo struct {
	spec           *openapi3.Swagger
	prefixOverride string
}

const (
	defaultCrawlMaxDepth         = 3
	defaultCrawlMaxPageCount     = 1000
	defaultCrawlConcurrency      = 10
	defaultMaxConcurrentCrawlers = 1
)

// NewCustomProbe creates a new custom HTTP probe
func NewCustomProbe(inspector *container.Inspector,
	cmds []config.HTTPProbeCmd,
	retryCount int,
	retryWait int,
	targetPorts []uint16,
	crawlMaxDepth int,
	crawlMaxPageCount int,
	crawlConcurrency int,
	maxConcurrentCrawlers int,
	probeFull bool,
	probeExitOnFailure bool,
	apiSpecs []string,
	apiSpecFiles []string,
	printState bool,
	printPrefix string) (*CustomProbe, error) {
	//note: the default probe should already be there if the user asked for it

	//-1 means disabled
	if crawlMaxDepth == 0 {
		crawlMaxDepth = defaultCrawlMaxDepth
	}

	//-1 means disabled
	if crawlMaxPageCount == 0 {
		crawlMaxPageCount = defaultCrawlMaxPageCount
	}

	//-1 means disabled
	if crawlConcurrency == 0 {
		crawlConcurrency = defaultCrawlConcurrency
	}

	//-1 means disabled
	if maxConcurrentCrawlers == 0 {
		maxConcurrentCrawlers = defaultMaxConcurrentCrawlers
	}

	probe := &CustomProbe{
		PrintState:            printState,
		PrintPrefix:           printPrefix,
		Cmds:                  cmds,
		RetryCount:            retryCount,
		RetryWait:             retryWait,
		TargetPorts:           targetPorts,
		ProbeFull:             probeFull,
		ProbeExitOnFailure:    probeExitOnFailure,
		APISpecs:              apiSpecs,
		APISpecFiles:          apiSpecFiles,
		ContainerInspector:    inspector,
		crawlMaxDepth:         crawlMaxDepth,
		crawlMaxPageCount:     crawlMaxPageCount,
		crawlConcurrency:      crawlConcurrency,
		maxConcurrentCrawlers: maxConcurrentCrawlers,
		doneChan:              make(chan struct{}),
	}

	if probe.maxConcurrentCrawlers > 0 {
		probe.concurrentCrawlers = make(chan struct{}, probe.maxConcurrentCrawlers)
	}

	availableHostPorts := map[string]string{}

	for nsPortKey, nsPortData := range inspector.ContainerInfo.NetworkSettings.Ports {
		//skip IPC ports
		if (nsPortKey == inspector.CmdPort) || (nsPortKey == inspector.EvtPort) {
			continue
		}

		parts := strings.Split(string(nsPortKey), "/")
		if len(parts) == 2 && parts[1] != "tcp" {
			log.Debugf("HTTP probe - skipping non-tcp port => %v", nsPortKey)
			continue
		}

		availableHostPorts[nsPortData[0].HostPort] = parts[0]
	}

	log.Debugf("HTTP probe - available host ports => %+v", availableHostPorts)

	if len(probe.TargetPorts) > 0 {
		for _, pnum := range probe.TargetPorts {
			pspec := dockerapi.Port(fmt.Sprintf("%v/tcp", pnum))
			if _, ok := inspector.ContainerInfo.NetworkSettings.Ports[pspec]; ok {
				if inspector.InContainer {
					probe.Ports = append(probe.Ports, fmt.Sprintf("%d", pnum))
				} else {
					probe.Ports = append(probe.Ports, inspector.ContainerInfo.NetworkSettings.Ports[pspec][0].HostPort)
				}
			} else {
				log.Debugf("HTTP probe - ignoring port => %v", pspec)
			}
		}
		log.Debugf("HTTP probe - filtered ports => %+v", probe.Ports)
	} else {
		//order the port list based on the order of the 'EXPOSE' instructions
		if len(inspector.ImageInspector.DockerfileInfo.ExposedPorts) > 0 {
			for epi := len(inspector.ImageInspector.DockerfileInfo.ExposedPorts) - 1; epi >= 0; epi-- {
				portInfo := inspector.ImageInspector.DockerfileInfo.ExposedPorts[epi]
				if strings.Index(portInfo, "/") == -1 {
					portInfo = fmt.Sprintf("%v/tcp", portInfo)
				}

				pspec := dockerapi.Port(portInfo)

				if _, ok := inspector.ContainerInfo.NetworkSettings.Ports[pspec]; ok {
					hostPort := inspector.ContainerInfo.NetworkSettings.Ports[pspec][0].HostPort
					if inspector.InContainer {
						if containerPort := availableHostPorts[hostPort]; containerPort != "" {
							probe.Ports = append(probe.Ports, containerPort)
						} else {
							log.Debugf("HTTP probe - could not find container port from host port => %v", hostPort)
						}
					} else {
						probe.Ports = append(probe.Ports, hostPort)
					}

					if _, ok := availableHostPorts[hostPort]; ok {
						log.Debugf("HTTP probe - delete exposed port from the available host ports => %v (%v)", hostPort, portInfo)
						delete(availableHostPorts, hostPort)
					}
				} else {
					log.Debugf("HTTP probe - Unknown exposed port - %v", portInfo)
				}
			}
		}

		for hostPort, containerPort := range availableHostPorts {
			if inspector.InContainer {
				probe.Ports = append(probe.Ports, containerPort)
			} else {
				probe.Ports = append(probe.Ports, hostPort)
			}
		}

		log.Debugf("HTTP probe - probe.Ports => %+v", probe.Ports)
	}

	if len(probe.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

func (p *CustomProbe) loadAPISpecFiles() {
	for _, info := range p.APISpecFiles {
		fileName := info
		prefixOverride := ""
		if strings.Contains(info, ":") {
			parts := strings.SplitN(info, ":", 2)
			fileName = parts[0]
			prefixOverride = parts[1]
		}

		spec, err := loadAPISpecFromFile(fileName)
		if err != nil {
			fmt.Printf("%s info=http.probe.apispec.error message='error loading api spec file' error='%v'\n", p.PrintPrefix, err)
			continue
		}

		if spec == nil {
			fmt.Printf("%s info=http.probe.apispec message='unsupported spec type'\n", p.PrintPrefix)
			continue
		}

		info := apiSpecInfo{
			spec:           spec,
			prefixOverride: prefixOverride,
		}

		p.APISpecProbes = append(p.APISpecProbes, info)
	}
}

func parseAPISpec(rdata []byte) (*openapi3.Swagger, error) {
	if isOpenAPI(rdata) {
		log.Debug("http.CustomProbe.parseAPISpec - is openapi")
		loader := openapi3.NewSwaggerLoader()
		loader.IsExternalRefsAllowed = true
		spec, err := loader.LoadSwaggerFromData(rdata)
		if err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.LoadSwaggerFromData - error=%v", err)
			return nil, err
		}

		return spec, nil
	}

	if isSwagger(rdata) {
		log.Debug("http.CustomProbe.parseAPISpec - is swagger")
		spec2 := &openapi2.Swagger{}
		if err := yaml.Unmarshal(rdata, spec2); err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.yaml.Unmarshal - error=%v", err)
			return nil, err
		}

		spec, err := openapi2conv.ToV3Swagger(spec2)
		if err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.ToV3Swagger - error=%v", err)
			return nil, err
		}

		return spec, nil
	}

	log.Debugf("http.CustomProbe.parseAPISpec - unsupported api spec type (%d): %s", len(rdata), string(rdata))
	return nil, nil
}

func loadAPISpecFromEndpoint(endpoint string) (*openapi3.Swagger, error) {
	httpClient := &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint(%s)", endpoint)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.http.NewRequest - error=%v", err)
		return nil, err
	}

	res, err := httpClient.Do(req)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.httpClient.Do - error=%v", err)
		return nil, err
	}

	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if res.Body != nil {
		rdata, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.response.read - error=%v", err)
			return nil, err
		}

		return parseAPISpec(rdata)
	}

	log.Debug("http.CustomProbe.loadAPISpecFromEndpoint.response - no body")
	return nil, nil
}

func loadAPISpecFromFile(name string) (*openapi3.Swagger, error) {
	rdata, err := ioutil.ReadFile(name)
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

func apiSpecPrefix(spec *openapi3.Swagger) (string, error) {
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
					if vinfo.Default != nil {
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

		fmt.Println("target API prefix:", parsed.Path)
		if parsed.Path != "" && parsed.Path != "/" {
			prefix = prefix
		}
	}

	return prefix, nil
}

func (p *CustomProbe) loadAPISpecs(proto, targetHost, port string) {
	//TODO:
	//Need to support user provided target port for the spec,
	//but these need to be mapped to the actual port at runtime
	//Need to support user provided target proto for the spec
	for _, info := range p.APISpecs {
		specPath := info
		prefixOverride := ""
		if strings.Contains(info, ":") {
			parts := strings.SplitN(info, ":", 2)
			specPath = parts[0]
			prefixOverride = parts[1]
		}

		addr := fmt.Sprintf("%s://%v:%v%v", proto, targetHost, port, specPath)
		spec, err := loadAPISpecFromEndpoint(addr)
		if err != nil {
			fmt.Printf("%s info=http.probe.apispec.error message='error loading api spec from endpoint' error='%v'\n", p.PrintPrefix, err)
			continue
		}

		if spec == nil {
			fmt.Printf("%s info=http.probe.apispec message='unsupported spec type'\n", p.PrintPrefix)
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

func (p *CustomProbe) probeAPISpecEndpoints(proto, targetHost, port, prefix string, spec *openapi3.Swagger) {
	addr := fmt.Sprintf("%s://%s:%s", proto, targetHost, port)

	if p.PrintState {
		fmt.Printf("%s state=http.probe.api-spec.probe.endpoint.starting addr='%s' prefix='%s' endpoints=%d\n", p.PrintPrefix, addr, prefix, len(spec.Paths))
	}

	httpClient := &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
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
	if p.RetryCount > 0 {
		maxRetryCount = p.RetryCount
	}

	notReadyErrorWait := time.Duration(16)
	webErrorWait := time.Duration(8)
	otherErrorWait := time.Duration(4)
	if p.RetryWait > 0 {
		webErrorWait = time.Duration(p.RetryWait)
		notReadyErrorWait = time.Duration(p.RetryWait * 2)
		otherErrorWait = time.Duration(p.RetryWait / 2)
	}

	method = strings.ToUpper(method)
	for i := 0; i < maxRetryCount; i++ {
		req, err := http.NewRequest(method, endpoint, nil)
		//no body, no request headers and no credentials for now
		res, err := client.Do(req)
		p.CallCount++

		if res != nil {
			if res.Body != nil {
				io.Copy(ioutil.Discard, res.Body)
			}

			defer res.Body.Close()
		}

		statusCode := "error"
		callErrorStr := ""
		if err == nil {
			statusCode = fmt.Sprintf("%v", res.StatusCode)
		} else {
			callErrorStr = fmt.Sprintf("error='%v'", err.Error())
		}

		if p.PrintState {
			fmt.Printf("%s info=http.probe.api-spec.probe.endpoint.call status=%v method=%v endpoint=%v attempt=%v %v time=%v\n",
				p.PrintPrefix,
				statusCode,
				method,
				endpoint,
				i+1,
				callErrorStr,
				time.Now().UTC().Format(time.RFC3339))
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

// Start starts the HTTP probe instance execution
func (p *CustomProbe) Start() {
	if p.PrintState {
		fmt.Printf("%s state=http.probe.starting message='WAIT FOR HTTP PROBE TO FINISH'\n", p.PrintPrefix)
	}

	go func() {
		//TODO: need to do a better job figuring out if the target app is ready to accept connections
		time.Sleep(9 * time.Second)

		if p.PrintState {
			fmt.Printf("%s state=http.probe.running\n", p.PrintPrefix)
		}

		httpClient := &http.Client{
			Timeout: time.Second * 30,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		http2Client := &http.Client{
			Timeout: time.Second * 30,
			Transport: &http2.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		log.Info("HTTP probe started...")

		findIdx := func(ports []string, target string) int {
			for idx, val := range ports {
				if val == target {
					return idx
				}
			}
			return -1
		}

		httpIdx := findIdx(p.Ports, httpPortStr)
		httpsIdx := findIdx(p.Ports, httpsPortStr)
		if httpIdx != -1 && httpsIdx != -1 && httpsIdx < httpIdx {
			//want to probe http first
			log.Debugf("http.probe - swapping http and https ports (http=%v <-> https=%v)",
				httpIdx, httpsIdx)

			p.Ports[httpIdx], p.Ports[httpsIdx] = p.Ports[httpsIdx], p.Ports[httpIdx]
		}

		if p.PrintState {
			fmt.Printf("%s info=http.probe.ports count=%d targets='%s'\n",
				p.PrintPrefix, len(p.Ports), strings.Join(p.Ports, ","))

			var cmdListPreview []string
			var cmdListTail string
			for idx, c := range p.Cmds {
				cmdListPreview = append(cmdListPreview, fmt.Sprintf("%s %s", c.Method, c.Resource))
				if idx == 2 {
					cmdListTail = ",..."
					break
				}
			}

			fmt.Printf("%s info=http.probe.commands count=%d commands='%s%s'\n",
				p.PrintPrefix, len(p.Cmds), strings.Join(cmdListPreview, ","), cmdListTail)
		}

		for _, port := range p.Ports {
			//If it's ok stop after the first successful probe pass
			if p.OkCount > 0 && !p.ProbeFull {
				break
			}

			for _, cmd := range p.Cmds {
				reqBody := strings.NewReader(cmd.Body)

				var protocols []string
				if cmd.Protocol == "" {
					//need a smarter and more dynamic way to determine the actual protocol type
					switch port {
					case httpPortStr:
						protocols = []string{config.ProtoHTTP}
					case httpsPortStr:
						protocols = []string{config.ProtoHTTPS}
					default:
						protocols = []string{config.ProtoHTTP, config.ProtoHTTPS}
					}
				} else {
					protocols = []string{cmd.Protocol}
				}

				for _, proto := range protocols {
					var targetHost string
					if p.ContainerInspector.InContainer {
						targetHost = p.ContainerInspector.ContainerInfo.NetworkSettings.IPAddress
					} else {
						targetHost = p.ContainerInspector.DockerHostIP
					}

					var prefix string
					var client *http.Client
					switch proto {
					case config.ProtoHTTP:
						client = httpClient
						prefix = proto
					case config.ProtoHTTPS:
						client = httpClient
						prefix = proto
					case config.ProtoHTTP2:
						client = http2Client
						prefix = "https"
					}

					addr := fmt.Sprintf("%s://%v:%v%v", prefix, targetHost, port, cmd.Resource)

					maxRetryCount := probeRetryCount
					if p.RetryCount > 0 {
						maxRetryCount = p.RetryCount
					}

					notReadyErrorWait := time.Duration(16)
					webErrorWait := time.Duration(8)
					otherErrorWait := time.Duration(4)
					if p.RetryWait > 0 {
						webErrorWait = time.Duration(p.RetryWait)
						notReadyErrorWait = time.Duration(p.RetryWait * 2)
						otherErrorWait = time.Duration(p.RetryWait / 2)
					}

					for i := 0; i < maxRetryCount; i++ {
						req, err := http.NewRequest(cmd.Method, addr, reqBody)
						for _, hline := range cmd.Headers {
							hparts := strings.SplitN(hline, ":", 2)
							if len(hparts) != 2 {
								log.Debugf("ignoring malformed header (%v)", hline)
								continue
							}

							hname := strings.TrimSpace(hparts[0])
							hvalue := strings.TrimSpace(hparts[1])
							req.Header.Add(hname, hvalue)
						}

						if (cmd.Username != "") || (cmd.Password != "") {
							req.SetBasicAuth(cmd.Username, cmd.Password)
						}

						res, err := client.Do(req)
						p.CallCount++
						reqBody.Seek(0, 0)

						if res != nil {
							if res.Body != nil {
								io.Copy(ioutil.Discard, res.Body)
							}

							defer res.Body.Close()
						}

						statusCode := "error"
						callErrorStr := ""
						if err == nil {
							statusCode = fmt.Sprintf("%v", res.StatusCode)
						} else {
							callErrorStr = fmt.Sprintf("error='%v'", err.Error())
						}

						if p.PrintState {
							fmt.Printf("%s info=http.probe.call status=%v method=%v target=%v attempt=%v %v time=%v\n",
								p.PrintPrefix,
								statusCode,
								cmd.Method,
								addr,
								i+1,
								callErrorStr,
								time.Now().UTC().Format(time.RFC3339))
						}

						if err == nil {
							p.OkCount++

							if p.OkCount == 1 {
								//fetch the API spec when we know the target is reachable
								p.loadAPISpecs(proto, targetHost, port)

								//ideally api spec probes should work without http probe commands
								//for now trigger the api spec probes after the first successful http probe command
								//and once the api specs are loaded
								for _, specInfo := range p.APISpecProbes {
									var apiPrefix string
									if specInfo.prefixOverride != "" {
										apiPrefix = specInfo.prefixOverride
									} else {
										apiPrefix, err = apiSpecPrefix(specInfo.spec)
										if err != nil {
											fmt.Printf("%s info=http.probe.api-spec.error message='api prefix error' error='%v'\n",
												p.PrintPrefix, err)
											continue
										}
									}

									p.probeAPISpecEndpoints(proto, targetHost, port, apiPrefix, specInfo.spec)
								}
							}

							if cmd.Crawl {
								p.crawl(targetHost, addr)
							}
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
			}
		}

		log.Info("HTTP probe done.")

		if p.PrintState {
			fmt.Printf("%s info=http.probe.summary total=%v failures=%v successful=%v\n",
				p.PrintPrefix, p.CallCount, p.ErrCount, p.OkCount)

			warning := ""
			switch {
			case p.CallCount == 0:
				warning = "warning=no.calls"
			case p.OkCount == 0:
				warning = "warning=no.successful.calls"
			}

			fmt.Printf("%s state=http.probe.done %s\n", p.PrintPrefix, warning)
		}

		if p.CallCount > 0 && p.OkCount == 0 && p.ProbeExitOnFailure {
			fmt.Printf("%s info=probe.error reason=no.successful.calls\n", p.PrintPrefix)
			if p.ContainerInspector != nil {
				p.ContainerInspector.ShowContainerLogs()
			}
			fmt.Printf("%s state=exited\n", p.PrintPrefix)
			os.Exit(-1)
		}

		p.workers.Wait()
		close(p.doneChan)
	}()
}

// DoneChan returns the 'done' channel for the HTTP probe instance
func (p *CustomProbe) DoneChan() <-chan struct{} {
	return p.doneChan
}

func (p *CustomProbe) crawl(domain, addr string) {
	p.workers.Add(1)
	if p.maxConcurrentCrawlers > 0 &&
		p.concurrentCrawlers != nil {
		p.concurrentCrawlers <- struct{}{}
	}

	go func() {
		defer func() {
			if p.maxConcurrentCrawlers > 0 &&
				p.concurrentCrawlers != nil {
				<-p.concurrentCrawlers
			}

			p.workers.Done()
		}()

		var pageCount int

		c := colly.NewCollector()
		c.UserAgent = "ds.crawler"
		c.IgnoreRobotsTxt = true
		c.Async = true
		c.AllowedDomains = []string{domain}
		c.AllowURLRevisit = false

		if p.crawlMaxDepth > 0 {
			c.MaxDepth = p.crawlMaxDepth
		}

		if p.crawlConcurrency > 0 {
			c.Limit(&colly.LimitRule{
				DomainGlob:  "*",
				Parallelism: p.crawlConcurrency,
			})
		}

		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(a[href]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("href"))
		})

		c.OnHTML("link[href]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(link[href]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			switch e.Attr("rel") {
			case "dns-prefetch", "preconnect", "alternate":
				return
			}

			e.Request.Visit(e.Attr("href"))
		})

		c.OnHTML("script[src], source[src], img[src]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(script/source/img) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("src"))
		})

		c.OnHTML("source[srcset]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML(source[srcset]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("srcset"))
		})

		c.OnHTML("[data-src]", func(e *colly.HTMLElement) {
			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				log.Debugf("http.CustomProbe.crawl.OnHTML([data-src]) - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("data-src"))
		})

		c.OnRequest(func(r *colly.Request) {
			fmt.Printf("%s info=probe.crawler page=%v url=%v\n", p.PrintPrefix, pageCount, r.URL)

			if p.crawlMaxPageCount > 0 &&
				pageCount > p.crawlMaxPageCount {
				fmt.Println("reached max visits...")
				log.Debugf("http.CustomProbe.crawl.OnRequest - reached max page count (%v)", p.crawlMaxPageCount)
				r.Abort()
				return
			}

			pageCount++
		})

		c.OnError(func(_ *colly.Response, err error) {
			log.Tracef("http.CustomProbe.crawl - error=%v", err)
		})

		c.Visit(addr)
		c.Wait()
		fmt.Printf("%s info=probe.crawler.done addr=%v\n", p.PrintPrefix, addr)
	}()
}
