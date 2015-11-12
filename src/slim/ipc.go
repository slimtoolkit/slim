package main

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/req"
	"github.com/gdamore/mangos/protocol/sub"
	//"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
)

//var cmdChannelAddr = "ipc:///tmp/docker-slim-launcher.cmds.ipc"
var cmdChannelAddr = "tcp://127.0.0.1:65501"
var cmdChannel mangos.Socket

func newCmdClient(addr string) (mangos.Socket, error) {
	socket, err := req.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionSendDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline, time.Second*3); err != nil {
		socket.Close()
		return nil, err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}

	return socket, nil
}

func shutdownCmdChannel() {
	if cmdChannel != nil {
		cmdChannel.Close()
		cmdChannel = nil
	}
}

func sendCmd(channel mangos.Socket, cmd string) (string, error) {
	sendTimeouts := 0
	recvTimeouts := 0

	log.Debugf("sendCmd(%s)\n", cmd)
	for {
		if err := channel.Send([]byte(cmd)); err != nil {
			switch err {
			case mangos.ErrSendTimeout:
				log.Info("sendCmd(): send timeout...")
				sendTimeouts++
				if sendTimeouts > 3 {
					return "", err
				}
			default:
				return "", err
			}
		}

		response, err := channel.Recv()
		if err != nil {
			switch err {
			case mangos.ErrRecvTimeout:
				log.Info("sendCmd(): receive timeout...")
				recvTimeouts++
				if recvTimeouts > 3 {
					return "", err
				}
			default:
				return "", err
			}
		}

		return string(response), nil
	}
}

var evtChannelAddr = "tcp://127.0.0.1:65502"

//var evtChannelAddr = "ipc:///tmp/docker-slim-launcher.events.ipc"
var evtChannel mangos.Socket

func newEvtChannel(addr string) (mangos.Socket, error) {
	socket, err := sub.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := socket.SetOption(mangos.OptionRecvDeadline,time.Second * 120); err != nil {
		socket.Close()
		return nil,err
	}

	//socket.AddTransport(ipc.NewTransport())
	socket.AddTransport(tcp.NewTransport())
	if err := socket.Dial(addr); err != nil {
		socket.Close()
		return nil, err
	}

	err = socket.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return nil, err
	}

	return socket, nil
}

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}

func getEvt(channel mangos.Socket) (string, error) {
	log.Debug("getEvt()")
	evt, err := channel.Recv()
	if err != nil {
		return "", err
	}

	return string(evt), nil
}
