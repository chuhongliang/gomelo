package gomelo

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type MessageType uint8

const (
	TypeRequest  MessageType = 1
	TypeResponse MessageType = 2
	TypeNotify   MessageType = 3
	TypeError    MessageType = 4
)

type ClientOptions struct {
	Host                 string
	Port                 int
	Timeout              time.Duration
	HeartbeatInterval    time.Duration
	ReconnectInterval    time.Duration
	MaxReconnectAttempts int
}

type RequestCallback struct {
	Resolve func(interface{})
	Reject  func(error)
	Timer   *time.Timer
}

type EventHandler struct {
	Callback func(interface{})
	Target   interface{}
}

type Client struct {
	opts ClientOptions

	conn      net.Conn
	connected atomic.Bool
	closed    atomic.Bool

	seq     atomic.Uint64
	pending sync.Map

	eventHandlers sync.Map

	writeCh chan []byte
	doneCh  chan struct{}
	wg      sync.WaitGroup

	routeToID sync.Map
	idToRoute sync.Map
}

func NewClient(opts ClientOptions) *Client {
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 30 * time.Second
	}
	if opts.ReconnectInterval == 0 {
		opts.ReconnectInterval = 3 * time.Second
	}
	if opts.MaxReconnectAttempts == 0 {
		opts.MaxReconnectAttempts = 5
	}

	return &Client{
		opts:    opts,
		writeCh: make(chan []byte, 256),
		doneCh:  make(chan struct{}),
	}
}

func (c *Client) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.opts.Host, c.opts.Port)

	conn, err := net.DialTimeout("tcp", addr, c.opts.Timeout)
	if err != nil {
		return err
	}

	if err := c.handshake(conn); err != nil {
		conn.Close()
		return err
	}

	c.conn = conn
	c.connected.Store(true)

	c.wg.Add(3)
	go c.readLoop()
	go c.writeLoop()
	go c.heartbeatLoop()

	return nil
}

func (c *Client) handshake(conn net.Conn) error {
	key := generateKey()
	req := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n",
		c.opts.Host, key)

	if _, err := conn.Write([]byte(req)); err != nil {
		return err
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}

	resp := string(buf[:n])
	if len(resp) < 12 || resp[9] != '1' {
		return errors.New("websocket handshake failed")
	}

	return nil
}

func (c *Client) Disconnect() {
	c.closed.Store(true)
	c.connected.Store(false)
	close(c.doneCh)

	if c.conn != nil {
		c.conn.Close()
	}

	c.wg.Wait()
}

func (c *Client) RegisterRoute(route string, routeID uint32) {
	c.routeToID.Store(route, routeID)
	c.idToRoute.Store(routeID, route)
}

func (c *Client) Request(route string, msg interface{}) (interface{}, error) {
	if !c.connected.Load() {
		return nil, errors.New("not connected")
	}

	seq := c.seq.Add(1)
	data, err := c.encode(TypeRequest, route, seq, msg)
	if err != nil {
		return nil, err
	}

	resultCh := make(chan interface{}, 1)
	errCh := make(chan error, 1)

	timer := time.AfterFunc(c.opts.Timeout, func() {
		c.pending.Delete(seq)
		errCh <- errors.New("request timeout")
	})

	c.pending.Store(seq, RequestCallback{
		Resolve: func(v interface{}) { resultCh <- v },
		Reject:  func(e error) { errCh <- e },
		Timer:   timer,
	})

	c.writeCh <- data

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	}
}

func (c *Client) Notify(route string, msg interface{}) error {
	if !c.connected.Load() {
		return errors.New("not connected")
	}

	data, err := c.encode(TypeNotify, route, 0, msg)
	if err != nil {
		return err
	}

	select {
	case c.writeCh <- data:
		return nil
	default:
		return errors.New("write buffer full")
	}
}

func (c *Client) On(event string, callback func(interface{}), target interface{}) {
	handlers, _ := c.eventHandlers.LoadOrStore(event, &sync.Map{})
	handlers.(*sync.Map).Store(target, EventHandler{Callback: callback, Target: target})
}

func (c *Client) Off(event string, target interface{}) {
	handlers, ok := c.eventHandlers.Load(event)
	if !ok {
		return
	}
	handlers.(*sync.Map).Delete(target)
}

func (c *Client) Emit(event string, data interface{}) {
	handlers, ok := c.eventHandlers.Load(event)
	if !ok {
		return
	}
	handlers.(*sync.Map).Range(func(key, value any) bool {
		if h, ok := value.(EventHandler); ok {
			h.Callback(data)
		}
		return true
	})
}

func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.doneCh:
			return
		default:
		}

		if c.conn == nil {
			return
		}

		frame, err := c.readFrame()
		if err != nil {
			c.connected.Store(false)
			c.cleanupPending(errors.New("connection closed"))
			return
		}

		if len(frame) > 0 {
			c.handlePacket(frame)
		}
	}
}

func (c *Client) writeLoop() {
	defer c.wg.Done()

	for {
		select {
		case data := <-c.writeCh:
			if c.conn == nil {
				continue
			}

			frame := c.makeFrame(data)
			if _, err := c.conn.Write(frame); err != nil {
				c.connected.Store(false)
				return
			}
		case <-c.doneCh:
			return
		}
	}
}

func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(c.opts.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.connected.Load() {
				c.Notify("sys.heartbeat", map[string]interface{}{"ts": time.Now().UnixMilli()})
			}
		case <-c.doneCh:
			return
		}
	}
}

func (c *Client) encode(msgType MessageType, route string, seq uint64, msg interface{}) ([]byte, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	hasRouteID := false
	if msgType == TypeRequest {
		if _, ok := c.routeToID.Load(route); ok {
			hasRouteID = true
		}
	}

	var headerLen int
	if msgType == TypeResponse {
		headerLen = 1 + 4
	} else if hasRouteID {
		headerLen = 1 + 2
	} else {
		headerLen = 1 + 1 + len(route)
	}

	totalLen := 4 + headerLen + len(body)
	buf := make([]byte, totalLen)

	binary.BigEndian.PutUint32(buf[0:4], uint32(headerLen+len(body)-4))

	offset := 4
	buf[offset] = byte(msgType)
	offset++

	if msgType == TypeResponse {
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(seq))
		offset += 4
	} else if hasRouteID {
		if id, _ := c.routeToID.Load(route); id != nil {
			binary.BigEndian.PutUint16(buf[offset:offset+2], id.(uint16))
		}
		offset += 2
	} else {
		buf[offset] = byte(len(route))
		offset++
		copy(buf[offset:offset+len(route)], route)
		offset += len(route)
	}

	copy(buf[offset:offset+len(body)], body)

	return buf, nil
}

func (c *Client) handlePacket(data []byte) {
	if len(data) < 5 {
		return
	}

	length := binary.BigEndian.Uint32(data[0:4])
	if int(length)+4 > len(data) || length > 64*1024 {
		return
	}

	msgType := MessageType(data[4])
	offset := 5

	var route string
	var seq uint64

	if msgType == TypeResponse {
		seq = uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
	} else {
		routeLen := int(data[offset])
		offset++
		route = string(data[offset : offset+routeLen])
		offset += routeLen
	}

	body := data[offset:]
	var msgData interface{}
	if err := json.Unmarshal(body, &msgData); err != nil {
		msgData = string(body)
	}

	switch msgType {
	case TypeResponse:
		if cb, ok := c.pending.LoadAndDelete(seq); ok {
			callback := cb.(RequestCallback)
			callback.Timer.Stop()
			callback.Resolve(msgData)
		}
	case TypeRequest, TypeNotify:
		c.Emit(route, msgData)
	case TypeError:
		c.Emit("error", msgData)
	}
}

func (c *Client) cleanupPending(err error) {
	c.pending.Range(func(key, value any) bool {
		if cb, ok := value.(RequestCallback); ok {
			callback := cb
			callback.Timer.Stop()
			callback.Reject(err)
		}
		c.pending.Delete(key)
		return true
	})
}

func (c *Client) readFrame() ([]byte, error) {
	header := make([]byte, 2)
	if _, err := c.conn.Read(header); err != nil {
		return nil, err
	}

	opcode := header[0] & 0x0F
	payloadLen := uint64(header[1] & 0x7F)

	if opcode == 8 {
		return nil, errors.New("close")
	}

	if payloadLen == 126 {
		ext := make([]byte, 2)
		c.conn.Read(ext)
		payloadLen = binary.BigEndian.Uint64(ext[:2])
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		c.conn.Read(ext)
		payloadLen = binary.BigEndian.Uint64(ext[:8])
	}

	data := make([]byte, payloadLen)
	c.conn.Read(data)

	return data, nil
}

func (c *Client) makeFrame(data []byte) []byte {
	frame := make([]byte, 2+len(data))
	frame[0] = 0x81
	frame[1] = byte(len(data))
	copy(frame[2:], data)
	return frame
}

func generateKey() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() % 256)
	}
	result := make([]byte, 24)
	for i := range result {
		result[i] = b[i%16]
	}
	return fmt.Sprintf("%q", result)
}
