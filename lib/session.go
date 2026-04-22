package lib

import (
	"fmt"
	"sync"
)

type SessionStorage struct {
	kv map[string]any
	mu sync.RWMutex
}

func NewSessionStorage() *SessionStorage {
	return &SessionStorage{kv: make(map[string]any)}
}

func (s *SessionStorage) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kv[key]
}

func (s *SessionStorage) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = value
}

func (s *SessionStorage) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kv, key)
}

func (s *SessionStorage) KV() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]any)
	for k, v := range s.kv {
		cp[k] = v
	}
	return cp
}

func (s *SessionStorage) DeepCopy() *SessionStorage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := &SessionStorage{kv: make(map[string]any, len(s.kv))}
	for k, v := range s.kv {
		cp.kv[k] = deepCloneValue(v)
	}
	return cp
}

func deepCloneValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case map[string]any:
		if val == nil {
			return nil
		}
		cp := make(map[string]any, len(val))
		for k, v := range val {
			cp[k] = deepCloneValue(v)
		}
		return cp
	case []any:
		if val == nil {
			return nil
		}
		cp := make([]any, len(val))
		for i, v := range val {
			cp[i] = deepCloneValue(v)
		}
		return cp
	case string, int, int64, float64, bool:
		return val
	default:
		return v
	}
}

type Session struct {
	id           uint64
	uid          string
	serverID     string
	serverType   string
	connectionID uint64
	conn         Connection

	storage *SessionStorage
	mu      sync.RWMutex
	closed  bool
}

func (s *Session) Storage() *SessionStorage {
	return s.storage
}

func (s *Session) SetConnection(conn Connection) { s.conn = conn }
func (s *Session) Connection() Connection        { return s.conn }

func NewSession() *Session {
	return &Session{storage: NewSessionStorage()}
}

func (s *Session) Get(key string) any {
	return s.storage.Get(key)
}

func (s *Session) Set(key string, value any) {
	s.storage.Set(key, value)
}

func (s *Session) Remove(key string) {
	s.storage.Remove(key)
}

func (s *Session) ID() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

func (s *Session) SetID(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.id = id
}

func (s *Session) GetUID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.uid
}

func (s *Session) UID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.uid
}

func (s *Session) SetUID(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uid = uid
}

func (s *Session) GetServerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverID
}

func (s *Session) SetServerID(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverID = serverID
}

func (s *Session) GetServerType() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverType
}

func (s *Session) SetServerType(serverType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverType = serverType
}

func (s *Session) GetConnectionID() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectionID
}

func (s *Session) SetConnectionID(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectionID = id
}

func (s *Session) Bind(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uid = uid
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.conn != nil {
		s.conn.Close()
	}
}

func (s *Session) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

func (s *Session) KV() map[string]any {
	return s.storage.KV()
}

func (s *Session) SendResponse(seq uint64, route string, body any) error {
	s.mu.RLock()
	closed := s.closed
	conn := s.conn
	s.mu.RUnlock()

	if closed || conn == nil {
		return fmt.Errorf("session: closed or connection is nil")
	}
	return conn.Send(&Message{
		Type:  Response,
		Seq:   seq,
		Route: route,
		Body:  body,
	})
}

func (s *Session) Send(msg *Message) error {
	s.mu.RLock()
	closed := s.closed
	conn := s.conn
	s.mu.RUnlock()

	if closed || conn == nil {
		return fmt.Errorf("session: closed or connection is nil")
	}
	return conn.Send(msg)
}

func (s *Session) DeepCopy() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &Session{
		id:           s.id,
		uid:          s.uid,
		serverID:     s.serverID,
		serverType:   s.serverType,
		connectionID: s.connectionID,
		conn:         nil,
		storage:      s.storage.DeepCopy(),
		closed:       s.closed,
	}
}
