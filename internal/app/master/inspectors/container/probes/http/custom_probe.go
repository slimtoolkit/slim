package http

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/docker-slim/docker-slim/internal/app/master/config"
	"github.com/docker-slim/docker-slim/internal/app/master/inspectors/container"

	log "github.com/Sirupsen/logrus"
)

const (
	probeRetryCount = 5
)

// CustomProbe is a custom HTTP probe
type CustomProbe struct {
	PrintState         bool
	PrintPrefix        string
	Ports              []string
	Cmds               []config.HTTPProbeCmd
	ContainerInspector *container.Inspector
	doneChan           chan struct{}
}

// NewCustomProbe creates a new custom HTTP probe
func NewCustomProbe(inspector *container.Inspector,
	cmds []config.HTTPProbeCmd,
	printState bool,
	printPrefix string) (*CustomProbe, error) {
	//note: the default probe should already be there if the user asked for it

	probe := &CustomProbe{
		PrintState:         printState,
		PrintPrefix:        printPrefix,
		Cmds:               cmds,
		ContainerInspector: inspector,
		doneChan:           make(chan struct{}),
	}

	for nsPortKey, nsPortData := range inspector.ContainerInfo.NetworkSettings.Ports {
		if (nsPortKey == inspector.CmdPort) || (nsPortKey == inspector.EvtPort) {
			continue
		}

		probe.Ports = append(probe.Ports, nsPortData[0].HostPort)
	}

	return probe, nil
}

// Start starts the HTTP probe instance execution
func (p *CustomProbe) Start() {
	go func() {
		//TODO: need to do a better job figuring out if the target app is ready to accept connections
		time.Sleep(9 * time.Second)

		if p.PrintState {
			fmt.Printf("%s state=http.probe.starting\n", p.PrintPrefix)
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

		for _, port := range p.Ports {
			for _, cmd := range p.Cmds {
				var protocols []string
				if cmd.Protocol == "" {
					protocols = []string{"http", "https"}
				} else {
					protocols = []string{cmd.Protocol}
				}

				for _, proto := range protocols {
					addr := fmt.Sprintf("%s://%v:%v%v", proto, p.ContainerInspector.DockerHostIP, port, cmd.Resource)

					for i := 0; i < probeRetryCount; i++ {
						req, err := http.NewRequest(cmd.Method, addr, nil)
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

						if res != nil {
							if res.Body != nil {
								io.Copy(ioutil.Discard, res.Body)
							}

							defer res.Body.Close()
						}

						statusCode := 0
						callErrorStr := "none"
						if err == nil {
							statusCode = res.StatusCode
						} else {
							callErrorStr = err.Error()
						}

						if p.PrintState {
							fmt.Printf("%s info=http.probe.call status=%v method=%v target=%v attempt=%v error=%v time=%v\n",
								p.PrintPrefix,
								statusCode,
								cmd.Method,
								addr,
								i+1,
								callErrorStr,
								time.Now().UTC().Format(time.RFC3339))
						}

						if err == nil {
							break
						} else {
							if urlErr, ok := err.(*url.Error); ok {
								if urlErr.Err == io.EOF {
									log.Debugf("HTTP probe - target not ready yet (retry again later)...")
									time.Sleep(12 * time.Second)
								} else {
									log.Debugf("HTTP probe - web error... retry again later...")
									time.Sleep(8 * time.Second)

								}
							} else {
								log.Debugf("HTTP probe - other error... retry again later...")
								time.Sleep(3 * time.Second)
							}
						}

					}
				}
			}
		}

		log.Info("HTTP probe done.")

		if p.PrintState {
			fmt.Printf("%s state=http.probe.done\n", p.PrintPrefix)
		}

		close(p.doneChan)
	}()
}

// DoneChan returns the 'done' channel for the HTTP probe instance
func (p *CustomProbe) DoneChan() <-chan struct{} {
	return p.doneChan
}
