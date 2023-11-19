package ipc

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/ipc/channel"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
)

type Server struct {
	evtChannel *channel.EventServer
	cmdChannel *channel.CommandServer
	cmdChan    chan command.Message
	doneChan   <-chan struct{}
}

func NewServer(doneChan <-chan struct{}) (*Server, error) {
	server := Server{
		doneChan: doneChan,
		cmdChan:  make(chan command.Message, 10),
	}

	if err := server.initChannels(); err != nil {
		log.Errorf("sensor: ipc.NewServer error = %v", err)
		return nil, err
	}

	return &server, nil
}

func (s *Server) initChannels() error {
	evtChannelAddr := fmt.Sprintf("0.0.0.0:%d", channel.EvtPort)
	evtChannel := channel.NewEventServer(evtChannelAddr)
	s.evtChannel = evtChannel

	cmdChannelAddr := fmt.Sprintf("0.0.0.0:%d", channel.CmdPort)
	cmdChannel := channel.NewCommandServer(cmdChannelAddr, s)
	s.cmdChannel = cmdChannel

	return nil
}

func (s *Server) shutdownChannels() error {
	if s.cmdChannel != nil {
		s.cmdChannel.Stop()
		s.cmdChannel = nil
	}

	if s.evtChannel != nil {
		s.evtChannel.Stop()
		s.evtChannel = nil
	}

	return nil
}

func (s *Server) Stop() {
	s.shutdownChannels()
}

func (s *Server) CommandChan() <-chan command.Message {
	return s.cmdChan
}

func (s *Server) OnRequest(data []byte) ([]byte, error) {
	resp := command.Response{
		Status: command.ResponseStatusError,
	}

	if cmd, err := command.Decode(data); err == nil {
		s.cmdChan <- cmd
		resp.Status = command.ResponseStatusOk
	} else {
		log.Errorf("ipc.Server.OnRequest: error decoding request = %v", err)
	}

	respData, err := json.Marshal(&resp)
	if err != nil {
		log.Errorf("ipc.Server.OnRequest: error encoding response = %v", err)
		return nil, err
	}

	return respData, nil
}

func (s *Server) Run() error {
	log.Debug("sensor: ipc.Server.Run()")
	err := s.evtChannel.Start(true)
	if err != nil {
		log.Errorf("sensor: ipc.Server.Run() - evtChannel.Start error = %v\n", err)
		return err
	}

	err = s.cmdChannel.Start(true)
	if err != nil {
		log.Errorf("sensor: ipc.Server.Run() - cmdChannel.Start error = %v\n", err)
		return err
	}

	s.cmdChannel.WaitForConnection()
	s.evtChannel.WaitForConnection()

	go func() {
		for {
			log.Debug("sensor: ipc.Server.Run - waiting for done signal...")
			select {
			case <-s.doneChan:
				log.Debug("sensor: ipc.Server.Run - done...")
				s.Stop()
				return
			}
		}
	}()

	return nil
}

func (s *Server) TryPublishEvt(evt *event.Message, retries uint) error {
	log.Debugf("ipc.Server.TryPublishEvt(%+v)", evt)

	if s.evtChannel == nil {
		log.Warnf("ipc.Server.TryPublishEvt(): skipped - server has already been stopped")
		return nil
	}

	data, err := json.Marshal(evt)
	if err != nil {
		log.Errorf("app.TryPublishEvt(): failed to encode event - %v", err)
		return err
	}

	return s.evtChannel.Publish(data, retries)
}
