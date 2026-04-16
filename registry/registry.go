package registry

import (
	"sync"
	"time"
)

type ServerState int

const (
	StateStarting ServerState = iota
	StateRunning
	StateRunningHalf
	StateStopped
)

type ServerInfo struct {
	ID         string
	ServerType string
	Host       string
	Port       int
	State      ServerState
	Count      int
	RegisterAt int64
	LastUpdate int64
}

type WatchCallback func(event string, servers []*ServerInfo)

type Registry interface {
	Register(server *ServerInfo) error
	Unregister(serverID string) error
	GetServer(serverType string) (*ServerInfo, bool)
	GetServers(serverType string) []*ServerInfo
	GetAllServers() map[string][]*ServerInfo
	Watch(callback WatchCallback)
	Close()
}

type localRegistry struct {
	servers  map[string]*ServerInfo
	byType   map[string][]*ServerInfo
	watchers []WatchCallback
	mu       sync.RWMutex
}

func New() Registry {
	return &localRegistry{
		servers: make(map[string]*ServerInfo),
		byType:  make(map[string][]*ServerInfo),
	}
}

func (r *localRegistry) Register(server *ServerInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server.RegisterAt = time.Now().Unix()
	server.LastUpdate = time.Now().Unix()

	r.servers[server.ID] = server

	found := false
	for _, s := range r.byType[server.ServerType] {
		if s.ID == server.ID {
			found = true
			break
		}
	}
	if !found {
		r.byType[server.ServerType] = append(r.byType[server.ServerType], server)
	}

	callbacks := make([]WatchCallback, len(r.watchers))
	copy(callbacks, r.watchers)

	for _, cb := range callbacks {
		cb("add", r.byType[server.ServerType])
	}

	return nil
}

func (r *localRegistry) Unregister(serverID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, ok := r.servers[serverID]
	if !ok {
		return nil
	}

	delete(r.servers, serverID)

	st := r.byType[server.ServerType]
	for i, s := range st {
		if s.ID == serverID {
			r.byType[server.ServerType] = append(st[:i], st[i+1:]...)
			break
		}
	}

	callbacks := make([]WatchCallback, len(r.watchers))
	copy(callbacks, r.watchers)

	for _, cb := range callbacks {
		cb("remove", r.byType[server.ServerType])
	}

	return nil
}

func (r *localRegistry) GetServer(serverType string) (*ServerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers, ok := r.byType[serverType]
	if !ok || len(servers) == 0 {
		return nil, false
	}

	return servers[0], true
}

func (r *localRegistry) GetServers(serverType string) []*ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ServerInfo, len(r.byType[serverType]))
	copy(result, r.byType[serverType])
	return result
}

func (r *localRegistry) GetAllServers() map[string][]*ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]*ServerInfo)
	for t, ss := range r.byType {
		result[t] = make([]*ServerInfo, len(ss))
		copy(result[t], ss)
	}
	return result
}

func (r *localRegistry) Watch(callback WatchCallback) {
	r.mu.Lock()
	r.watchers = append(r.watchers, callback)
	r.mu.Unlock()
}

func (r *localRegistry) Close() {
	r.mu.Lock()
	r.servers = make(map[string]*ServerInfo)
	r.byType = make(map[string][]*ServerInfo)
	r.watchers = nil
	r.mu.Unlock()
}
