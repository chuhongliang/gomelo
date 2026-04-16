package lib

type RouterRegistry struct {
	router   *Router
	pipeline *Pipeline
}

func NewRouterRegistry() *RouterRegistry {
	return &RouterRegistry{
		router:   NewRouter(),
		pipeline: NewPipeline(),
	}
}

func (r *RouterRegistry) SetRoute(serverType string, handler RouteHandler) {
	r.router.SetRoute(serverType, handler)
}

func (r *RouterRegistry) GetRoute(serverType string) (RouteHandler, bool) {
	return r.router.GetRoute(serverType)
}

func (r *RouterRegistry) Use(m Middleware) {
	r.pipeline.Use(m)
}

func (r *RouterRegistry) On(route string, h HandlerFunc) {
	r.pipeline.On(route, h)
}

func (r *RouterRegistry) Router() *Router {
	return r.router
}

func (r *RouterRegistry) Pipeline() *Pipeline {
	return r.pipeline
}
