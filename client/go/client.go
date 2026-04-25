package client

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ProtocolType string

const (
	ProtocolTCP   ProtocolType = "tcp"
	ProtocolUDP   ProtocolType = "udp"
	ProtocolWebSocket ProtocolType = "ws"
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
	Protocol             ProtocolType
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
	connMu    sync.RWMutex
	connected atomic.Bool
	closed    atomic.Bool

	seq     atomic.Uint64
	pending sync.Map

	eventHandlers sync.Map

	writeCh     chan []byte
	doneCh      chan struct{}
	reconnectCh chan struct{}
	wg          sync.WaitGroup

	routeToID sync.Map
	idToRoute sync.Map

	nextRouteID uint32

	onConnected    func()
	onDisconnected func()
	onError        func(error)
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
	if opts.Protocol == "" {
		opts.Protocol = ProtocolWebSocket
	}

	return &Client{
		opts:        opts,
		writeCh:     make(chan []byte, 256),
		doneCh:      make(chan struct{}),
		reconnectCh: make(chan struct{}, 1),
	}
}

func (c *Client) OnConnected(cb func()) {
	c.onConnected = cb
}

func (c *Client) OnDisconnected(cb func()) {
	c.onDisconnected = cb
}

func (c *Client) OnError(cb func(error)) {
	c.onError = cb
}

func (c *Client) Connect() error {
	if err := c.doConnect(); err != nil {
		return err
	}

	c.wg.Add(4)
	go c.readLoop()
	go c.writeLoop()
	go c.heartbeatLoop()
	go c.reconnectLoop()

	return nil
}

func (c *Client) doConnect() error {
	addr := fmt.Sprintf("%s:%d", c.opts.Host, c.opts.Port)

	var conn net.Conn
	var err error

	switch c.opts.Protocol {
	case ProtocolUDP:
		conn, err = net.DialTimeout("udp", addr, c.opts.Timeout)
	default:
		conn, err = net.DialTimeout("tcp", addr, c.opts.Timeout)
	}
	if err != nil {
		return err
	}

	if c.opts.Protocol == ProtocolWebSocket {
		if err := c.wsHandshake(conn); err != nil {
			conn.Close()
			return err
		}
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
	c.connected.Store(true)

	if c.onConnected != nil {
		go c.onConnected()
	}

	return nil
}

func (c *Client) reconnectLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.doneCh:
			return
		case <-c.reconnectCh:
			if c.closed.Load() {
				return
			}

			for attempt := 1; attempt <= c.opts.MaxReconnectAttempts; attempt++ {
				if c.closed.Load() {
					return
				}

				time.Sleep(time.Duration(attempt*1000) * time.Millisecond)

				if c.closed.Load() {
					return
				}

				if err := c.doConnect(); err != nil {
					if c.onError != nil {
						go c.onError(fmt.Errorf("reconnect attempt %d failed: %v", attempt, err))
					}
					continue
				}

				c.wg.Add(2)
				go c.readLoop()
				go c.writeLoop()
				break
			}
		}
	}
}

func (c *Client) wsHandshake(conn net.Conn) error {
	key := generateKey()
	req := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s:%d\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Protocol: gomelo\r\n\r\n",
		c.opts.Host, c.opts.Port, key)

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

	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	c.wg.Wait()
}

func (c *Client) RegisterRoute(route string, routeID uint32) {
	c.routeToID.Store(route, routeID)
	c.idToRoute.Store(routeID, route)
}

func (c *Client) GenerateRouteID() uint32 {
	return atomic.AddUint32(&c.nextRouteID, 1)
}

func (c *Client) Request(route string, msg interface{}) (interface{}, error) {
	return c.RequestWithTimeout(route, msg, c.opts.Timeout)
}

func (c *Client) RequestWithTimeout(route string, msg interface{}, timeout time.Duration) (interface{}, error) {
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

	timer := time.AfterFunc(timeout, func() {
		if _, ok := c.pending.LoadAndDelete(seq); ok {
			errCh <- errors.New("request timeout")
		}
	})

	c.pending.Store(seq, RequestCallback{
		Resolve: func(v interface{}) { resultCh <- v },
		Reject:  func(e error) { errCh <- e },
		Timer:   timer,
	})

	select {
	case c.writeCh <- data:
	case <-time.After(time.Millisecond * 100):
		timer.Stop()
		c.pending.Delete(seq)
		return nil, errors.New("write buffer full")
	}

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
	case <-time.After(time.Millisecond * 100):
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
		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			return
		}

		var frame []byte
		var err error

		if c.opts.Protocol == ProtocolWebSocket {
			frame, err = c.readWSFrame(conn)
		} else {
			frame, err = c.readBinaryFrame(conn)
		}

		if err != nil {
			c.connected.Store(false)

			if c.onDisconnected != nil {
				go c.onDisconnected()
			}

			if !c.closed.Load() && c.opts.MaxReconnectAttempts > 0 && c.opts.Protocol != ProtocolUDP {
				select {
				case c.reconnectCh <- struct{}{}:
				default:
				}
			}

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
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil {
				continue
			}

			var frame []byte
			if c.opts.Protocol == ProtocolWebSocket {
				frame = c.makeWSFrame(data)
			} else {
				frame = data
			}

			if _, err := conn.Write(frame); err != nil {
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

	routeID, hasRouteID := c.routeToID.Load(route)
	useRouteID := hasRouteID && msgType == TypeRequest

	var headerLen int
	if msgType == TypeResponse {
		headerLen = 1 + 4
	} else if useRouteID {
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
	} else if useRouteID {
		binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(routeID.(uint32)))
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
	if int(length)+4 > len(data) || length > 64*1024 || length == 0 {
		return
	}

	msgType := MessageType(data[4])
	offset := 5

	var route string
	var seq uint64

	if msgType == TypeResponse {
		if offset+4 > len(data) {
			return
		}
		seq = uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
	} else {
		if offset >= len(data) {
			return
		}
		routeLen := int(data[offset])
		offset++
		if offset+routeLen > len(data) {
			return
		}
		route = string(data[offset : offset+routeLen])
		offset += routeLen
	}

	if offset >= len(data) {
		return
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

func (c *Client) readWSFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := conn.Read(header); err != nil {
		return nil, err
	}

	opcode := header[0] & 0x0F
	payloadLen := uint64(header[1] & 0x7F)

	if opcode == 8 {
		return nil, errors.New("server closed connection")
	}

	if payloadLen == 126 {
		ext := make([]byte, 2)
		if _, err := conn.Read(ext); err != nil {
			return nil, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(ext[:2]))
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		if _, err := conn.Read(ext); err != nil {
			return nil, err
		}
		payloadLen = binary.BigEndian.Uint64(ext[:8])
	}

	data := make([]byte, payloadLen)
	n := 0
	for n < int(payloadLen) {
		read, err := conn.Read(data[n:])
		if err != nil {
			return nil, err
		}
		n += read
	}

	return data, nil
}

func (c *Client) readBinaryFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := conn.Read(header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)
	if length > 64*1024 || length == 0 {
		return nil, errors.New("invalid frame length")
	}

	data := make([]byte, length)
	n := 0
	for n < int(length) {
		read, err := conn.Read(data[n:])
		if err != nil {
			return nil, err
		}
		n += read
	}

	return data, nil
}

func (c *Client) makeWSFrame(data []byte) []byte {
	frame := make([]byte, 2+len(data))
	frame[0] = 0x81
	frame[1] = byte(len(data))
	copy(frame[2:], data)
	return frame
}

func generateKey() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	key := make([]byte, 16)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range key {
		key[i] = chars[r.Intn(len(chars))]
	}
	return base64Encode(key)
}

func base64Encode(src []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var dst strings.Builder
	for i := 0; i < len(src); i += 3 {
		var n uint32
		switch len(src) - i {
		case 1:
			n = uint32(src[i]) << 16
		case 2:
			n = uint32(src[i])<<16 | uint32(src[i+1])<<8
		default:
			n = uint32(src[i])<<16 | uint32(src[i+1])<<8 | uint32(src[i+2])
		}
		dst.WriteByte(chars[(n>>18)&0x3F])
		dst.WriteByte(chars[(n>>12)&0x3F])
		if len(src)-i > 1 {
			dst.WriteByte(chars[(n>>6)&0x3F])
		}
		if len(src)-i > 2 {
			dst.WriteByte(chars[n&0x3F])
		}
	}
	pad := (4 - len(src)%3) % 4
	for i := 0; i < pad; i++ {
		dst.WriteByte('=')
	}
	return dst.String()
}