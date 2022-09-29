package channel

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	ErrNoData           = errors.New("no data")
	ErrFrameTIDMismatch = errors.New("frame TID mismatch")
	ErrFrameUnexpected  = errors.New("unexpected frame type")
	ErrRemoteError      = errors.New("remote error")
	ErrWaitTimeout      = errors.New("wait timeout")
	ErrFrameMalformed   = errors.New("malformed frame")
)

// Channel ports
const (
	CmdPort = 65501
	EvtPort = 65502
)

const (
	proto                         = "tcp"
	defaultConnectTimeoutDuration = 11 * time.Second
	defaultReadTimeoutDuration    = 11 * time.Second
	defaultWriteTimeoutDuration   = 11 * time.Second
	msgEndByte                    = '\n'
)

var (
	frameHeader  = []byte("[<|]")
	frameTrailer = []byte("[|>]\n")
)

// FrameType is a Frame struct type
type FrameType string

// Supported frame types
const (
	RequestFrameType  FrameType = "ft.request"
	ResponseFrameType FrameType = "ft.response"
	EventFrameType    FrameType = "ft.event"
	ErrorFrameType    FrameType = "ft.error"
	ControlFrameType  FrameType = "ft.control"
)

type Frame struct {
	TID  string          `json:"tid"`
	Type FrameType       `json:"type"`
	Body json.RawMessage `json:"body,omitempty"`
}

func newFrame(ftype FrameType, data []byte, tid string) *Frame {
	frame := Frame{
		Type: ftype,
		Body: data,
	}

	if tid != "" {
		frame.TID = tid
	} else {
		frame.TID = GenerateTID()
	}

	return &frame
}

func createFrameBytes(frame *Frame) ([]byte, error) {
	raw, err := json.Marshal(frame)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.Write(frameHeader)
	b.Write(raw)
	b.Write(frameTrailer)

	return b.Bytes(), nil
}

func createFrameBytesFromFields(ftype FrameType, data []byte, tid string) ([]byte, error) {
	frame := newFrame(ftype, data, tid)
	return createFrameBytes(frame)
}

func GenerateTID() string {
	now := time.Now().UnixNano()
	random := make([]byte, 8)
	rand.Read(random)
	return fmt.Sprintf("%d.%s", now, hex.EncodeToString(random))
}

func getFrame(raw []byte) (*Frame, error) {
	if len(raw) > (len(frameHeader)+len(frameTrailer)) && bytes.HasPrefix(raw, frameHeader) && bytes.HasSuffix(raw, frameTrailer) {
		data := raw[len(frameHeader) : len(raw)-len(frameTrailer)]

		var frame Frame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}

		return &frame, nil
	}

	return nil, ErrFrameMalformed
}

type ConnectionHandler interface {
	OnConnection(conn net.Conn)
}

type Server struct {
	addr     string
	listener net.Listener
	handler  ConnectionHandler
}

func NewServer(addr string) *Server {
	server := Server{
		addr: addr,
	}

	return &server
}

func (s *Server) SetConnHandler(handler ConnectionHandler) {
	s.handler = handler
}

func (s *Server) Start(async bool) error {
	var err error
	log.Debugf("channel.Server.Start() - addr=%v [time=%v]", s.addr, time.Now().UnixNano())
	s.listener, err = net.Listen(proto, s.addr)
	if err != nil {
		log.Debugf("channel:Server.Start() - net.Listen error = %v", err)
		return err
	}

	loop := func() {
		log.Debugf("channel.Server.Start.loop()... [time=%v]", time.Now().UnixNano())
		for {
			conn, err := s.listener.Accept()
			log.Debugf("channel.Server.Start.loop.Accept - new connection... [time=%v]", time.Now().UnixNano())

			if err != nil {
				log.Errorf("channel.Server.Start() - loop.Accept error = %v", err)
				return
			}

			log.Debugf("channel.Server.Start.loop(): new connection = %s -> %s", conn.RemoteAddr(), conn.LocalAddr())

			if s.handler != nil {
				log.Debug("channel.Server.Start.loop(): new connection - call handler...")
				s.handler.OnConnection(conn)
			}
		}
	}

	if async {
		go loop()
	} else {
		loop()
	}

	return nil
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

type EventServer struct {
	*Server

	mu    sync.Mutex
	links []net.Conn

	pending chan struct{}
}

func NewEventServer(addr string) *EventServer {
	server := &EventServer{
		Server:  NewServer(addr),
		pending: make(chan struct{}),
	}

	server.SetConnHandler(server)
	return server
}

func (s *EventServer) OnConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.links = append(s.links, conn)

	select {
	case <-s.pending: // Has already been closed
		break
	default:
		close(s.pending)
	}
}

func (s *EventServer) WaitForConnection() {
	<-s.pending
}

func (s *EventServer) Publish(data []byte, retries uint) error {
	if len(data) == 0 {
		return nil
	}

	frame, err := createFrameBytesFromFields(EventFrameType, data, "")
	if err != nil {
		return err
	}

	for _, conn := range s.links {
		timeouts := uint(0)
		for {
			conn.SetWriteDeadline(time.Now().Add(defaultWriteTimeoutDuration))
			n, err := conn.Write(frame)
			log.Debugf("channel.Broadcast.Write: %s -> %s - conn.Write wc=%v err=%v", conn.RemoteAddr(), conn.LocalAddr(), n, err)
			if err == nil {
				break
			}

			if os.IsTimeout(err) {
				log.Debugf("channel.Broadcast.Write: %s -> %s - write timeout...", conn.RemoteAddr(), conn.LocalAddr())
				timeouts++
				if retries > 0 && timeouts > retries {
					break
				}
			} else if err != nil {
				log.Errorf("channel.Broadcast.Write: %s -> %s - write error = %v", conn.RemoteAddr(), conn.LocalAddr(), err)
				break
			}
		}
	}

	return nil
}

type Client struct {
	addr         string
	conn         net.Conn
	reader       *bufio.Reader
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func durationValue(vparam int, vdefault time.Duration) time.Duration {
	val := vdefault

	switch {
	case vparam > 0:
		val = time.Duration(vparam) * time.Second
	case vparam < 0:
		val = 0
	}

	return val
}

func NewClient(addr string, connectWait, connectTimeout, readTimeout, writeTimeout int) (*Client, error) {
	cwd := durationValue(connectWait, 0)
	ctd := durationValue(connectTimeout, defaultConnectTimeoutDuration) //todo: use non-timeout net.Dial or net.DialContext
	rtd := durationValue(readTimeout, defaultReadTimeoutDuration)
	wtd := durationValue(writeTimeout, defaultWriteTimeoutDuration)

	client := Client{
		addr:         addr,
		readTimeout:  rtd,
		writeTimeout: wtd,
	}

	var timeout <-chan time.Time
	if cwd > 0 {
		log.Debugf("channel.NewClient: connect wait timeout - %v", cwd)
		timeout = time.After(cwd)
	}

	connectStart := time.Now()
done:
	for {
		select {
		case <-timeout:
			log.Debugf("channel.NewClient: connect wait timeout (waited=%v)", time.Since(connectStart))
			return nil, ErrWaitTimeout
		default:
			start := time.Now()
			var err error
			if ctd != 0 {
				log.Debugf("channel.NewClient: net.DialTimeout(%v,%v,%v) [time=%v]", proto, addr, ctd, time.Now().UnixNano())
				client.conn, err = net.DialTimeout(proto, addr, ctd)
			} else {
				log.Debugf("channel.NewClient: net.Dial(%v,%v)", proto, addr)
				client.conn, err = net.Dial(proto, addr)
			}

			if err == nil {
				break done
			}

			log.Debugf("channel.NewClient: (dial time = %v) - connect error = %v", time.Since(start), err)

			if connectWait < 1 {
				return nil, err
			}

			log.Debug("channel.NewClient: waiting before trying to connect again...")
			time.Sleep(2 * time.Second)
		}
	}

	client.reader = bufio.NewReader(client.conn)

	return &client, nil
}

func (c *Client) Write(frame *Frame, retries uint) (n int, err error) {
	frameBytes, e := createFrameBytes(frame)
	if err != nil {
		return 0, e
	}

	timeouts := uint(0)

	for {
		if c.writeTimeout != 0 {
			c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		}

		n, err = c.conn.Write(frameBytes)
		log.Debugf("channel.Client.Write: remote=%s -> local=%s - conn.Write wc=%v err=%v [time=%v]", c.conn.RemoteAddr(), c.conn.LocalAddr(), n, err, time.Now().UnixNano())
		if err == nil {
			return n, nil
		}

		if os.IsTimeout(err) {
			log.Debugf("channel.Client.Write: %s -> %s - write timeout...", c.conn.RemoteAddr(), c.conn.LocalAddr())
			timeouts++
			if retries > 0 && timeouts > retries {
				return
			}
		} else if err != nil {
			log.Errorf("channel.Client.Write: %s -> %s - write error = %v", c.conn.RemoteAddr(), c.conn.LocalAddr(), err)
			return
		}
	}
}

func (c *Client) Read(retries uint) (*Frame, error) {
	timeouts := uint(0)

	var raw string
	for {
		var err error
		if c.readTimeout != 0 {
			c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		}
		raw, err = c.reader.ReadString(msgEndByte)
		log.Debugf("channel.Client.Read() - (%p)reader.ReadString => err=%v raw=%#v", c.reader, err, raw)

		if err != nil {
			if err == io.EOF {
				log.Debug("channel.Client.Read: connection done...")
				return nil, err
			} else if os.IsTimeout(err) {
				log.Debug("channel.Client.Read: read timeout...")
				timeouts++
				if retries > 0 && timeouts > retries {
					return nil, err
				}

				continue
			} else {
				log.Errorf("channel.Client.Read: read error (%v), exiting...", err)
				return nil, err
			}
		}

		log.Debugf("channel.Client.Read: got raw frame ='%s'", raw)
		break
	}

	frame, err := getFrame([]byte(raw))
	if err != nil {
		log.Infof("channel.Client.Read: malformed frame (%v) ='%s'", len(raw), raw)
		return nil, err
	}

	if frame != nil {
		log.Debugf("channel.Client.Read: frame data => tid=%s type=%s body='%s'",
			frame.TID, frame.Type, string(frame.Body))
	} else {
		log.Infof("channel.Client.Read: malformed frame (%v) ='%s'", len(raw), raw)
	}

	return frame, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type EventClient struct {
	*Client
}

func NewEventClient(addr string, connectWait, connectTimeout, readTimeout int) (*EventClient, error) {
	client, err := NewClient(addr, connectWait, connectTimeout, readTimeout, -1)
	if err != nil {
		log.Errorf("channel.NewSubscriber: NewClient error = %v", err)
		return nil, err
	}

	eventClient := &EventClient{
		Client: client,
	}

	return eventClient, nil
}

func (c *EventClient) Next(retries uint) ([]byte, string, error) {
	frame, err := c.Read(retries)
	if err != nil {
		return nil, "", err
	}

	if frame == nil {
		log.Info("channel.EventClient.Next: c.Read() - no data, closed connection...")
		return nil, "", nil
	}

	return frame.Body, frame.TID, nil
}

type CommandClient struct {
	*Client
}

func NewCommandClient(addr string, connectWait, connectTimeout, readTimeout, writeTimeout int) (*CommandClient, error) {
	cwd := durationValue(connectWait, 0)
	var timeout <-chan time.Time
	if cwd > 0 {
		timeout = time.After(cwd)
	}

	connectStart := time.Now()
	for {
		select {
		case <-timeout:
			log.Debugf("channel.NewCommandClient: connect wait timeout (waited=%v)...", time.Since(connectStart))
			return nil, ErrWaitTimeout
		default:
			client, err := NewClient(addr, connectWait, connectTimeout, readTimeout, writeTimeout)
			if err != nil {
				log.Errorf("channel.NewCommandClient: NewClient error = %v", err)
				return nil, err
			}

			err = verifyCommandChannel(client, 3)
			if err == nil {
				cmdClient := &CommandClient{
					Client: client,
				}

				return cmdClient, nil
			}

			if errors.Is(err, io.EOF) {
				log.Debug("channel.NewCommandClient: closed connection.")
			} else {
				log.Errorf("channel.NewCommandClient: channel verify error = %v", err)
			}

			if connectWait < 1 {
				return nil, err
			}

			log.Debugf("channel.NewCommandClient: waiting before trying to connect again...")
			time.Sleep(5 * time.Second)
		}
	}
}

func verifyCommandChannel(client *Client, retries uint) error {
	reqFrame := newFrame(ControlFrameType, nil, "")

	_, err := client.Write(reqFrame, retries)
	if err != nil {
		log.Errorf("verifyCommandChannel: client.Write error = %v", err)
		return err
	}

	replyFrame, err := client.Read(retries)
	if err != nil {
		log.Debugf("verifyCommandChannel: client.Read error = %v", err)
		return err
	}

	if replyFrame == nil {
		log.Info("verifyCommandChannel: client.Read() - no data")
		return ErrFrameUnexpected
	}

	log.Debugf("verifyCommandChannel: client.Read() (tid=%v type=%v) result.data='%s'",
		replyFrame.TID, replyFrame.Type, string(replyFrame.Body))

	if replyFrame.Type == ErrorFrameType {
		return ErrRemoteError
	}

	if replyFrame.Type != ControlFrameType {
		return ErrFrameUnexpected
	}

	if reqFrame.TID != replyFrame.TID {
		log.Errorf("verifyCommandChannel: frame TID mismatch  %s = %s", reqFrame.TID, replyFrame.TID)
		return ErrFrameTIDMismatch
	}

	return nil
}

func (c *CommandClient) Call(data []byte, retries uint) ([]byte, error) {
	log.Debug("channel.CommandClient.Call: calling c.Write()")
	if len(data) == 0 {
		return nil, ErrNoData
	}

	reqFrame := newFrame(RequestFrameType, data, "")
	_, err := c.Write(reqFrame, retries)
	if err != nil {
		log.Errorf("channel.CommandClient.Call: c.Write error = %v", err)
		return nil, err
	}

	log.Debugf("channel.CommandClient.Call: calling c.Read()")
	replyFrame, err := c.Read(retries)
	if err != nil {
		log.Errorf("channel.CommandClient.Call: c.Read error = %v", err)
		return nil, err
	}

	if replyFrame == nil {
		log.Info("channel.CommandClient.Call: c.Read() - no data, closed connection...")
		return nil, nil
	}

	log.Debugf("channel.CommandClient.Call: c.Read() (tid=%v type=%v) result.data='%s'",
		replyFrame.TID, replyFrame.Type, string(replyFrame.Body))

	if replyFrame.Type == ErrorFrameType {
		return nil, ErrRemoteError
	}

	if replyFrame.Type != ResponseFrameType {
		return nil, ErrFrameUnexpected
	}

	if reqFrame.TID != replyFrame.TID {
		log.Errorf("channel.CommandClient.Call: frame TID mismatch  %s = %s", reqFrame.TID, replyFrame.TID)
		return nil, ErrFrameTIDMismatch
	}

	return replyFrame.Body, nil
}

type RequestHandler interface {
	OnRequest(data []byte) ([]byte, error)
}

type CommandServer struct {
	*Server
	handler RequestHandler

	pending chan struct{}
}

func NewCommandServer(addr string, handler RequestHandler) *CommandServer {
	server := &CommandServer{
		Server:  NewServer(addr),
		pending: make(chan struct{}),
	}

	server.SetConnHandler(server)
	server.SetReqHandler(handler)
	return server
}

func (s *CommandServer) SetReqHandler(handler RequestHandler) {
	s.handler = handler
}

func (s *CommandServer) WaitForConnection() {
	<-s.pending
}

func (s *CommandServer) OnConnection(conn net.Conn) {
	log.Debugf("channel.CommandServer.OnConnection: %s -> %s", conn.RemoteAddr(), conn.LocalAddr())

	select {
	case <-s.pending: // Has already been closed
		break
	default:
		close(s.pending)
	}

	go func() {
		defer func() {
			log.Debug("channel.CommandServer.OnConnection.worker: closing connection...")
			conn.Close()
		}()

		reader := bufio.NewReader(conn)
		for {
			conn.SetReadDeadline(time.Now().Add(defaultReadTimeoutDuration))
			inRaw, err := reader.ReadString(msgEndByte)
			if err == io.EOF {
				log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - connection done...", conn.RemoteAddr(), conn.LocalAddr())
				return
			} else if os.IsTimeout(err) {
				log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - read timeout...", conn.RemoteAddr(), conn.LocalAddr())
				continue
			} else if err != nil {
				log.Errorf("channel.CommandServer.OnConnection.worker: %s -> %s - read error (%v)", conn.RemoteAddr(), conn.LocalAddr(), err)
				return
			}

			if s.handler != nil {
				log.Debugf("channel.CommandServer.OnConnection.worker: raw frame => '%s'", inRaw)
				inFrame, err := getFrame([]byte(inRaw))
				if err != nil {
					log.Errorf("channel.CommandServer.OnConnection.worker: %s -> %s - error getting frame (%v) [raw(%v)='%s']",
						conn.RemoteAddr(), conn.LocalAddr(), err, len(inRaw), inRaw)
					return
				}

				if inFrame == nil {
					log.Errorf("channel.CommandServer.OnConnection.worker: %s -> %s - no frame", conn.RemoteAddr(), conn.LocalAddr())
					continue
				}

				log.Debugf("channel.CommandServer.OnConnection.worker: in frame => tid=%v type=%v body='%s'",
					inFrame.TID, inFrame.Type, string(inFrame.Body))

				var outFrame []byte
				if inFrame.Type == ControlFrameType {
					outFrame, err = createFrameBytesFromFields(ControlFrameType, nil, inFrame.TID)
					if err != nil {
						log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - error creating control out frame (%v)", conn.RemoteAddr(), conn.LocalAddr(), err)
						return
					}
				} else {
					outData, err := s.handler.OnRequest(inFrame.Body)
					if err != nil {
						log.Errorf("channel.CommandServer.OnConnection.worker: handler.OnRequest error => %v", err)
						outFrame, err = createFrameBytesFromFields(ErrorFrameType, nil, inFrame.TID)
						if err != nil {
							log.Errorf("channel.CommandServer.OnConnection.worker: %s -> %s - error creating out frame (%v)", conn.RemoteAddr(), conn.LocalAddr(), err)
							return
						}
					} else {
						outFrame, err = createFrameBytesFromFields(ResponseFrameType, outData, inFrame.TID)
						if err != nil {
							log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - error creating out frame (%v)", conn.RemoteAddr(), conn.LocalAddr(), err)
							return
						}
					}
				}

				for {
					conn.SetWriteDeadline(time.Now().Add(defaultWriteTimeoutDuration))
					wc, err := conn.Write(outFrame)
					//todo handle timeouts properly
					log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - conn.Write wc=%v err=%v", conn.RemoteAddr(), conn.LocalAddr(), wc, err)
					if err == nil {
						log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - replied with frame='%s'", conn.RemoteAddr(), conn.LocalAddr(), string(outFrame))
						break
					}

					if os.IsTimeout(err) {
						log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - write timeout (trying again)...", conn.RemoteAddr(), conn.LocalAddr())
					} else if err == io.EOF {
						log.Debugf("channel.CommandServer.OnConnection.worker: %s -> %s - connection done (worker exiting)...", conn.RemoteAddr(), conn.LocalAddr())
						return
					} else if err != nil {
						log.Errorf("channel.CommandServer.OnConnection.worker: %s -> %s - write error = %v (worker exiting)", conn.RemoteAddr(), conn.LocalAddr(), err)
						return
					}
				}
			}
		}
	}()
}
