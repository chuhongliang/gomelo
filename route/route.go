package route

import (
	"sync"
)

type Compressor struct {
	routes map[string]uint16
	ids    map[uint16]string
	nextID uint16
	mu     sync.RWMutex
}

func NewCompressor() *Compressor {
	return &Compressor{
		routes: make(map[string]uint16),
		ids:    make(map[uint16]string),
	}
}

func (c *Compressor) Register(route string) uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if id, ok := c.routes[route]; ok {
		return id
	}

	c.nextID++
	id := c.nextID
	c.routes[route] = id
	c.ids[id] = route
	return id
}

func (c *Compressor) Compress(route string) uint16 {
	c.mu.RLock()
	id, ok := c.routes[route]
	c.mu.RUnlock()

	if ok {
		return id
	}

	return c.Register(route)
}

func (c *Compressor) Decompress(id uint16) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ids[id]
}

func (c *Compressor) GetRouteID(route string) (uint16, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	id, ok := c.routes[route]
	return id, ok
}

func (c *Compressor) GetRoute(id uint16) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	route, ok := c.ids[id]
	return route, ok
}

func (c *Compressor) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.routes)
}

func (c *Compressor) Clear() {
	c.mu.Lock()
	c.routes = make(map[string]uint16)
	c.ids = make(map[uint16]string)
	c.nextID = 0
	c.mu.Unlock()
}

type RouterTable struct {
	compressor *Compressor
	handlers   map[string]any
	mu         sync.RWMutex
}

func NewRouterTable() *RouterTable {
	return &RouterTable{
		compressor: NewCompressor(),
		handlers:   make(map[string]any),
	}
}

func (t *RouterTable) Register(route string, handler any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.compressor.Register(route)
	t.handlers[route] = handler
}

func (t *RouterTable) RegisterID(route string, handler any) uint16 {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := t.compressor.Register(route)
	t.handlers[route] = handler
	return id
}

func (t *RouterTable) GetHandler(route string) (any, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	h, ok := t.handlers[route]
	return h, ok
}

func (t *RouterTable) GetHandlerByID(id uint16) (any, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	route, ok := t.compressor.GetRoute(id)
	if !ok {
		return nil, false
	}

	h, ok := t.handlers[route]
	return h, ok
}

func (t *RouterTable) Compress(route string) uint16 {
	return t.compressor.Compress(route)
}

func (t *RouterTable) Decompress(id uint16) string {
	return t.compressor.Decompress(id)
}

type RouteManager struct {
	tables map[string]*RouterTable
	mu     sync.RWMutex
}

func NewRouteManager() *RouteManager {
	return &RouteManager{
		tables: make(map[string]*RouterTable),
	}
}

func (m *RouteManager) GetOrCreateTable(serverType string) *RouterTable {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, ok := m.tables[serverType]; ok {
		return t
	}

	t := NewRouterTable()
	m.tables[serverType] = t
	return t
}

func (m *RouteManager) Register(serverType, route string, handler any) {
	t := m.GetOrCreateTable(serverType)
	t.Register(route, handler)
}

func (m *RouteManager) RegisterWithID(serverType, route string, handler any) uint16 {
	t := m.GetOrCreateTable(serverType)
	return t.RegisterID(route, handler)
}

func (m *RouteManager) GetHandler(serverType, route string) (any, bool) {
	m.mu.RLock()
	t, ok := m.tables[serverType]
	m.mu.RUnlock()

	if !ok {
		return nil, false
	}

	return t.GetHandler(route)
}

func (m *RouteManager) Compress(serverType, route string) uint16 {
	m.mu.RLock()
	t, ok := m.tables[serverType]
	m.mu.RUnlock()

	if !ok {
		return 0
	}

	return t.Compress(route)
}

func (m *RouteManager) Decompress(serverType string, id uint16) string {
	m.mu.RLock()
	t, ok := m.tables[serverType]
	m.mu.RUnlock()

	if !ok {
		return ""
	}

	return t.Decompress(id)
}
