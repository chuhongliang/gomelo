package forward

import (
	"fmt"
	"sync"
	"time"

	"gomelo/lib"
	"gomelo/rpc"
	"gomelo/selector"
	"gomelo/server_registry"
)

type MessageForwarder interface {
	Forward(session *lib.Session, msg *lib.Message, server server_registry.ServerInfo) error
	Start() error
	Stop()
}

type forwarder struct {
	app        *lib.App
	selector   selector.Selector
	rpcClients map[string]rpc.RPCClient
	mu         sync.RWMutex
	running    bool
}

func NewForwarder(app *lib.App, sel selector.Selector) MessageForwarder {
	return &forwarder{
		app:        app,
		selector:   sel,
		rpcClients: make(map[string]rpc.RPCClient),
	}
}

func (f *forwarder) Start() error {
	f.running = true
	return nil
}

func (f *forwarder) Stop() {
	f.running = false
	f.mu.Lock()
	for _, c := range f.rpcClients {
		c.Close()
	}
	f.rpcClients = make(map[string]rpc.RPCClient)
	f.mu.Unlock()
}

func (f *forwarder) Forward(session *lib.Session, msg *lib.Message, server server_registry.ServerInfo) error {
	if !f.app.IsFrontend() {
		return nil
	}

	forwardBody := map[string]any{
		"uid":   session.UID(),
		"route": msg.Route,
		"body":  msg.Body,
	}

	return f.doForward(server, msg.Route, forwardBody)
}

func (f *forwarder) doForward(server server_registry.ServerInfo, route string, body any) error {
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

	return client.Notify(service, method, body)
}

func (f *forwarder) getServerType(route string) string {
	parts := splitRoute(route)
	if len(parts) > 0 {
		return parts[0]
	}
	return "chat"
}

func splitRoute(route string) []string {
	var result []string
	var current []byte

	for _, c := range route {
		if c == '.' {
			if len(current) > 0 {
				result = append(result, string(current))
				current = nil
			}
		} else {
			current = append(current, byte(c))
		}
	}

	if len(current) > 0 {
		result = append(result, string(current))
	}

	return result
}

func (f *forwarder) getOrCreateClient(server server_registry.ServerInfo) (rpc.RPCClient, error) {
	key := fmt.Sprintf("%s:%d", server.Host, server.Port)

	f.mu.RLock()
	client, ok := f.rpcClients[key]
	f.mu.RUnlock()

	if ok {
		return client, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if client, ok := f.rpcClients[key]; ok {
		return client, nil
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

	f.rpcClients[key] = client
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

func (m *ForwardManager) Forward(session *lib.Session, msg *lib.Message) error {
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
		m.forwarders[serverType] = forwarder
	}
	m.mu.Unlock()

	return forwarder.Forward(session, msg, server)
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
