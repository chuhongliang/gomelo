package component

import (
	"sync"
	"time"

	"github.com/chuhongliang/gomelo/lib"
)

type ConnectionManager struct {
	opts     *ConnectionOptions
	conns    map[uint64]*ManagedConnection
	sessions map[uint64]*lib.Session

	connID uint64
	mu     sync.RWMutex
}

type ConnectionOptions struct {
	MaxConns          int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
}

var defaultConnOpts = &ConnectionOptions{
	MaxConns:          10000,
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      10 * time.Second,
	HeartbeatInterval: 30 * time.Second,
	HeartbeatTimeout:  90 * time.Second,
}

type ManagedConnection struct {
	ID       uint64
	Conn     lib.Connection
	Session  *lib.Session
	LastSeen time.Time

	done chan struct{}
}

func NewConnectionManager(opts *ConnectionOptions) *ConnectionManager {
	if opts == nil {
		opts = defaultConnOpts
	}
	return &ConnectionManager{
		opts:     opts,
		conns:    make(map[uint64]*ManagedConnection),
		sessions: make(map[uint64]*lib.Session),
	}
}

func (m *ConnectionManager) Add(conn lib.Connection, session *lib.Session) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connID++
	m.conns[conn.ID()] = &ManagedConnection{
		ID:       conn.ID(),
		Conn:     conn,
		Session:  session,
		LastSeen: time.Now(),
		done:     make(chan struct{}),
	}
	m.sessions[session.ID()] = session
}

func (m *ConnectionManager) GetConnection(id uint64) (lib.Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.conns[id]
	if ok {
		return c.Conn, ok
	}
	return nil, false
}

func (m *ConnectionManager) GetSession(id uint64) (*lib.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *ConnectionManager) RemoveConnection(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[id]; ok {
		close(c.done)
		delete(m.conns, id)
		if c.Session != nil && c.Session.ID() != 0 {
			delete(m.sessions, c.Session.ID())
		}
	}
}

func (m *ConnectionManager) CloseConnection(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[id]; ok {
		c.Conn.Close()
		close(c.done)
		delete(m.conns, id)
	}
}

func (m *ConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}

func (m *ConnectionManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.conns {
		c.Conn.Close()
		close(c.done)
	}
	m.conns = make(map[uint64]*ManagedConnection)
	m.sessions = make(map[uint64]*lib.Session)
}

type SessionManager struct {
	sessions map[uint64]*lib.Session
	uidMap   map[string]uint64

	seq uint64
	mu  sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[uint64]*lib.Session),
		uidMap:   make(map[string]uint64),
	}
}

func (m *SessionManager) Create(sid uint64) *lib.Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sid == 0 {
		m.seq++
		sid = m.seq
	}

	session := lib.NewSession()
	session.SetID(sid)
	m.sessions[sid] = session
	return session
}

func (m *SessionManager) Get(sid uint64) (*lib.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sid]
	return s, ok
}

func (m *SessionManager) GetByUID(uid string) (*lib.Session, bool) {
	m.mu.RLock()
	sid, ok := m.uidMap[uid]
	m.mu.RUnlock()

	if !ok {
		return nil, false
	}
	return m.Get(sid)
}

func (m *SessionManager) Bind(sid uint64, uid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sid]
	if !ok {
		return false
	}

	if session.UID() != "" {
		delete(m.uidMap, session.UID())
	}

	session.SetUID(uid)
	m.uidMap[uid] = sid
	return true
}

func (m *SessionManager) Unbind(uid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	sid, ok := m.uidMap[uid]
	if !ok {
		return false
	}

	session, ok := m.sessions[sid]
	if !ok {
		return false
	}

	session.SetUID("")
	delete(m.uidMap, uid)
	return true
}

func (m *SessionManager) Remove(sid uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sid]
	if ok && session.UID() != "" {
		delete(m.uidMap, session.UID())
	}
	delete(m.sessions, sid)
}

func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[uint64]*lib.Session)
	m.uidMap = make(map[string]uint64)
	m.seq = 0
}

type Channel struct {
	id    string
	group map[uint64]*lib.Session
	mu    sync.RWMutex
}

func NewChannel(id string) *Channel {
	return &Channel{
		id:    id,
		group: make(map[uint64]*lib.Session),
	}
}

func (c *Channel) Add(session *lib.Session) {
	c.mu.Lock()
	c.group[session.ID()] = session
	c.mu.Unlock()
}

func (c *Channel) Remove(sid uint64) {
	c.mu.Lock()
	delete(c.group, sid)
	c.mu.Unlock()
}

func (c *Channel) Contains(sid uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.group[sid]
	return ok
}

func (c *Channel) Members() []*lib.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()

	members := make([]*lib.Session, 0, len(c.group))
	for _, s := range c.group {
		members = append(members, s)
	}
	return members
}

func (c *Channel) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.group)
}

func (c *Channel) Clear() {
	c.mu.Lock()
	c.group = make(map[uint64]*lib.Session)
	c.mu.Unlock()
}

func (c *Channel) Push(route string, msg any) {
	c.mu.RLock()
	members := make([]*lib.Session, 0, len(c.group))
	for _, s := range c.group {
		members = append(members, s)
	}
	c.mu.RUnlock()

	for _, s := range members {
		if conn := s.Connection(); conn != nil {
			for attempt := 0; attempt < 3; attempt++ {
				if attempt > 0 {
					time.Sleep(time.Duration(attempt*50) * time.Millisecond)
				}
				if err := conn.Send(&lib.Message{Type: lib.Broadcast, Route: route, Body: msg}); err == nil {
					break
				}
			}
		}
	}
}

type ChannelManager struct {
	channels map[string]*Channel
	mu       sync.RWMutex
}

func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]*Channel),
	}
}

func (m *ChannelManager) Create(id string) (*Channel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.channels[id]; ok {
		return c, false
	}

	c := NewChannel(id)
	m.channels[id] = c
	return c, true
}

func (m *ChannelManager) Get(id string) (*Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.channels[id]
	return c, ok
}

func (m *ChannelManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
}

func (m *ChannelManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels = make(map[string]*Channel)
}
