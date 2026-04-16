package codec

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"gomelo/lib"
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

type ProtobufCodec struct {
	protos map[string]*ProtoSchema
	routes map[string]uint16
	ids    map[uint16]string
	nextID uint16
	mu     sync.RWMutex
}

type ProtoField struct {
	Name string
	Type ProtoType
	Tag  int
}

type ProtoType int

const (
	ProtoDouble ProtoType = iota
	ProtoFloat
	ProtoInt64
	ProtoUInt64
	ProtoInt32
	ProtoFixed64
	ProtoFixed32
	ProtoBool
	ProtoString
	ProtoMessage
	ProtoBytes
	ProtoUInt32
	ProtoEmpty
)

type ProtoSchema struct {
	Name   string
	Fields []*ProtoField
}

func NewProtobufCodec() *ProtobufCodec {
	return &ProtobufCodec{
		protos: make(map[string]*ProtoSchema),
		routes: make(map[string]uint16),
		ids:    make(map[uint16]string),
	}
}

func (c *ProtobufCodec) Register(name string, fields []*ProtoField) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.protos[name] = &ProtoSchema{
		Name:   name,
		Fields: fields,
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

func (c *ProtobufCodec) Encode(msg *lib.Message) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var buf []byte

	msgType := MessageType(msg.Type)
	buf = append(buf, byte(msgType))

	if msgType == TypeRequest || msgType == TypeNotify {
		if id, ok := c.routes[msg.Route]; ok {
			buf = append(buf, 0x01)
			var idBytes [2]byte
			binary.BigEndian.PutUint16(idBytes[:], id)
			buf = append(buf, idBytes[:]...)
		} else if msg.Route != "" {
			buf = append(buf, 0x00)
			buf = append(buf, msg.Route...)
			buf = append(buf, 0)
		}
	}

	if msg.Seq != 0 {
		var idBytes [8]byte
		binary.BigEndian.PutUint64(idBytes[:], msg.Seq)
		buf = append(buf, idBytes[:]...)
	}

	if data, ok := msg.Body.([]byte); ok {
		buf = append(buf, data...)
	} else if msg.Body != nil {
		buf = append(buf, fmt.Sprintf("%v", msg.Body)...)
	}

	return buf, nil
}

func (c *ProtobufCodec) Decode(data []byte) (*lib.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(data) < 2 {
		return nil, fmt.Errorf("invalid message")
	}

	msg := &lib.Message{
		Type: lib.MessageType(data[0]),
	}

	flag := data[1]
	offset := 2

	if flag == 0x01 {
		if len(data) < offset+2 {
			return nil, fmt.Errorf("invalid route id")
		}
		routeID := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		if r, ok := c.ids[routeID]; ok {
			msg.Route = r
		}
	} else if flag == 0x00 {
		for i := offset; i < len(data); i++ {
			if data[i] == 0 {
				msg.Route = string(data[offset:i])
				offset = i + 1
				break
			}
		}
	}

	if offset < len(data) {
		if len(data) >= offset+8 {
			msg.Seq = binary.BigEndian.Uint64(data[offset : offset+8])
			offset += 8
		}
	}

	if offset < len(data) {
		msg.Body = data[offset:]
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

type CodecType string

const (
	CodecTypeJSON     CodecType = "json"
	CodecTypeProtobuf CodecType = "protobuf"
)

func NewCodec(t CodecType) Codec {
	switch t {
	case CodecTypeProtobuf:
		return NewProtobufCodec()
	default:
		return NewJSONCodec()
	}
}
