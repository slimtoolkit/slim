package ipc

import (
	"encoding/json"
	"fmt"
	//"time"

	//"github.com/go-mangos/mangos"
	//"github.com/go-mangos/mangos/protocol/pub"
	//"github.com/go-mangos/mangos/protocol/rep"

	//"nanomsg.org/go/mangos/v2"
	//"nanomsg.org/go/mangos/v2/protocol/pub"
	//"nanomsg.org/go/mangos/v2/protocol/rep"
	//_ "nanomsg.org/go/mangos/v2/transport/tcp"

	log "github.com/Sirupsen/logrus"

	//"github.com/go-mangos/mangos/transport/ipc"
	//"github.com/go-mangos/mangos/transport/tcp"

	"github.com/docker-slim/docker-slim/pkg/ipc/channel"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
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

	data, err := json.Marshal(evt)
	if err != nil {
		log.Errorf("app.TryPublishEvt(): failed to encode event - %v", err)
		return err
	}

	return s.evtChannel.Publish(data, retries)
}

///////////////////////////////////////////////////////////////////////////
/*
// InitChannels initializes the communication channels with the master
func InitChannels() error {
	var err error
	evtChannel, err = newEvtPublisher(evtChannelAddr)
	if err != nil {
		return err
	}

	cmdChannel, err = newCmdServer(cmdChannelAddr)
	if err != nil {
		return err
	}

	return nil
}
*/

/*
// ShutdownChannels destroys the communication channels with the master
func ShutdownChannels() {
	shutdownCmdChannel()
	shutdownEvtChannel()
}
*/

/*
// RunCmdServer starts the command server
func RunCmdServer(done <-chan struct{}) (<-chan command.Message, error) {
	return runCmdServer(cmdChannel, done)
}
*/

/*
var cmdChannelAddr = fmt.Sprintf("tcp://0.0.0.0:%d", channel.CmdPort)

//var cmdChannelAddr = "ipc:///tmp/docker-slim-sensor.cmds.ipc"
//var cmdChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-sensor.cmds.ipc"
var cmdChannel mangos.Socket

func newCmdServer(addr string) (mangos.Socket, error) {
	log.Info("sensor: creating cmd server...")
	socket, err := rep.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	//socket.AddTransport(tcp.NewTransport())
	if err := socket.Listen(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}
*/

/*
func runCmdServer(channel mangos.Socket, done <-chan struct{}) (<-chan command.Message, error) {
	cmdChan := make(chan command.Message)
	go func() {
		for {
			// Could also use sock.RecvMsg to get header
			log.Debug("sensor: cmd server - waiting for a command...")
			select {
			case <-done:
				log.Debug("sensor: cmd server - done...")
				return
			default:
				if rawCmd, err := channel.Recv(); err != nil {
					switch err {
					case mangos.ErrRecvTimeout:
						log.Debug("sensor: cmd server - timeout... ok")
					default:
						log.Debugln("sensor: cmd server - error =>", err)
					}
				} else {
					log.Debug("sensor: cmd server - got a command => ", string(rawCmd))

					if cmd, err := command.Decode(rawCmd); err != nil {
						log.Println(err)
						err = channel.Send([]byte("error"))
						if err != nil {
							log.Warnln("sensor: cmd server - fail to send command status reply =>", err)
						}
					} else {
						err = channel.Send([]byte("ok"))
						if err != nil {
							log.Warnln("sensor: cmd server - fail to send command status reply =>", err)
						}

						cmdChan <- cmd
					}

					//for now just ack the command and process the command asynchronously
					//NOTE:
					//must reply before receiving the next message
					//otherwise nanomsg/mangos will be confused :-)
					cmdStatusReply := "ok"
					err = channel.Send([]byte(cmdStatusReply))
					if err != nil {
						log.Warnln("sensor: cmd server - fail to send command status reply =>", err)
					}
				}
			}
		}
	}()

	return cmdChan, nil
}
*/

/*
func shutdownCmdChannel() {
	if cmdChannel != nil {
		cmdChannel.Close()
		cmdChannel = nil
	}
}
*/

/*
var evtChannelAddr = fmt.Sprintf("tcp://0.0.0.0:%d", channel.EvtPort)

//var evtChannelAddr = "ipc:///tmp/docker-slim-sensor.events.ipc"
//var evtChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-sensor.events.ipc"
var evtChannel mangos.Socket

func newEvtPublisher(addr string) (mangos.Socket, error) {
	log.Infof("sensor: creating event publisher (addr=%v)...", addr)
	socket, err := pub.NewSocket()
	if err != nil {
		log.Debugf("newEvtPublisher(%v): pub.NewSocket() error = %v", addr, err)
		return nil, err
	}

	//if err := socket.SetOption(mangos.OptionSendDeadline, time.Second*3); err != nil {
	//	socket.Close()
	//	return nil, err
	//}

	//socket.AddTransport(ipc.NewTransport())
	//socket.AddTransport(tcp.NewTransport())
	if err = socket.Listen(addr); err != nil {
		log.Debugf("newEvtPublisher(%v): socket.Listen() error = %v", addr, err)
		socket.Close()
		return nil, err
	}

	return socket, nil
}
*/
/*
func publishEvt(channel mangos.Socket, msg *event.Message) error {
	log.Debugf("publishEvt(%+v)", msg)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Debugf("fail to encoding '%+v' event: %v", msg, err)
		return err
	}

	if err := channel.Send(data); err != nil {
		log.Debugf("fail to publish '%+v' event: %v", msg, err)
		return err
	}

	return nil
}

// TryPublishEvt attempts to publish an event to the master
func TryPublishEvt(ptry uint, msg *event.Message) {
	log.Debugf("TryPublishEvt(%v,%+v)", ptry, msg)

	for ptry := 0; ptry < 3; ptry++ {
		log.Debugf("sensor: trying to publish '%+v' event (attempt %v)", msg, ptry+1)
		err := publishEvt(evtChannel, msg)
		if err == nil {
			log.Infof("sensor: published '%+v'", msg)
			break
		}

		switch err {
		case mangos.ErrRecvTimeout:
			log.Debug("sensor: publish event timeout... ok")
		default:
			log.Warnln("sensor: publish event error =>", err)
		}
	}
}
*/

/*
func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}
*/
