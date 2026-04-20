package server_registry

import (
	"sync"
	"time"
)

type ServerInfo struct {
	ID         string
	ServerType string
	Host       string
	Port       int
	Frontend   bool
	State      int
	Count      int
	RegisterAt int64
	LastUpdate int64
	Metadata   map[string]any
}

type RegistryEventHandler interface {
	OnServerRegister(server ServerInfo)
	OnServerUnregister(serverID string)
}

type ServerRegistry interface {
	Register(server ServerInfo) error
	Unregister(serverID string) error
	GetServer(serverID string) (ServerInfo, bool)
	GetServersByType(serverType string) []ServerInfo
	GetAllServers() []ServerInfo
	GetServerTypes() []string
	Watch(ch chan<- []ServerInfo)
	SetEventHandler(handler RegistryEventHandler)
	Close()
}

type serverRegistry struct {
	servers        map[string]ServerInfo
	serverTypeMaps map[string][]ServerInfo
	serverTypes    []string
	watchers       []chan<- []ServerInfo
	handler        RegistryEventHandler
	mu             sync.RWMutex
	closed         bool
}

func New() ServerRegistry {
	return &serverRegistry{
		servers:        make(map[string]ServerInfo),
		serverTypeMaps: make(map[string][]ServerInfo),
		serverTypes:    make([]string, 0),
	}
}

func (r *serverRegistry) Register(server ServerInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.servers[server.ID] = server
	st := server.ServerType

	if r.serverTypeMaps[st] == nil {
		r.serverTypeMaps[st] = make([]ServerInfo, 0)
		r.serverTypes = append(r.serverTypes, st)
	}

	found := false
	for _, s := range r.serverTypeMaps[st] {
		if s.ID == server.ID {
			found = true
			break
		}
	}
	if !found {
		r.serverTypeMaps[st] = append(r.serverTypeMaps[st], server)
	}

	r.notifyWatchers()

	if r.handler != nil {
		r.handler.OnServerRegister(server)
	}

	return nil
}

func (r *serverRegistry) Unregister(serverID string) error {
	r.mu.Lock()
	server, ok := r.servers[serverID]
	if !ok {
		r.mu.Unlock()
		return nil
	}

	delete(r.servers, serverID)
	st := server.ServerType

	newList := make([]ServerInfo, 0, len(r.serverTypeMaps[st]))
	for _, s := range r.serverTypeMaps[st] {
		if s.ID != serverID {
			newList = append(newList, s)
		}
	}
	r.serverTypeMaps[st] = newList

	if len(newList) == 0 {
		newTypes := make([]string, 0, len(r.serverTypes))
		for _, t := range r.serverTypes {
			if t != st {
				newTypes = append(newTypes, t)
			}
		}
		r.serverTypes = newTypes
	}

	r.notifyWatchers()
	r.mu.Unlock()

	if r.handler != nil {
		r.handler.OnServerUnregister(serverID)
	}

	return nil
}

func (r *serverRegistry) GetServer(serverID string) (ServerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.servers[serverID]
	return s, ok
}

func (r *serverRegistry) GetServersByType(serverType string) []ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := r.serverTypeMaps[serverType]
	result := make([]ServerInfo, len(servers))
	copy(result, servers)
	return result
}

func (r *serverRegistry) GetAllServers() []ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ServerInfo, 0, len(r.servers))
	for _, s := range r.servers {
		result = append(result, s)
	}
	return result
}

func (r *serverRegistry) GetServerTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.serverTypes))
	copy(result, r.serverTypes)
	return result
}

func (r *serverRegistry) Watch(ch chan<- []ServerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	safeCh := make(chan []ServerInfo, 100)
	go func() {
		for servers := range safeCh {
			for i := 0; i < 10; i++ {
				select {
				case ch <- servers:
					break
				case <-time.After(time.Millisecond):
				}
			}
		}
	}()

	r.watchers = append(r.watchers, safeCh)
}

func (r *serverRegistry) SetEventHandler(handler RegistryEventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
}

func (r *serverRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true
	for _, ch := range r.watchers {
		close(ch)
	}
	r.watchers = nil
}

func (r *serverRegistry) notifyWatchers() {
	r.mu.RLock()
	if r.closed || len(r.watchers) == 0 {
		r.mu.RUnlock()
		return
	}

	snapshot := make([]ServerInfo, 0, len(r.servers))
	for _, s := range r.servers {
		snapshot = append(snapshot, s)
	}
	watchers := r.watchers
	r.mu.RUnlock()

	for _, ch := range watchers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}
