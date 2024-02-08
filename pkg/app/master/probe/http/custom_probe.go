package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/app"
	"github.com/slimtoolkit/slim/pkg/app/master/config"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/container"
	"github.com/slimtoolkit/slim/pkg/app/master/inspectors/pod"
)

const (
	probeRetryCount = 5

	defaultHTTPPortStr    = "80"
	defaultHTTPSPortStr   = "443"
	defaultFastCGIPortStr = "9000"
)

type ovars = app.OutVars

// CustomProbe is a custom HTTP probe
type CustomProbe struct {
	xc *app.ExecutionContext

	opts config.HTTPProbeOptions

	ports      []string
	targetHost string

	APISpecProbes []apiSpecInfo

	printState bool

	CallCount uint64
	ErrCount  uint64
	OkCount   uint64

	doneChan           chan struct{}
	workers            sync.WaitGroup
	concurrentCrawlers chan struct{}
}

// NewEndpointProbe creates a new custom HTTP probe for an endpoint
func NewEndpointProbe(
	xc *app.ExecutionContext,
	targetEndpoint string,
	ports []uint,
	opts config.HTTPProbeOptions,
	printState bool,
) (*CustomProbe, error) {
	probe := newCustomProbe(xc, targetEndpoint, opts, printState)
	if len(ports) == 0 {
		ports = []uint{80}
	}

	for _, pnum := range ports {
		probe.ports = append(probe.ports, fmt.Sprintf("%d", pnum))
	}

	if len(probe.opts.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

// NewContainerProbe creates a new custom HTTP probe
func NewContainerProbe(
	xc *app.ExecutionContext,
	inspector *container.Inspector,
	opts config.HTTPProbeOptions,
	printState bool,
) (*CustomProbe, error) {
	probe := newCustomProbe(xc, inspector.TargetHost, opts, printState)

	availableHostPorts := map[string]string{}
	for nsPortKey, nsPortData := range inspector.AvailablePorts {
		log.Debugf("HTTP probe - target's network port key='%s' data='%#v'", nsPortKey, nsPortData)

		if nsPortKey.Proto() != "tcp" {
			log.Debugf("HTTP probe - skipping non-tcp port => %v", nsPortKey)
			continue
		}

		if nsPortData.HostPort == "" {
			log.Debugf("HTTP probe - skipping network setting without port data => %v", nsPortKey)
			continue
		}

		availableHostPorts[nsPortData.HostPort] = nsPortKey.Port()
	}

	log.Debugf("HTTP probe - available host ports => %+v", availableHostPorts)

	if len(probe.opts.Ports) > 0 {
		for _, pnum := range probe.opts.Ports {
			pspec := dockerapi.Port(fmt.Sprintf("%v/tcp", pnum))
			if _, ok := inspector.AvailablePorts[pspec]; ok {
				if inspector.SensorIPCMode == container.SensorIPCModeDirect {
					probe.ports = append(probe.ports, fmt.Sprintf("%d", pnum))
				} else {
					probe.ports = append(probe.ports, inspector.AvailablePorts[pspec].HostPort)
				}
			} else {
				log.Debugf("HTTP probe - ignoring port => %v", pspec)
			}
		}
		log.Debugf("HTTP probe - filtered ports => %+v", probe.ports)
	} else {
		//order the port list based on the order of the 'EXPOSE' instructions
		if len(inspector.ImageInspector.DockerfileInfo.ExposedPorts) > 0 {
			for epi := len(inspector.ImageInspector.DockerfileInfo.ExposedPorts) - 1; epi >= 0; epi-- {
				portInfo := inspector.ImageInspector.DockerfileInfo.ExposedPorts[epi]
				if !strings.Contains(portInfo, "/") {
					portInfo = fmt.Sprintf("%v/tcp", portInfo)
				}

				pspec := dockerapi.Port(portInfo)

				if _, ok := inspector.AvailablePorts[pspec]; ok {
					hostPort := inspector.AvailablePorts[pspec].HostPort
					if inspector.SensorIPCMode == container.SensorIPCModeDirect {
						if containerPort := availableHostPorts[hostPort]; containerPort != "" {
							probe.ports = append(probe.ports, containerPort)
						} else {
							log.Debugf("HTTP probe - could not find container port from host port => %v", hostPort)
						}
					} else {
						probe.ports = append(probe.ports, hostPort)
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
			if inspector.SensorIPCMode == container.SensorIPCModeDirect {
				probe.ports = append(probe.ports, containerPort)
			} else {
				probe.ports = append(probe.ports, hostPort)
			}
		}

		log.Debugf("HTTP probe - probe.Ports => %+v", probe.ports)
	}

	if len(probe.opts.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

func NewPodProbe(
	xc *app.ExecutionContext,
	inspector *pod.Inspector,
	opts config.HTTPProbeOptions,
	printState bool,
) (*CustomProbe, error) {
	probe := newCustomProbe(xc, inspector.TargetHost(), opts, printState)

	availableHostPorts := map[string]string{}
	for nsPortKey, nsPortData := range inspector.AvailablePorts() {
		log.Debugf("HTTP probe - target's network port key='%s' data='%#v'", nsPortKey, nsPortData)
		availableHostPorts[nsPortData.HostPort] = nsPortKey.Port()
	}

	log.Debugf("HTTP probe - available host ports => %+v", availableHostPorts)

	if len(probe.opts.Ports) > 0 {
		for _, pnum := range probe.opts.Ports {
			pspec := dockerapi.Port(fmt.Sprintf("%v/tcp", pnum))
			if port, ok := inspector.AvailablePorts()[pspec]; ok {
				probe.ports = append(probe.ports, port.HostPort)
			} else {
				log.Debugf("HTTP probe - ignoring port => %v", pspec)
			}
		}

		log.Debugf("HTTP probe - filtered ports => %+v", probe.ports)
	} else {
		for hostPort := range availableHostPorts {
			probe.ports = append(probe.ports, hostPort)
		}

		log.Debugf("HTTP probe - probe.Ports => %+v", probe.ports)
	}

	if len(probe.opts.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

func newCustomProbe(
	xc *app.ExecutionContext,
	targetHost string,
	opts config.HTTPProbeOptions,
	printState bool,
) *CustomProbe {
	//note: the default probe should already be there if the user asked for it

	//-1 means disabled
	if opts.CrawlMaxDepth == 0 {
		opts.CrawlMaxDepth = defaultCrawlMaxDepth
	}

	//-1 means disabled
	if opts.CrawlMaxPageCount == 0 {
		opts.CrawlMaxPageCount = defaultCrawlMaxPageCount
	}

	//-1 means disabled
	if opts.CrawlConcurrency == 0 {
		opts.CrawlConcurrency = defaultCrawlConcurrency
	}

	//-1 means disabled
	if opts.CrawlConcurrencyMax == 0 {
		opts.CrawlConcurrencyMax = defaultMaxConcurrentCrawlers
	}

	probe := &CustomProbe{
		xc:         xc,
		opts:       opts,
		printState: printState,
		targetHost: targetHost,
		doneChan:   make(chan struct{}),
	}

	if opts.CrawlConcurrencyMax > 0 {
		probe.concurrentCrawlers = make(chan struct{}, opts.CrawlConcurrencyMax)
	}

	return probe
}

func (p *CustomProbe) Ports() []string {
	return p.ports
}

// Start starts the HTTP probe instance execution
func (p *CustomProbe) Start() {
	if p.printState {
		p.xc.Out.State("http.probe.starting",
			ovars{
				"message": "WAIT FOR HTTP PROBE TO FINISH",
			})
	}

	go func() {
		//TODO: need to do a better job figuring out if the target app is ready to accept connections
		time.Sleep(9 * time.Second) //base start wait time
		if p.opts.StartWait > 0 {
			if p.printState {
				p.xc.Out.State("http.probe.start.wait", ovars{"time": p.opts.StartWait})
			}

			//additional wait time
			time.Sleep(time.Duration(p.opts.StartWait) * time.Second)

			if p.printState {
				p.xc.Out.State("http.probe.start.wait.done")
			}
		}

		if p.printState {
			p.xc.Out.State("http.probe.running")
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

		httpIdx := findIdx(p.ports, defaultHTTPPortStr)
		httpsIdx := findIdx(p.ports, defaultHTTPSPortStr)
		if httpIdx != -1 && httpsIdx != -1 && httpsIdx < httpIdx {
			//want to probe http first
			log.Debugf("http.probe - swapping http and https ports (http=%v <-> https=%v)",
				httpIdx, httpsIdx)

			p.ports[httpIdx], p.ports[httpsIdx] = p.ports[httpsIdx], p.ports[httpIdx]
		}

		if p.printState {
			p.xc.Out.Info("http.probe.ports",
				ovars{
					"count":   len(p.ports),
					"targets": strings.Join(p.ports, ","),
				})

			var cmdListPreview []string
			var cmdListTail string
			for idx, c := range p.opts.Cmds {
				cmdListPreview = append(cmdListPreview, fmt.Sprintf("%s %s", c.Method, c.Resource))
				if idx == 2 {
					cmdListTail = ",..."
					break
				}
			}

			cmdInfo := fmt.Sprintf("%s%s", strings.Join(cmdListPreview, ","), cmdListTail)
			p.xc.Out.Info("http.probe.commands",
				ovars{
					"count":    len(p.opts.Cmds),
					"commands": cmdInfo,
				})
		}

		for _, port := range p.ports {
			//If it's ok stop after the first successful probe pass
			if p.OkCount > 0 && !p.opts.Full {
				break
			}

			for _, cmd := range p.opts.Cmds {
				var reqBody io.Reader
				var rbSeeker io.Seeker

				if cmd.BodyFile != "" {
					_, err := os.Stat(cmd.BodyFile)
					if err != nil {
						log.Errorf("http.probe - cmd.BodyFile (%s) check error: %v", cmd.BodyFile, err)
					} else {
						bodyFile, err := os.Open(cmd.BodyFile)
						if err != nil {
							log.Errorf("http.probe - cmd.BodyFile (%s) read error: %v", cmd.BodyFile, err)
						} else {
							reqBody = bodyFile
							rbSeeker = bodyFile
							//the file will be closed only when the function exits
							defer bodyFile.Close()
						}
					}
				} else {
					strBody := strings.NewReader(cmd.Body)
					reqBody = strBody
					rbSeeker = strBody
				}

				// TODO: need a smarter and more dynamic way to determine the actual protocol type

				// Set up FastCGI defaults if the default CGI port is used without a FastCGI config.
				if port == defaultFastCGIPortStr && cmd.FastCGI == nil {
					log.Debugf("HTTP probe - FastCGI default port (%s) used, setting up HTTP probe FastCGI wrapper defaults", port)

					// Typicall the entrypoint into a PHP app.
					if cmd.Resource == "/" {
						cmd.Resource = "/index.php"
					}

					// SplitPath is typically on the first .php path element.
					var splitPath []string
					if phpIdx := strings.Index(cmd.Resource, ".php"); phpIdx != -1 {
						splitPath = []string{cmd.Resource[:phpIdx+4]}
					}

					cmd.FastCGI = &config.FastCGIProbeWrapperConfig{
						// /var/www is a typical root for PHP indices.
						Root:      "/var/www",
						SplitPath: splitPath,
					}
				}

				var protocols []string
				if cmd.Protocol == "" {
					switch port {
					case defaultHTTPPortStr:
						protocols = []string{config.ProtoHTTP}
					case defaultHTTPSPortStr:
						protocols = []string{config.ProtoHTTPS}
					default:
						protocols = []string{config.ProtoHTTP, config.ProtoHTTPS}
					}
				} else {
					protocols = []string{cmd.Protocol}
				}

				for _, proto := range protocols {
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

					if IsValidWSProto(proto) {
						wc, err := NewWebsocketClient(proto, p.targetHost, port)
						if err != nil {
							log.Debugf("HTTP probe - new websocket error - %v", err)
							continue
						}

						wc.ReadCh = make(chan WebsocketMessage, 10)
						for i := 0; i < maxRetryCount; i++ {
							err = wc.Connect()
							if err != nil {
								log.Debugf("HTTP probe - ws target not ready yet (retry again later) [err=%v]...", err)
								time.Sleep(notReadyErrorWait * time.Second)
								continue
							}

							wc.CheckConnection()
							//TODO: prep data to write from the HTTPProbeCmd fields
							err = wc.WriteString("ws.data")
							p.CallCount++

							if p.printState {
								statusCode := "error"
								callErrorStr := "none"
								if err == nil {
									statusCode = "ok"
								} else {
									callErrorStr = err.Error()
								}

								p.xc.Out.Info("http.probe.call.ws",
									ovars{
										"status":    statusCode,
										"stats.rc":  wc.ReadCount,
										"stats.pic": wc.PingCount,
										"stats.poc": wc.PongCount,
										"target":    wc.Addr,
										"attempt":   i + 1,
										"error":     callErrorStr,
										"time":      time.Now().UTC().Format(time.RFC3339),
									})
							}

							if err != nil {
								p.ErrCount++
								log.Debugf("HTTP probe - websocket write error - %v", err)
								time.Sleep(notReadyErrorWait * time.Second)
							} else {
								p.OkCount++

								//try to read something from the socket
								select {
								case wsMsg := <-wc.ReadCh:
									log.Debugf("HTTP probe - websocket read - [type=%v data=%s]", wsMsg.Type, string(wsMsg.Data))
								case <-time.After(time.Second * 5):
									log.Debugf("HTTP probe - websocket read time out")
								}

								break
							}
						}

						wc.Disconnect()
						continue
					}

					var client *http.Client
					switch {
					case cmd.FastCGI != nil:
						log.Debug("HTTP probe - FastCGI embedded proxy configured")
						client = getFastCGIClient(cmd.FastCGI)
					default:
						var err error
						if client, err = getHTTPClient(proto); err != nil {
							p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
							continue
						}
					}

					baseAddr := getHTTPAddr(proto, p.targetHost, port)
					// TODO: cmd.Resource may need to be a part of cmd.FastCGI instead.
					addr := fmt.Sprintf("%s%s", baseAddr, cmd.Resource)

					req, err := newHTTPRequestFromCmd(cmd, addr, reqBody)
					if err != nil {
						p.xc.Out.Error("HTTP probe - construct request error - %v", err.Error())
						continue
					}

					for i := 0; i < maxRetryCount; i++ {
						res, err := client.Do(req.Clone(context.Background()))
						p.CallCount++
						rbSeeker.Seek(0, 0)

						if res != nil {
							if res.Body != nil {
								io.Copy(io.Discard, res.Body)
							}

							res.Body.Close()
						}

						statusCode := "error"
						callErrorStr := "none"
						if err == nil {
							statusCode = fmt.Sprintf("%v", res.StatusCode)
						} else {
							callErrorStr = err.Error()
						}

						if p.printState {
							p.xc.Out.Info("http.probe.call",
								ovars{
									"status":  statusCode,
									"method":  cmd.Method,
									"target":  addr,
									"attempt": i + 1,
									"error":   callErrorStr,
									"time":    time.Now().UTC().Format(time.RFC3339),
								})
						}

						if err == nil {
							p.OkCount++

							if p.OkCount == 1 {
								if len(p.opts.APISpecs) != 0 && len(p.opts.APISpecFiles) != 0 && cmd.FastCGI != nil {
									p.xc.Out.Info("HTTP probe - API spec probing not implemented for fastcgi")
								} else {
									p.probeAPISpecs(proto, p.targetHost, port)
								}
							}

							if cmd.Crawl {
								if cmd.FastCGI != nil {
									p.xc.Out.Info("HTTP probe - crawling not implemented for fastcgi")
								} else {
									p.crawl(proto, p.targetHost, addr)
								}
							}
							break
						} else {
							p.ErrCount++

							urlErr := &url.Error{}
							if errors.As(err, &urlErr) {
								if errors.Is(urlErr.Err, io.EOF) {
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

		if p.printState {
			p.xc.Out.Info("http.probe.summary",
				ovars{
					"total":      p.CallCount,
					"failures":   p.ErrCount,
					"successful": p.OkCount,
				})

			outVars := ovars{}
			//warning := ""
			switch {
			case p.CallCount == 0:
				outVars["warning"] = "no.calls"
				//warning = "warning=no.calls"
			case p.OkCount == 0:
				//warning = "warning=no.successful.calls"
				outVars["warning"] = "no.successful.calls"
			}

			p.xc.Out.State("http.probe.done", outVars)
		}

		p.workers.Wait()
		close(p.doneChan)
	}()
}

func (p *CustomProbe) probeAPISpecs(proto, targetHost, port string) {
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
			var err error
			apiPrefix, err = apiSpecPrefix(specInfo.spec)
			if err != nil {
				p.xc.Out.Error("http.probe.api-spec.error.prefix", err.Error())
				continue
			}
		}

		p.probeAPISpecEndpoints(proto, targetHost, port, apiPrefix, specInfo.spec)
	}
}

// DoneChan returns the 'done' channel for the HTTP probe instance
func (p *CustomProbe) DoneChan() <-chan struct{} {
	return p.doneChan
}

func newHTTPRequestFromCmd(cmd config.HTTPProbeCmd, addr string, reqBody io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(context.Background(), cmd.Method, addr, reqBody)
	if err != nil {
		return nil, err
	}

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

	return req, nil
}
