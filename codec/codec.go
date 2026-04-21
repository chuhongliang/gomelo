package codec

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/chuhongliang/gomelo/lib"

	"google.golang.org/protobuf/proto"
)

type MessageType int

const (
	TypeRequest MessageType = iota
	TypeResponse
	TypeNotify
	TypeError
)

type Codec interface {
	Encode(msg *lib.Message) ([]byte, error)
	Decode(data []byte) (*lib.Message, error)
}

type JSONCodec struct{}

func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

func (c *JSONCodec) Encode(msg *lib.Message) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *JSONCodec) Decode(data []byte) (*lib.Message, error) {
	msg := &lib.Message{}
	if err := json.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

type ProtobufCodec struct {
	routes map[string]uint16
	ids    map[uint16]string
	types  map[string]reflect.Type
	nextID uint16
	mu     sync.RWMutex
}

func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{
		routes: make(map[string]uint16),
		ids:    make(map[uint16]string),
		types:  make(map[string]reflect.Type),
	}
}

func (c *ProtobufCodec) RegisterRoute(route string) uint16 {
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

func (c *ProtobufCodec) RegisterType(route string, msg proto.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.types[route] = reflect.TypeOf(msg)
}

func (c *ProtobufCodec) getType(route string) reflect.Type {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.types[route]
}

func (c *ProtobufCodec) Encode(msg *lib.Message) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	msgType := byte(msg.Type)
	routeID, hasRoute := c.routes[msg.Route]
	seq := msg.Seq

	var body []byte
	var err error

	if msg.Body != nil {
		if p, ok := msg.Body.(proto.Message); ok {
			body, err = proto.Marshal(p)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
		} else {
			return nil, fmt.Errorf("body must implement proto.Message")
		}
	}

	var headerLen int
	if hasRoute {
		headerLen = 1 + 2 + 8
	} else {
		headerLen = 1 + 1 + len(msg.Route) + 1 + 8
	}

	result := make([]byte, 0, headerLen+len(body))
	result = append(result, msgType)

	if hasRoute {
		result = append(result, 0x01)
		var idBytes [2]byte
		binary.BigEndian.PutUint16(idBytes[:], routeID)
		result = append(result, idBytes[:]...)
	} else {
		result = append(result, 0x00)
		result = append(result, msg.Route...)
		result = append(result, 0)
	}

	var seqBytes [8]byte
	binary.BigEndian.PutUint64(seqBytes[:], seq)
	result = append(result, seqBytes[:]...)
	result = append(result, body...)

	return result, nil
}

func (c *ProtobufCodec) Decode(data []byte) (*lib.Message, error) {
	c.mu.RLock()
	ids := c.ids
	types := c.types
	c.mu.RUnlock()

	if len(data) < 10 {
		return nil, fmt.Errorf("message too short")
	}

	msg := &lib.Message{
		Type: lib.MessageType(data[0]),
	}

	offset := 1
	flag := data[offset]
	offset++

	var route string
	if flag == 0x01 {
		if len(data) < offset+2 {
			return nil, fmt.Errorf("invalid route id")
		}
		routeID := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2
		route = ids[routeID]
	} else if flag == 0x00 {
		for i := offset; i < len(data); i++ {
			if data[i] == 0 {
				route = string(data[offset:i])
				offset = i + 1
				break
			}
		}
	}
	msg.Route = route

	if len(data) < offset+8 {
		return nil, fmt.Errorf("invalid seq")
	}
	msg.Seq = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset < len(data) {
		bodyBytes := data[offset:]
		if t, ok := types[route]; ok {
			instance := reflect.New(t).Interface().(proto.Message)
			if err := proto.Unmarshal(bodyBytes, instance); err != nil {
				return nil, fmt.Errorf("unmarshal body for route %s: %w", route, err)
			}
			msg.Body = instance
		} else {
			msg.Body = bodyBytes
		}
	}

	return msg, nil
}

func (c *ProtobufCodec) GetRouteID(route string) (uint16, bool) {
	id, ok := c.routes[route]
	return id, ok
}

func (c *ProtobufCodec) GetRoute(id uint16) (string, bool) {
	route, ok := c.ids[id]
	return route, ok
}

type CodecType string

const (
	CodecTypeJSON     CodecType = "json"
	CodecTypeProtobuf CodecType = "protobuf"
)

func NewCodec(t CodecType) Codec {
	switch t {
	case CodecTypeProtobuf:
		return NewProtobufCodec()
	case CodecTypeJSON:
		return NewJSONCodec()
	default:
		return NewJSONCodec()
	}
}
