package http

import (
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
	"github.com/gocolly/colly/v2"
	log "github.com/sirupsen/logrus"

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

	return probe, nil
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
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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
						protocols = []string{"http"}
					case httpsPortStr:
						protocols = []string{"https"}
					default:
						protocols = []string{"http", "https"}
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

					addr := fmt.Sprintf("%s://%v:%v%v", proto, targetHost, port, cmd.Resource)

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

						res, err := httpClient.Do(req)
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
				log.Debugf("http.CustomProbe.crawl.OnHTML - reached max page count, ignoring link (%v)", p.crawlMaxPageCount)
				return
			}

			e.Request.Visit(e.Attr("href"))
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
