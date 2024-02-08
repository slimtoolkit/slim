package http

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/slimtoolkit/slim/pkg/acounter"
)

const (
	ProtoWS  = "ws"
	ProtoWSS = "wss"
)

type WebsocketClient struct {
	OnRead    func(mtype int, mdata []byte)
	ReadCh    chan WebsocketMessage
	Conn      *websocket.Conn
	ReadCount acounter.Type
	PongCount acounter.Type
	PingCount acounter.Type
	Addr      string
	pongCh    chan string
	doneCh    chan struct{}
}

type WebsocketMessage struct {
	Type int
	Data []byte
}

func NewWebsocketClient(proto, host, port string) (*WebsocketClient, error) {
	if proto == "" {
		proto = ProtoWS
	}

	if !IsValidWSProto(proto) {
		return nil, fmt.Errorf("invalid ws proto - %s", proto)
	}

	wsclient := &WebsocketClient{
		Addr:   fmt.Sprintf("%s://%s:%s", proto, host, port),
		doneCh: make(chan struct{}),
		pongCh: make(chan string, 10),
	}

	return wsclient, nil
}

func IsValidWSProto(proto string) bool {
	switch proto {
	case ProtoWS, ProtoWSS:
		return true
	default:
		return false
	}
}

func (wc *WebsocketClient) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(wc.Addr, nil)
	if err != nil {
		log.Debugf("WebsocketClient.Connect: ws.Dial error=%v", err)
		return err
	}

	pongHandler := func(data string) error {
		wc.PongCount.Inc()
		log.Debugf("WebsocketClient: Pong Handler - data='%s'", data)
		wc.pongCh <- data
		return nil
	}
	conn.SetPongHandler(pongHandler)

	defaultPingHandler := conn.PingHandler()
	pingHandler := func(data string) error {
		wc.PingCount.Inc()
		log.Debugf("WebsocketClient: Ping Handler - data='%s'", data)
		return defaultPingHandler(data)
	}
	conn.SetPingHandler(pingHandler)

	go func() {
		for {
			log.Debug("WebsocketClient: reader - waiting for errors...")
			select {
			case <-wc.doneCh:
				log.Debug("WebsocketClient: reader - error collector - done...")
				return
			default:
				mtype, message, err := conn.ReadMessage()
				if err != nil {
					log.Debugf("WebsocketClient: reader - read error=%v", err)
					//read err: websocket: close 1000 (normal)
					return
				}

				log.Debugf("WebsocketClient: reader - read={type=%s(%d) len=%d data='%s'}\n", wsMessageType(mtype), mtype, len(message), message)
				wc.ReadCount.Inc()
				if wc.OnRead != nil {
					wc.OnRead(mtype, message)
				}

				if wc.ReadCh != nil {
					select {
					case wc.ReadCh <- WebsocketMessage{Type: mtype, Data: message}:
						log.Debug("WebsocketClient: reader - posted message to wc.ReadCh")
					case <-time.After(time.Second * 5):
						log.Debug("WebsocketClient: reader - timed out posting message to wc.ReadCh")
					}
				}
			}
		}
	}()

	wc.Conn = conn
	return nil
}

func wsMessageType(val int) string {
	switch val {
	case websocket.TextMessage:
		return "text"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.CloseMessage:
		return "close"
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	default:
		return fmt.Sprintf("unknown(%d)", val)
	}
}

func (wc *WebsocketClient) CheckConnection() error {
	log.Debugf("WebsocketClient.CheckConnection: conn.WriteControl(websocket.PingMessage)")

	err := wc.Conn.WriteControl(websocket.PingMessage, []byte("wc.ping"), time.Now().Add(3*time.Second))
	if err != nil {
		log.Debugf("WebsocketClient.CheckConnection: conn.WriteControl(websocket.PingMessage) error=%v", err)
		return err
	}

	select {
	case pongData := <-wc.pongCh:
		log.Debugf("WebsocketClient.CheckConnection: pong data = %v", pongData)
	case <-time.After(time.Second * 5):
		log.Debug("WebsocketClient.CheckConnection: pong data timeout")
	}

	return nil
}

func (wc *WebsocketClient) WriteString(data string) error {
	err := wc.Conn.WriteMessage(websocket.TextMessage, []byte(data))
	if err != nil {
		log.Debugf("WebsocketClient.WriteString: conn.WriteMessage(websocket.TextMessage) error=%v", err)
		return err
	}

	return nil
}

func (wc *WebsocketClient) WriteBinary(data []byte) error {
	err := wc.Conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		log.Debugf("WebsocketClient.WriteString: conn.WriteMessage(websocket.BinaryMessage) error=%v", err)
		return err
	}

	return nil
}

func (wc *WebsocketClient) Disconnect() error {
	if wc.Conn != nil {
		close(wc.doneCh)
		wc.doneCh = nil

		err := wc.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Debugf("WebsocketClient.Disconnect: conn.WriteMessage(websocket.CloseMessage) error=%v", err)
			return err
		}

		//TODO: should wait for the 'websocket: close 1000 (normal)' read error
		time.Sleep(2 * time.Second)

		wc.Conn.Close()
		wc.Conn = nil
	}

	return nil
}
