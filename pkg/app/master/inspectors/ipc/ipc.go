package ipc

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/ipc/channel"
	"github.com/slimtoolkit/slim/pkg/ipc/command"
	"github.com/slimtoolkit/slim/pkg/ipc/event"
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
