package websocket

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chuhongliang/gomelo/lib"
)

var (
	ErrNotWebSocket     = errors.New("not a websocket connection")
	ErrInvalidHandshake = errors.New("invalid websocket handshake")
)

type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(origin string) bool
}

type Conn struct {
	id       uint64
	conn     net.Conn
	mu       sync.Mutex
	closed   bool
	readBuf  []byte
	writeBuf []byte
}

func NewConn(id uint64, conn net.Conn) *Conn {
	return &Conn{
		id:       id,
		conn:     conn,
		readBuf:  make([]byte, 4096),
		writeBuf: make([]byte, 0, 4096),
	}
}

func (c *Conn) ID() uint64 { return c.id }

func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.conn.Close()
	}
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) Send(msg *lib.Message) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	_, err = c.conn.Write(append(header, data...))
	return err
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *Conn) ReadMessage() ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length > 64*1024 {
		return nil, errors.New("message too large")
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Conn) WriteMessage(data []byte) error {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	_, err := c.conn.Write(header)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(data)
	return err
}

type WSConnection struct {
	*Conn
	session   *lib.Session
	onClose   func(*lib.Session)
	msgParser func(*lib.Session, []byte)
}

func NewWSConnection(id uint64, conn net.Conn, session *lib.Session) *WSConnection {
	return &WSConnection{
		Conn:    NewConn(id, conn),
		session: session,
	}
}

func (c *WSConnection) SetOnClose(fn func(*lib.Session)) {
	c.onClose = fn
}

func (c *WSConnection) SetMessageParser(fn func(*lib.Session, []byte)) {
	c.msgParser = fn
}

func (c *WSConnection) Start() {
	go c.readLoop()
}

func (c *WSConnection) readLoop() {
	defer func() {
		if c.onClose != nil {
			c.onClose(c.session)
		}
		c.Close()
	}()

	for {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		data, err := c.ReadMessage()
		if err != nil {
			return
		}
		if c.msgParser != nil {
			c.msgParser(c.session, data)
		}
	}
}

type SessionManager struct {
	sessions map[uint64]*lib.Session
	conns    map[uint64]*WSConnection
	mu       sync.RWMutex
	nextID   uint64
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[uint64]*lib.Session),
		conns:    make(map[uint64]*WSConnection),
	}
}

func (sm *SessionManager) NewSession(conn net.Conn) (*lib.Session, *WSConnection) {
	sm.mu.Lock()
	id := atomic.AddUint64(&sm.nextID, 1)
	session := lib.NewSession()
	wsConn := NewWSConnection(id, conn, session)
	session.SetConnection(wsConn)
	session.Set("remoteAddr", conn.RemoteAddr().String())

	sm.sessions[id] = session
	sm.conns[id] = wsConn
	sm.mu.Unlock()

	return session, wsConn
}

func (sm *SessionManager) RemoveSession(id uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if wsConn, ok := sm.conns[id]; ok {
		wsConn.Close()
	}
	delete(sm.sessions, id)
	delete(sm.conns, id)
}

func (sm *SessionManager) GetSession(id uint64) (*lib.Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[id]
	return s, ok
}
