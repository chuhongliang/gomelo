package selector

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/chuhongliang/gomelo/server_registry"
)

type SelectorHandler func([]server_registry.ServerInfo) server_registry.ServerInfo

type Selector interface {
	Select(serverType string) server_registry.ServerInfo
	SelectMulti(serverType string, n int) []server_registry.ServerInfo
	Register(serverType string, handler SelectorHandler)
	GetStats() (total, fail int64)
}

type selector struct {
	reg      server_registry.ServerRegistry
	handlers map[string]SelectorHandler
	mu       sync.RWMutex
	stats    struct {
		total int64
		fail  int64
	}
}

func New(reg server_registry.ServerRegistry) Selector {
	s := &selector{
		reg:      reg,
		handlers: make(map[string]SelectorHandler),
	}

	return s
}

func (s *selector) Select(serverType string) server_registry.ServerInfo {
	atomic.AddInt64(&s.stats.total, 1)

	servers := s.reg.GetServersByType(serverType)
	if len(servers) == 0 {
		atomic.AddInt64(&s.stats.fail, 1)
		return server_registry.ServerInfo{}
	}

	s.mu.RLock()
	handler := s.handlers[serverType]
	s.mu.RUnlock()

	if handler != nil {
		return handler(servers)
	}

	lb := &LoadBalancer{}
	return lb.Select(servers)
}

func (s *selector) SelectMulti(serverType string, n int) []server_registry.ServerInfo {
	atomic.AddInt64(&s.stats.total, 1)

	servers := s.reg.GetServersByType(serverType)
	if len(servers) == 0 {
		atomic.AddInt64(&s.stats.fail, 1)
		return nil
	}

	if n > len(servers) {
		n = len(servers)
	}

	s.mu.RLock()
	handler := s.handlers[serverType]
	s.mu.RUnlock()

	if handler != nil {
		result := make([]server_registry.ServerInfo, 0, n)
		for i := 0; i < n; i++ {
			result = append(result, handler(servers))
		}
		return result
	}

	lb := &LoadBalancer{}
	return lb.SelectMulti(servers, n)
}

func (s *selector) Register(serverType string, handler SelectorHandler) {
	s.mu.Lock()
	s.handlers[serverType] = handler
	s.mu.Unlock()
}

func (s *selector) GetStats() (total, fail int64) {
	total = atomic.LoadInt64(&s.stats.total)
	fail = atomic.LoadInt64(&s.stats.fail)
	return
}

type LoadBalancer struct {
	mu     sync.Mutex
	curIdx map[string]int
}

func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		curIdx: make(map[string]int),
	}
}

func (lb *LoadBalancer) Select(servers []server_registry.ServerInfo) server_registry.ServerInfo {
	if len(servers) == 0 {
		return server_registry.ServerInfo{}
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	key := servers[0].ServerType
	idx := lb.curIdx[key]
	server := servers[idx%len(servers)]
	lb.curIdx[key] = (idx + 1) % len(servers)

	return server
}

func (lb *LoadBalancer) SelectMulti(servers []server_registry.ServerInfo, n int) []server_registry.ServerInfo {
	if len(servers) == 0 || n <= 0 {
		return nil
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	key := servers[0].ServerType
	idx := lb.curIdx[key]

	result := make([]server_registry.ServerInfo, n)
	for i := 0; i < n; i++ {
		result[i] = servers[(idx+i)%len(servers)]
	}

	lb.curIdx[key] = (idx + n) % len(servers)
	return result
}

type ConsistentHashSelector struct {
	ring       map[int64]string
	sortedKeys []int64
	nodes      map[string]server_registry.ServerInfo
	replicas   int
	mu         sync.RWMutex
	hashFunc   func(string) int64
}

func NewConsistentHashSelector(replicas int, hashFunc func(string) int64) *ConsistentHashSelector {
	if hashFunc == nil {
		hashFunc = defaultHash
	}
	return &ConsistentHashSelector{
		ring:       make(map[int64]string),
		sortedKeys: make([]int64, 0),
		nodes:      make(map[string]server_registry.ServerInfo),
		replicas:   replicas,
		hashFunc:   hashFunc,
	}
}

func defaultHash(key string) int64 {
	h := int64(0)
	for _, c := range key {
		h = h*31 + int64(c)
	}
	return h
}

func (s *ConsistentHashSelector) AddServer(server server_registry.ServerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%d", server.Host, server.Port)
	s.nodes[key] = server

	added := make([]int64, 0, s.replicas)
	for i := 0; i < s.replicas; i++ {
		hash := s.hashFunc(fmt.Sprintf("%s-%d", key, i))
		s.ring[hash] = key
		added = append(added, hash)
	}

	s.sortedKeys = append(s.sortedKeys, added...)
	sort.Slice(s.sortedKeys, func(i, j int) bool { return s.sortedKeys[i] < s.sortedKeys[j] })
}

func (s *ConsistentHashSelector) RemoveServer(server server_registry.ServerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%d", server.Host, server.Port)
	delete(s.nodes, key)

	removed := make(map[int64]bool, s.replicas)
	for i := 0; i < s.replicas; i++ {
		hash := s.hashFunc(fmt.Sprintf("%s-%d", key, i))
		delete(s.ring, hash)
		removed[hash] = true
	}

	newKeys := make([]int64, 0, len(s.sortedKeys)-s.replicas)
	for _, k := range s.sortedKeys {
		if !removed[k] {
			newKeys = append(newKeys, k)
		}
	}
	s.sortedKeys = newKeys
}

func (s *ConsistentHashSelector) Select(keys ...string) server_registry.ServerInfo {
	if len(keys) == 0 || len(s.ring) == 0 {
		return server_registry.ServerInfo{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	hash := s.hashFunc(keys[0])
	closest := s.findClosest(hash)

	if key, ok := s.ring[closest]; ok {
		if server, ok := s.nodes[key]; ok {
			return server
		}
	}

	return server_registry.ServerInfo{}
}

func (s *ConsistentHashSelector) findClosest(hash int64) int64 {
	keys := s.sortedKeys

	if len(keys) == 0 {
		return hash
	}

	idx := sort.Search(len(keys), func(i int) bool { return keys[i] >= hash })

	if idx < len(keys) {
		return keys[idx]
	}
	return keys[0]
}
