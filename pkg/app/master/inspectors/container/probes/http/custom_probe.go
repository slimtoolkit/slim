package http

import (
	"fmt"
	//"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"

	"github.com/docker-slim/docker-slim/pkg/app"
	//"github.com/docker-slim/docker-slim/pkg/app/master/commands"
	"github.com/docker-slim/docker-slim/pkg/app/master/config"
	"github.com/docker-slim/docker-slim/pkg/app/master/inspectors/container"
)

const (
	probeRetryCount = 5
	httpPortStr     = "80"
	httpsPortStr    = "443"
)

type ovars = app.OutVars

// CustomProbe is a custom HTTP probe
type CustomProbe struct {
	PrintState            bool
	PrintPrefix           string
	Ports                 []string
	Cmds                  []config.HTTPProbeCmd
	StartWait             int
	RetryCount            int
	RetryWait             int
	TargetPorts           []uint16
	ProbeFull             bool
	ProbeExitOnFailure    bool
	APISpecs              []string
	APISpecFiles          []string
	APISpecProbes         []apiSpecInfo
	ProbeApps             []string
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
	xc                    *app.ExecutionContext
}

// NewCustomProbe creates a new custom HTTP probe
func NewCustomProbe(
	xc *app.ExecutionContext,
	inspector *container.Inspector,
	cmds []config.HTTPProbeCmd,
	startWait int,
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
	probeApps []string,
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
		StartWait:             startWait,
		RetryCount:            retryCount,
		RetryWait:             retryWait,
		TargetPorts:           targetPorts,
		ProbeFull:             probeFull,
		ProbeExitOnFailure:    probeExitOnFailure,
		APISpecs:              apiSpecs,
		APISpecFiles:          apiSpecFiles,
		ProbeApps:             probeApps,
		ContainerInspector:    inspector,
		crawlMaxDepth:         crawlMaxDepth,
		crawlMaxPageCount:     crawlMaxPageCount,
		crawlConcurrency:      crawlConcurrency,
		maxConcurrentCrawlers: maxConcurrentCrawlers,
		doneChan:              make(chan struct{}),
		xc:                    xc,
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
				if inspector.SensorIPCMode == container.SensorIPCModeDirect {
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
					if inspector.SensorIPCMode == container.SensorIPCModeDirect {
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
			if inspector.SensorIPCMode == container.SensorIPCModeDirect {
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

// Start starts the HTTP probe instance execution
func (p *CustomProbe) Start() {
	if p.PrintState {
		p.xc.Out.State("http.probe.starting",
			ovars{
				"message": "WAIT FOR HTTP PROBE TO FINISH",
			})
	}

	go func() {
		//TODO: need to do a better job figuring out if the target app is ready to accept connections
		time.Sleep(9 * time.Second) //base start wait time
		if p.StartWait > 0 {
			if p.PrintState {
				p.xc.Out.State("http.probe.start.wait", ovars{"time": p.StartWait})
			}

			//additional wait time
			time.Sleep(time.Duration(p.StartWait) * time.Second)

			if p.PrintState {
				p.xc.Out.State("http.probe.start.wait.done")
			}
		}

		if p.PrintState {
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

		httpIdx := findIdx(p.Ports, httpPortStr)
		httpsIdx := findIdx(p.Ports, httpsPortStr)
		if httpIdx != -1 && httpsIdx != -1 && httpsIdx < httpIdx {
			//want to probe http first
			log.Debugf("http.probe - swapping http and https ports (http=%v <-> https=%v)",
				httpIdx, httpsIdx)

			p.Ports[httpIdx], p.Ports[httpsIdx] = p.Ports[httpsIdx], p.Ports[httpIdx]
		}

		if p.PrintState {
			p.xc.Out.Info("http.probe.ports",
				ovars{
					"count":   len(p.Ports),
					"targets": strings.Join(p.Ports, ","),
				})

			var cmdListPreview []string
			var cmdListTail string
			for idx, c := range p.Cmds {
				cmdListPreview = append(cmdListPreview, fmt.Sprintf("%s %s", c.Method, c.Resource))
				if idx == 2 {
					cmdListTail = ",..."
					break
				}
			}

			cmdInfo := fmt.Sprintf("%s%s", strings.Join(cmdListPreview, ","), cmdListTail)
			p.xc.Out.Info("http.probe.commands",
				ovars{
					"count":    len(p.Cmds),
					"commands": cmdInfo,
				})
		}

		for _, port := range p.Ports {
			//If it's ok stop after the first successful probe pass
			if p.OkCount > 0 && !p.ProbeFull {
				break
			}

			for _, cmd := range p.Cmds {
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
					targetHost := p.ContainerInspector.TargetHost

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

					if IsValidWSProto(proto) {
						wc, err := NewWebsocketClient(proto, targetHost, port)
						if err != nil {
							log.Debugf("HTTP probe - new websocket error - %v", err)
							continue
						}

						wc.ReadCh = make(chan WebsocketMessage, 10)
						for i := 0; i < maxRetryCount; i++ {
							err = wc.Connect()
							if err != nil {
								log.Debugf("HTTP probe - ws target not ready yet (retry again later)...")
								time.Sleep(notReadyErrorWait * time.Second)
							}

							wc.CheckConnection()
							//TODO: prep data to write from the HTTPProbeCmd fields
							err = wc.WriteString("ws.data")
							p.CallCount++

							if p.PrintState {
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

					client := getHTTPClient(proto)
					baseAddr := getHTTPAddr(proto, targetHost, port)
					addr := fmt.Sprintf("%s%s", baseAddr, cmd.Resource)

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
						rbSeeker.Seek(0, 0)

						if res != nil {
							if res.Body != nil {
								io.Copy(ioutil.Discard, res.Body)
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

						if p.PrintState {
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
											p.xc.Out.Error("http.probe.api-spec.error.prefix", err.Error())
											continue
										}
									}

									p.probeAPISpecEndpoints(proto, targetHost, port, apiPrefix, specInfo.spec)
								}
							}

							if cmd.Crawl {
								p.crawl(proto, targetHost, addr)
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

		if len(p.ProbeApps) > 0 {
			if p.PrintState {
				p.xc.Out.Info("http.probe.apps",
					ovars{
						"count": len(p.ProbeApps),
					})
			}

			for idx, appCall := range p.ProbeApps {
				if p.PrintState {
					p.xc.Out.Info("http.probe.app",
						ovars{
							"idx": idx,
							"app": appCall,
						})
				}

				p.xc.Out.Info("http.probe.app.output.start")
				//TODO LATER:
				//add more parameters and outputs for more advanced execution control capabilities
				err := exeAppCall(appCall)
				p.xc.Out.Info("http.probe.app.output.end")

				p.CallCount++
				statusCode := "error"
				callErrorStr := "none"
				if err == nil {
					p.OkCount++
					statusCode = "ok"
				} else {
					p.ErrCount++
					callErrorStr = err.Error()
				}

				if p.PrintState {
					p.xc.Out.Info("http.probe.app",
						ovars{
							"idx":    idx,
							"app":    appCall,
							"status": statusCode,
							"error":  callErrorStr,
							"time":   time.Now().UTC().Format(time.RFC3339),
						})
				}
			}
		}

		log.Info("HTTP probe done.")

		if p.PrintState {
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

		if p.CallCount > 0 && p.OkCount == 0 && p.ProbeExitOnFailure {
			p.xc.Out.Error("probe.error", "no.successful.calls")

			if p.ContainerInspector != nil {
				p.ContainerInspector.ShowContainerLogs()
			}

			p.xc.Out.State("exited",
				ovars{
					"exit.code": -1,
				})
			p.xc.Exit(-1)
		}

		p.workers.Wait()
		close(p.doneChan)
	}()
}

// DoneChan returns the 'done' channel for the HTTP probe instance
func (p *CustomProbe) DoneChan() <-chan struct{} {
	return p.doneChan
}

func exeAppCall(appCall string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Second)
	defer cancel()

	appCall = strings.TrimSpace(appCall)
	args, err := shlex.Split(appCall)
	if err != nil {
		log.Errorf("exeAppCall(%s): call parse error: %v", appCall, err)
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("empty appCall")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	//cmd.Dir = "."
	cmd.Stdin = os.Stdin

	//var outBuf, errBuf bytes.Buffer
	//cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
	//cmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Errorf("exeAppCall(%s): command start error: %v", appCall, err)
		return err
	}

	err = cmd.Wait()
	fmt.Printf("\n")
	if err != nil {
		log.Fatalf("exeAppCall(%s): command exited with error: %v", appCall, err)
		return err
	}

	//TODO: process outBuf and errBuf here
	return nil
}
