package forward

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gomelo/lib"
	routelib "gomelo/route"
	"gomelo/rpc"
	"gomelo/selector"
	"gomelo/server_registry"
)

type MessageForwarder interface {
	Forward(ctx context.Context, session *lib.Session, msg *lib.Message, server server_registry.ServerInfo) error
	Start() error
	Stop()
}

type forwarder struct {
	app        *lib.App
	selector   selector.Selector
	rpcClients sync.Map
	mu         sync.RWMutex
	running    atomic.Bool
}

func NewForwarder(app *lib.App, sel selector.Selector) MessageForwarder {
	return &forwarder{
		app:      app,
		selector: sel,
	}
}

func (f *forwarder) Start() error {
	f.running.Store(true)
	return nil
}

func (f *forwarder) Stop() {
	f.running.Store(false)
	f.mu.Lock()
	f.rpcClients.Range(func(key, value any) bool {
		if client, ok := value.(rpc.RPCClient); ok {
			client.Close()
		}
		return true
	})
	f.rpcClients = sync.Map{}
	f.mu.Unlock()
}

func (f *forwarder) Forward(ctx context.Context, session *lib.Session, msg *lib.Message, server server_registry.ServerInfo) error {
	if !f.running.Load() {
		return nil
	}
	if !f.app.IsFrontend() {
		return nil
	}

	forwardBody := map[string]any{
		"uid":   session.UID(),
		"route": msg.Route,
		"body":  msg.Body,
	}

	return f.doForward(ctx, server, msg.Route, forwardBody)
}

func (f *forwarder) doForward(ctx context.Context, server server_registry.ServerInfo, route string, body any) error {
	client, err := f.getOrCreateClient(server)
	if err != nil {
		return err
	}

	parts := splitRoute(route)
	if len(parts) < 2 {
		return fmt.Errorf("invalid route: %s", route)
	}
	service := parts[0]
	method := parts[1]

	invokeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return client.InvokeCtx(invokeCtx, service, method, body, nil)
}

func splitRoute(route string) []string {
	return routelib.SplitRoute(route)
}

func (f *forwarder) getOrCreateClient(server server_registry.ServerInfo) (rpc.RPCClient, error) {
	key := fmt.Sprintf("%s:%d", server.Host, server.Port)

	if client, ok := f.rpcClients.Load(key); ok {
		return client.(rpc.RPCClient), nil
	}

	client, err := rpc.NewClient(&rpc.ClientOptions{
		Host:     server.Host,
		Port:     server.Port,
		MaxConns: 5,
		MinConns: 1,
		Timeout:  5 * time.Second,
	})

	if err != nil {
		return nil, err
	}

	f.rpcClients.Store(key, client)
	return client, nil
}

type ForwardRule struct {
	Route      string
	ServerType string
}

type ForwardManager struct {
	rules      []ForwardRule
	app        *lib.App
	selector   selector.Selector
	forwarders map[string]MessageForwarder
	mu         sync.RWMutex
}

func NewForwardManager(app *lib.App, sel selector.Selector) *ForwardManager {
	return &ForwardManager{
		app:        app,
		selector:   sel,
		forwarders: make(map[string]MessageForwarder),
	}
}

func (m *ForwardManager) AddRule(route, serverType string) {
	m.mu.Lock()
	m.rules = append(m.rules, ForwardRule{Route: route, ServerType: serverType})
	m.mu.Unlock()
}

func (m *ForwardManager) Forward(ctx context.Context, session *lib.Session, msg *lib.Message) error {
	serverType := m.matchServerType(msg.Route)
	if serverType == "" {
		return fmt.Errorf("no server type matched for route: %s", msg.Route)
	}

	server := m.selector.Select(serverType)
	if server.ID == "" {
		return fmt.Errorf("no server available for type: %s", serverType)
	}

	m.mu.Lock()
	forwarder, ok := m.forwarders[serverType]
	if !ok {
		forwarder = NewForwarder(m.app, m.selector)
		forwarder.Start()
		m.forwarders[serverType] = forwarder
	}
	m.mu.Unlock()

	return forwarder.Forward(ctx, session, msg, server)
}

func (m *ForwardManager) matchServerType(route string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.rules {
		if route == rule.Route || hasPrefix(route, rule.Route) {
			return rule.ServerType
		}
	}

	parts := splitRoute(route)
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
