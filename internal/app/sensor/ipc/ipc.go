package ipc

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/pub"
	"github.com/go-mangos/mangos/protocol/rep"
	//"github.com/go-mangos/mangos/transport/ipc"
	"github.com/go-mangos/mangos/transport/tcp"

	"github.com/docker-slim/docker-slim/pkg/ipc/channel"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
)

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

// ShutdownChannels destroys the communication channels with the master
func ShutdownChannels() {
	shutdownCmdChannel()
	shutdownEvtChannel()
}

// RunCmdServer starts the command server
func RunCmdServer(done <-chan struct{}) (<-chan command.Message, error) {
	return runCmdServer(cmdChannel, done)
}

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
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Listen(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}

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
					} else {
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

func shutdownCmdChannel() {
	if cmdChannel != nil {
		cmdChannel.Close()
		cmdChannel = nil
	}
}

var evtChannelAddr = fmt.Sprintf("tcp://0.0.0.0:%d", channel.EvtPort)

//var evtChannelAddr = "ipc:///tmp/docker-slim-sensor.events.ipc"
//var evtChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-sensor.events.ipc"
var evtChannel mangos.Socket

func newEvtPublisher(addr string) (mangos.Socket, error) {
	log.Info("sensor: creating event publisher...")
	socket, err := pub.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionSendDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err = socket.Listen(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}

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

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}
