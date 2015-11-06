package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/franela/goreq"
)

func startHTTPProbe(dockerHostIP string, httpProbePorts []string) {
	go func(hpHost string, hpPorts []string) {
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
	}(dockerHostIP, httpProbePorts)
}
