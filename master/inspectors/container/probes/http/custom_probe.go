package http

import (
	"fmt"
	"time"

	"github.com/docker-slim/docker-slim/master/config"
	"github.com/docker-slim/docker-slim/master/inspectors/container"

	log "github.com/Sirupsen/logrus"
	"github.com/franela/goreq"
)

// CustomProbe is a custom HTTP probe
type CustomProbe struct {
	Ports              []string
	Cmds               []config.HTTPProbeCmd
	ContainerInspector *container.Inspector
	doneChan           chan struct{}
}

// NewCustomProbe creates a new custom HTTP probe
func NewCustomProbe(inspector *container.Inspector, cmds []config.HTTPProbeCmd) (*CustomProbe, error) {
	//add default probe: GET /
	cmds = append(cmds, config.HTTPProbeCmd{Protocol: "http", Method: "GET", Resource: "/"})

	probe := &CustomProbe{
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
		time.Sleep(4 * time.Second)

		log.Info("docker-slim: HTTP probe started...")
		goreq.SetConnectTimeout(10 * time.Second)

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
					res, err := goreq.Request{
						Method:  cmd.Method,
						Uri:     addr,
						Body:    cmd.Body,
						Timeout: 5 * time.Second,
						//ShowDebug: true,
					}.Do()

					if err == nil {
						log.Infof("docker-slim: http probe - %v %v => %v", cmd.Method, addr, res.StatusCode)
						break
					}

					log.Infof("docker-slim: http probe - %v %v error: %v", cmd.Method, addr, err)
				}
			}
		}

		log.Info("docker-slim: HTTP probe done.")
		close(p.doneChan)
	}()
}

// DoneChan returns the 'done' channel for the HTTP probe instance
func (p *CustomProbe) DoneChan() <-chan struct{} {
	return p.doneChan
}
