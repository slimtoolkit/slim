package http

import (
	"fmt"
	"time"

	"slim/inspectors/container"

	log "github.com/Sirupsen/logrus"
	"github.com/franela/goreq"
)

type HttpProbe struct {
	Ports              []string
	ContainerInspector *container.Inspector
}

func NewRootProbe(inspector *container.Inspector) (*HttpProbe, error) {
	probe := &HttpProbe{
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
	go func(hpHost string, hpPorts []string) {
		//TODO: need to do a better job figuring out if the target app is ready to accept connections
		time.Sleep(3 * time.Second)

		log.Info("docker-slim: HTTP probe started...")
		goreq.SetConnectTimeout(3 * time.Second)

		//very primitive http probe...
		for _, port := range hpPorts {
			httpAddr := fmt.Sprintf("http://%v:%v", hpHost, port)
			hpResHTTP, err := goreq.Request{
				Uri:     httpAddr,
				Timeout: 5 * time.Second,
				//ShowDebug: true,
			}.Do()
			if err != nil {
				log.Infof("docker-slim: http proble - GET %v error: %v\n", httpAddr, err)

				httpsAddr := fmt.Sprintf("https://%v:%v", hpHost, port)
				hpResHTTPS, err := goreq.Request{
					Uri:      httpAddr,
					Insecure: true,
					Timeout:  5 * time.Second,
					//ShowDebug: true,
				}.Do()
				if err != nil {
					log.Infof("docker-slim: http proble - GET %v error: %v\n", httpsAddr, err)
				} else {
					log.Infof("docker-slim: http proble - GET %v %v\n", httpsAddr, hpResHTTPS.StatusCode)
				}
			} else {
				log.Infof("docker-slim: http proble - GET %v %v\n", httpAddr, hpResHTTP.StatusCode)
			}
		}

		log.Info("docker-slim: HTTP probe done.")
	}(p.ContainerInspector.DockerHostIP, p.Ports)
}
