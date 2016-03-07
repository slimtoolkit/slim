package http

import (
	"fmt"
	"time"

	"github.com/cloudimmunity/docker-slim/master/config"
	"github.com/cloudimmunity/docker-slim/master/inspectors/container"

	log "github.com/Sirupsen/logrus"
	"github.com/franela/goreq"
)

type HttpProbe struct {
	Ports              []string
	Cmds               []config.HttpProbeCmd
	ContainerInspector *container.Inspector
}

func NewCustomProbe(inspector *container.Inspector, cmds []config.HttpProbeCmd) (*HttpProbe, error) {
	//add default probe: GET /
	cmds = append(cmds, config.HttpProbeCmd{Protocol: "http", Method: "GET", Resource: "/"})

	probe := &HttpProbe{
		Cmds:               cmds,
		ContainerInspector: inspector,
	}

	for nsPortKey, nsPortData := range inspector.ContainerInfo.NetworkSettings.Ports {
		if (nsPortKey == inspector.CmdPort) || (nsPortKey == inspector.EvtPort) {
			continue
		}

		probe.Ports = append(probe.Ports, nsPortData[0].HostPort)
	}

	return probe, nil
}

func (p *HttpProbe) Start() {
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
						Timeout: 5 * time.Second,
						//ShowDebug: true,
					}.Do()

					if err == nil {
						log.Infof("docker-slim: http probe - %v %v => %v\n", cmd.Method, addr, res.StatusCode)
						break
					}

					log.Infof("docker-slim: http probe - %v %v error: %v\n", cmd.Method, addr, err)
				}
			}
		}

		log.Info("docker-slim: HTTP probe done.")
	}()
}
