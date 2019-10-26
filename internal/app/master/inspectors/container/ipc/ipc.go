package ipc

import (
	"encoding/json"
	"fmt"
	"os"
	//"time"

	//"github.com/go-mangos/mangos"
	//"github.com/go-mangos/mangos/protocol/req"
	//"github.com/go-mangos/mangos/protocol/sub"

	//"nanomsg.org/go/mangos/v2"
	//"nanomsg.org/go/mangos/v2/protocol/sub"

	//"nanomsg.org/go/mangos/v2/protocol"
	//"nanomsg.org/go/mangos/v2/protocol/req"
	//_ "nanomsg.org/go/mangos/v2/transport/tcp"

	log "github.com/Sirupsen/logrus"

	//"github.com/go-mangos/mangos/transport/ipc"
	//"github.com/go-mangos/mangos/transport/tcp"

	"github.com/docker-slim/docker-slim/pkg/ipc/channel"
	"github.com/docker-slim/docker-slim/pkg/ipc/command"
	"github.com/docker-slim/docker-slim/pkg/ipc/event"
)

const (
	connectTimeout = 15
	readTimeout    = 30
	writeTimeout   = 30
)

type Client struct {
	connectWait int
	target      string
	cmdPort     string
	evtPort     string
	evtChannel  *channel.EventClient
	cmdChannel  *channel.CommandClient
}

func NewClient(target, cmdChannelPort, evtChannelPort string, connectWait int) (*Client, error) {
	log.Debugf("ipc.NewClient(%s,%s,%s)", target, cmdChannelPort, evtChannelPort)
	client := Client{
		target:      target,
		cmdPort:     cmdChannelPort,
		evtPort:     evtChannelPort,
		connectWait: connectWait,
	}

	if client.cmdPort == "" {
		client.cmdPort = fmt.Sprintf("%d", channel.CmdPort)
	}

	if client.evtPort == "" {
		client.evtPort = fmt.Sprintf("%d", channel.EvtPort)
	}

	if err := client.initChannels(); err != nil {
		log.Errorf("ipc.NewClient init error = %v", err)
		return nil, err
	}

	return &client, nil
}

func (c *Client) initChannels() error {
	cmdChannelAddr := fmt.Sprintf("%s:%s", c.target, c.cmdPort)
	cmdChannel, err := channel.NewCommandClient(cmdChannelAddr, c.connectWait, connectTimeout, readTimeout, writeTimeout)
	if os.IsTimeout(err) {
		log.Debug("ipc.initChannels(): connect timeout...")
		return err
	} else if err != nil {
		return err
	}

	c.cmdChannel = cmdChannel

	evtChannelAddr := fmt.Sprintf("%s:%s", c.target, c.evtPort)
	evtChannel, err := channel.NewEventClient(evtChannelAddr, c.connectWait, connectTimeout, -1)
	if os.IsTimeout(err) {
		log.Debug("ipc.initChannels(): connect timeout...")
		return err
	} else if err != nil {
		return err
	}

	c.evtChannel = evtChannel

	return nil
}

func (c *Client) shutdownChannels() error {
	if c.cmdChannel != nil {
		c.cmdChannel.Close()
		c.cmdChannel = nil
	}

	if c.evtChannel != nil {
		c.evtChannel.Close()
		c.evtChannel = nil
	}

	return nil
}

func (c *Client) Stop() error {
	return c.shutdownChannels()
}

func (c *Client) SendCommand(cmd command.Message) (*command.Response, error) {
	reqData, err := command.Encode(cmd)
	if err != nil {
		log.Error("ipc.Client.SendCommand(): malformed cmd - ", err)
		return nil, err
	}

	log.Debugf("ipc.Client.SendCommand() cmd channel call data='%s'\n", string(reqData))
	respData, err := c.cmdChannel.Call(reqData, 3)
	if err != nil {
		log.Errorf("ipc.Client.SendCommand() cmd channel call error=%v\n", err)
		return nil, err
	}

	log.Debugf("ipc.Client.SendCommand() cmd channel call response='%s'\n", string(respData))
	if len(respData) == 0 {
		log.Info("ipc.Client.SendCommand() no cmd channel call response (closed connection)")
		return nil, nil
	}

	var resp command.Response
	err = json.Unmarshal(respData, &resp)
	if err != nil {
		log.Error("ipc.Client.SendCommand(): malformed cmd response - ", err)
		return nil, err
	}

	return &resp, nil
}

func (c *Client) GetEvent() (*event.Message, error) {
	raw, id, err := c.evtChannel.Next(3)
	if err != nil {
		log.Errorf("ipc.Client.GetEvent(): event channel error = %v\n", err)
		return nil, err
	}

	log.Debugf("ipc.Client.GetEvent(): channel.Recv() - done [tid=%s evt=%v]\n", id, string(raw))
	if len(raw) == 0 {
		log.Info("ipc.Client.GetEvent() no event channel data (closed connection)")
		return nil, nil
	}

	var evt event.Message
	if err := json.Unmarshal(raw, &evt); err != nil {
		log.Errorf("ipc.Client.GetEvent(): malformed event = %v\n", err)
		return nil, err
	}

	return &evt, nil
}

/*
// InitContainerChannels initializes the communication channels with the target container
func InitContainerChannels(dockerHostIP, cmdChannelPort, evtChannelPort string) error {
	cmdChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIP, cmdChannelPort)
	evtChannelAddr = fmt.Sprintf("tcp://%v:%v", dockerHostIP, evtChannelPort)
	log.Debugf("cmdChannelAddr=%v evtChannelAddr=%v", cmdChannelAddr, evtChannelAddr)

	//evtChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-sensor.events.ipc", localVolumePath)
	//cmdChannelAddr = fmt.Sprintf("ipc://%v/ipc/docker-slim-sensor.cmds.ipc", localVolumePath)
	fmt.Println("QTMP: InitContainerChannels() - newEvtChannel()")
	var err error
	evtChannel, err = newEvtChannel(evtChannelAddr)
	if err != nil {
		return err
	}

	fmt.Println("QTMP: InitContainerChannels() - newEvtChannel()")
	cmdChannel, err = newCmdClient(cmdChannelAddr)
	if err != nil {
		return err
	}

	return nil
}
*/

/*
// SendContainerCmd sends the given command to the target container
func SendContainerCmd(cmd command.Message) (string, error) {
	return sendCmd(cmdChannel, cmd)
}
*/

/*
// GetContainerEvt returns the current event generated by the target container
func GetContainerEvt() (*event.Message, error) {
	return getEvt(evtChannel)
}
*/

/*
// ShutdownContainerChannels destroys the communication channels with the target container
func ShutdownContainerChannels() {
	shutdownEvtChannel()
	shutdownCmdChannel()
}
*/

/*
//var cmdChannelAddr = "ipc:///tmp/docker-slim-sensor.cmds.ipc"
var cmdChannelAddr = fmt.Sprintf("tcp://127.0.0.1:%d", channel.CmdPort)

var cmdChannel mangos.Socket

func newCmdClient(addr string) (mangos.Socket, error) {
	socket, err := req.NewSocket()
	if err != nil {
		return nil, err
	}
	fmt.Printf("TMP: newCmdClient() - socket.SetOption(mangos.OptionSendDeadline, time.Second*3)\n")
	if err := socket.SetOption(mangos.OptionSendDeadline, time.Second*3); err != nil {
		fmt.Printf("TMP: newCmdClient() - socket.SetOption(mangos.OptionSendDeadline, time.Second*3) - error = %v\n", err)
		socket.Close()
		return nil, err
	}
	fmt.Printf("TMP: newCmdClient() - socket.SetOption(mangos.OptionRecvDeadline, time.Second*3)\n")
	if err := socket.SetOption(mangos.OptionRecvDeadline, time.Second*3); err != nil {
		fmt.Printf("TMP: newCmdClient() - socket.SetOption(mangos.OptionRecvDeadline, time.Second*3) - error = %v\n", err)
		socket.Close()
		return nil, err
	}

	fmt.Printf("TMP: newCmdClient() - options set...\n")

	//socket.AddTransport(ipc.NewTransport())
	//socket.AddTransport(tcp.NewTransport())
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
*/
/*
func sendCmd(channel mangos.Socket, cmd command.Message) (string, error) {
	sendTimeouts := 0
	recvTimeouts := 0

	log.Debugf("sendCmd(%+v)", cmd)
	for {
		sendData, err := command.Encode(cmd)
		if err != nil {
			log.Info("sendCmd(): malformed cmd - ", err)
			return "", err
		}

		if err := channel.Send(sendData); err != nil {
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

		if response, err := channel.Recv(); err != nil {
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
		} else {
			return string(response), nil
		}
	}
}
*/

/*
var evtChannelAddr = fmt.Sprintf("tcp://127.0.0.1:%d", channel.EvtPort)

//var evtChannelAddr = "ipc:///tmp/docker-slim-sensor.events.ipc"
var evtChannel mangos.Socket

func newEvtChannel(addr string) (mangos.Socket, error) {
	socket, err := sub.NewSocket()
	if err != nil {
		log.Debugf("newEvtChannel(%v): sub.NewSocket() error = %v\n", addr, err)
		return nil, err
	}
	fmt.Printf("QTMPx: newEvtChannel() - socket.SetOption(mangos.OptionRecvDeadline, time.Second*120)\n")
	if err = socket.SetOption(mangos.OptionRecvDeadline, time.Second*120); err != nil {
		log.Debugf("newEvtChannel(): socket.SetOption(%v) error = %v\n", mangos.OptionRecvDeadline, err)
		fmt.Printf("QTMPx: newEvtChannel() - socket.SetOption(mangos.OptionRecvDeadline, time.Second*120) - error = %v\n", err)
		socket.Close()
		return nil, err
	}

	fmt.Printf("QTMPx: newEvtChannel() - CALLING socket.Dial(%v)\n", addr)
	//socket.AddTransport(ipc.NewTransport())
	//socket.AddTransport(tcp.NewTransport())
	if err = socket.Dial(addr); err != nil {
		fmt.Printf("QTMPx: newEvtChannel() - socket.Dial(%v) - error = %v\n", addr, err)
		log.Debugf("newEvtChannel(): socket.Dial(%v) error = %v\n", addr, err)
		socket.Close()
		return nil, err
	}

	//fmt.Printf("TMP: newEvtChannel() - socket.SetOption(mangos.OptionSubscribe)\n")
	//if err = socket.SetOption(mangos.OptionSubscribe, []byte("")); err != nil {
	//	fmt.Printf("TMP: newEvtChannel() - socket.SetOption(mangos.OptionSubscribe) - error = %v\n", err)
	//	return nil, err
	//}

	fmt.Printf("TMPx: newEvtChannel() - options set...\n")

	return socket, nil
}

func shutdownEvtChannel() {
	if evtChannel != nil {
		evtChannel.Close()
		evtChannel = nil
	}
}
*/

/*
func getEvt(channel mangos.Socket) (*event.Message, error) {
	log.Debug("getEvt()")
	data, err := channel.Recv()
	log.Debugf("getEvt(): channel.Recv() - done [evt=%v,err=%v]", string(data), err)
	if err != nil {
		return nil, err
	}

	var msg event.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}
*/
