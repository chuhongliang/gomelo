package lib

import "sync"

type Router struct {
	routes map[string]RouteHandler
	mu     sync.RWMutex
}

func NewRouter() *Router {
	return &Router{routes: make(map[string]RouteHandler)}
}

func (r *Router) SetRoute(serverType string, handler RouteHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[serverType] = handler
}

func (r *Router) GetRoute(serverType string) (RouteHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.routes[serverType]
	return h, ok
}

type Pipeline struct {
	middlewares  []Middleware
	handlers     map[string][]HandlerFunc
	mu           sync.RWMutex
	cache        sync.Map
	generation   uint64
}

type cacheEntry struct {
	handlers   []HandlerFunc
	generation uint64
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		middlewares: make([]Middleware, 0),
		handlers:    make(map[string][]HandlerFunc),
	}
}

func (p *Pipeline) Use(m Middleware) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.middlewares = append(p.middlewares, m)
	p.generation++
}

func (p *Pipeline) On(route string, handler HandlerFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[route] = append(p.handlers[route], handler)
	p.cache.Delete(route)
}

func (p *Pipeline) GetHandlers(route string) []HandlerFunc {
	p.mu.RLock()
	if cached, ok := p.cache.Load(route); ok {
		entry := cached.(*cacheEntry)
		if entry.generation == p.generation {
			p.mu.RUnlock()
			return entry.handlers
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if cached, ok := p.cache.Load(route); ok {
		entry := cached.(*cacheEntry)
		if entry.generation == p.generation {
			return entry.handlers
		}
	}

	handlers := p.handlers[route]
	if len(handlers) == 0 && route != "" {
		p.cache.Store(route, &cacheEntry{handlers: []HandlerFunc{}, generation: p.generation})
		return nil
	}

	var chain HandlerFunc
	for i := len(handlers) - 1; i >= 0; i-- {
		h := handlers[i]
		if chain == nil {
			chain = h
		} else {
			next := chain
			chain = func(c *Context) { h(c); next(c) }
		}
	}

	for i := len(p.middlewares) - 1; i >= 0; i-- {
		m := p.middlewares[i]
		next := chain
		chain = func(c *Context) { m(next)(c) }
	}

	p.cache.Store(route, &cacheEntry{handlers: []HandlerFunc{chain}, generation: p.generation})

	return []HandlerFunc{chain}
}

func (p *Pipeline) Invoke(ctx *Context) {
	ctx.handlers = p.GetHandlers(ctx.Route)
	ctx.Next()
}
