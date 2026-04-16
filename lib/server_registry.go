package lib

import (
	"sync"
)

type ServerRegistry struct {
	servers        map[string]map[string]any
	serverTypeMaps map[string][]map[string]any
	serverTypes    []string
	mu             sync.RWMutex
	event          *EventEmitter
}

func NewServerRegistry(event *EventEmitter) *ServerRegistry {
	return &ServerRegistry{
		servers:        make(map[string]map[string]any),
		serverTypeMaps: make(map[string][]map[string]any),
		serverTypes:    make([]string, 0),
		event:          event,
	}
}

func (r *ServerRegistry) SetServers(servers map[string]map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = servers
	for _, server := range servers {
		serverType, ok := server["serverType"].(string)
		if !ok {
			continue
		}
		r.serverTypeMaps[serverType] = append(r.serverTypeMaps[serverType], server)
		if !containsString(r.serverTypes, serverType) {
			r.serverTypes = append(r.serverTypes, serverType)
		}
	}
}

func (r *ServerRegistry) AddServers(servers []map[string]any) {
	r.mu.Lock()
	for _, item := range servers {
		id, ok := item["id"].(string)
		if !ok {
			continue
		}
		serverType, ok := item["serverType"].(string)
		if !ok {
			continue
		}
		r.servers[id] = item
		slist := r.serverTypeMaps[serverType]
		if slist == nil {
			slist = make([]map[string]any, 0)
			r.serverTypeMaps[serverType] = slist
		}
		replaceServer(&slist, item)
		r.serverTypeMaps[serverType] = slist
		if !containsString(r.serverTypes, serverType) {
			r.serverTypes = append(r.serverTypes, serverType)
		}
	}
	r.mu.Unlock()
	if r.event != nil {
		r.event.Emit("add_servers", servers)
	}
}

func (r *ServerRegistry) RemoveServers(ids []string) {
	r.mu.Lock()
	for _, id := range ids {
		if item, ok := r.servers[id]; ok {
			delete(r.servers, id)
			serverType, ok := item["serverType"].(string)
			if !ok {
				continue
			}
			slist := r.serverTypeMaps[serverType]
			removeServer(&slist, id)
			r.serverTypeMaps[serverType] = slist
		}
	}
	r.mu.Unlock()
	if r.event != nil {
		r.event.Emit("remove_servers", ids)
	}
}

func (r *ServerRegistry) GetServers() map[string]map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers
}

func (r *ServerRegistry) GetServerTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.serverTypes
}

func (r *ServerRegistry) GetServersByType(serverType string) []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.serverTypeMaps[serverType]
}

func (r *ServerRegistry) GetServerById(serverId string) (map[string]any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.servers[serverId]
	return s, ok
}
