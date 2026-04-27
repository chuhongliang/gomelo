package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

type CodecType string

const (
	CodecJSON     CodecType = "json"
	CodecProtobuf CodecType = "protobuf"
)

type RouteSchema struct {
	Route   string    `json:"route"`
	ID      uint16    `json:"id"`
	Codec   CodecType `json:"codec"`
	TypeURL string    `json:"typeUrl,omitempty"`
}

type ServerSchema struct {
	ServerID   string        `json:"serverId"`
	ServerType string        `json:"serverType"`
	Routes     []RouteSchema `json:"routes"`
}

type SchemaMessage struct {
	Type string       `json:"type"`
	Data ServerSchema `json:"data"`
}

type Manager struct {
	routes     map[string]RouteSchema
	routeIDs   map[uint16]RouteSchema
	serverID   string
	serverType string
}

func NewManager(serverID, serverType string) *Manager {
	return &Manager{
		routes:     make(map[string]RouteSchema),
		routeIDs:   make(map[uint16]RouteSchema),
		serverID:   serverID,
		serverType: serverType,
	}
}

func (s *Manager) RegisterRoute(route string, id uint16, codec CodecType, typeURL ...string) error {
	if route == "" {
		return fmt.Errorf("route cannot be empty")
	}
	if id == 0 {
		return fmt.Errorf("route id cannot be 0")
	}
	if _, exists := s.routes[route]; exists {
		return fmt.Errorf("route %s already registered", route)
	}
	if _, exists := s.routeIDs[id]; exists {
		return fmt.Errorf("route id %d already registered", id)
	}

	schema := RouteSchema{
		Route: route,
		ID:    id,
		Codec: codec,
	}
	if len(typeURL) > 0 {
		schema.TypeURL = typeURL[0]
	}

	s.routes[route] = schema
	s.routeIDs[id] = schema
	return nil
}

func (s *Manager) GetRouteSchema(route string) (RouteSchema, bool) {
	schema, ok := s.routes[route]
	return schema, ok
}

func (s *Manager) GetRouteSchemaByID(id uint16) (RouteSchema, bool) {
	schema, ok := s.routeIDs[id]
	return schema, ok
}

func (s *Manager) GetAllRoutes() []RouteSchema {
	routes := make([]RouteSchema, 0, len(s.routes))
	for _, schema := range s.routes {
		routes = append(routes, schema)
	}
	return routes
}

func (s *Manager) GetServerSchema() ServerSchema {
	return ServerSchema{
		ServerID:   s.serverID,
		ServerType: s.serverType,
		Routes:     s.GetAllRoutes(),
	}
}

func (s *Manager) GetSchemaMessage() (map[string]any, error) {
	schema := s.GetServerSchema()
	return map[string]any{
		"type": "schema",
		"data": schema,
	}, nil
}

func ParseSchemaMessage(data []byte) (*ServerSchema, error) {
	var msg SchemaMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if msg.Type != "schema" {
		return nil, fmt.Errorf("invalid schema message type: %s", msg.Type)
	}
	return &msg.Data, nil
}

func ParseRouteSchema(data []byte) (*RouteSchema, error) {
	var schema RouteSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

func (s *Manager) RegisterFromServerSchema(serverSchema *ServerSchema) error {
	for _, route := range serverSchema.Routes {
		if err := s.RegisterRoute(route.Route, route.ID, route.Codec, route.TypeURL); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Route, err)
		}
	}
	return nil
}

func (s *Manager) Clear() {
	s.routes = make(map[string]RouteSchema)
	s.routeIDs = make(map[uint16]RouteSchema)
}

func (s *Manager) ServerID() string {
	return s.serverID
}

func (s *Manager) ServerType() string {
	return s.serverType
}

func (s *Manager) SetServerInfo(serverID, serverType string) {
	s.serverID = serverID
	s.serverType = serverType
}

func (s *Manager) Merge(other *Manager) error {
	for _, route := range other.GetAllRoutes() {
		if err := s.RegisterRoute(route.Route, route.ID, route.Codec, route.TypeURL); err != nil {
			if strings.Contains(err.Error(), "already registered") {
				continue
			}
			return err
		}
	}
	return nil
}
