package ipc

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/pub"
	"github.com/gdamore/mangos/protocol/rep"
	//"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
)

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

func ShutdownChannels() {
	shutdownCmdChannel()
	shutdownEvtChannel()
}

func RunCmdServer(done <-chan struct{}) (<-chan string, error) {
	return runCmdServer(cmdChannel, done)
}

var cmdChannelAddr = "tcp://0.0.0.0:65501"

//var cmdChannelAddr = "ipc:///tmp/docker-slim-launcher.cmds.ipc"
//var cmdChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-launcher.cmds.ipc"
var cmdChannel mangos.Socket

func newCmdServer(addr string) (mangos.Socket, error) {
	log.Info("launcher: creating cmd server...")
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

func runCmdServer(channel mangos.Socket, done <-chan struct{}) (<-chan string, error) {
	cmdChan := make(chan string)
	go func() {
		for {
			// Could also use sock.RecvMsg to get header
			log.Debug("launcher: cmd server - waiting for a command...")
			select {
			case <-done:
				log.Debug("launcher: cmd server - done...")
				return
			default:
				if rawCmd, err := channel.Recv(); err != nil {
					switch err {
					case mangos.ErrRecvTimeout:
						log.Debug("launcher: cmd server - timeout... ok")
					default:
						log.Debugln("launcher: cmd server - error =>", err)
					}
				} else {
					cmd := string(rawCmd)
					log.Debugln("launcher: cmd server - got a command =>", cmd)
					cmdChan <- cmd
					//for now just ack the command and process the command asynchronously
					//NOTE:
					//must reply before receiving the next message
					//otherwise nanomsg/mangos will be confused :-)
					monitorFinishReply := "ok"
					err = channel.Send([]byte(monitorFinishReply))
					if err != nil {
						log.Warnln("launcher: cmd server - fail to send monitor.finish reply =>", err)
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

var evtChannelAddr = "tcp://0.0.0.0:65502"

//var evtChannelAddr = "ipc:///tmp/docker-slim-launcher.events.ipc"
//var evtChannelAddr = "ipc:///opt/dockerslim/ipc/docker-slim-launcher.events.ipc"
var evtChannel mangos.Socket

func newEvtPublisher(addr string) (mangos.Socket, error) {
	log.Info("launcher: creating event publisher...")
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

func publishEvt(channel mangos.Socket, evt string) error {
	if err := channel.Send([]byte(evt)); err != nil {
		log.Debugf("fail to publish '%v' event:%v\n", evt, err)
		return err
	}

	return nil
}

func TryPublishEvt(ptry uint, event string) {
	for ptry := 0; ptry < 3; ptry++ {
		log.Debugf("launcher: trying to publish '%v' event (attempt %v)\n", event, ptry+1)
		err := publishEvt(evtChannel, "monitor.finish.completed")
		if err == nil {
			log.Infof("launcher: published '%v'", event)
			break
		}

		switch err {
		case mangos.ErrRecvTimeout:
			log.Debug("launcher: publish event timeout... ok")
		default:
			log.Warnln("launcher: publish event error =>", err)
		}
	}
}

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}
