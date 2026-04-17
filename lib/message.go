package lib

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"sync"
)

type MessageType int

const (
	Request MessageType = iota
	Response
	Notify
	Broadcast
)

type Message struct {
	Type  MessageType
	Route string
	Seq   uint64
	Body  any
}

type MessageCodec interface {
	Encode(msg *Message) ([]byte, error)
	Decode(data []byte) (*Message, error)
	EncodeBody(msg *Message) ([]byte, error)
	DecodeBody(data []byte, v any) error
}

type jsonMessageCodec struct{}

func (c *jsonMessageCodec) Encode(msg *Message) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *jsonMessageCodec) Decode(data []byte) (*Message, error) {
	msg := &Message{}
	if err := json.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (c *jsonMessageCodec) EncodeBody(msg *Message) ([]byte, error) {
	return json.Marshal(msg.Body)
}

func (c *jsonMessageCodec) DecodeBody(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

var defaultCodec MessageCodec = &jsonMessageCodec{}

func SetMessageCodec(c MessageCodec) { defaultCodec = c }
func GetMessageCodec() MessageCodec  { return defaultCodec }

func (m *Message) Encode() ([]byte, error) {
	return defaultCodec.Encode(m)
}

func (m *Message) Decode(data []byte) error {
	msg, err := defaultCodec.Decode(data)
	if err != nil {
		return err
	}
	m.Type = msg.Type
	m.Route = msg.Route
	m.Seq = msg.Seq
	m.Body = msg.Body
	return nil
}

func (m *Message) EncodeBody() ([]byte, error) {
	return defaultCodec.EncodeBody(m)
}

func (m *Message) DecodeBody(v any) error {
	data, ok := m.Body.([]byte)
	if !ok {
		data, err := json.Marshal(m.Body)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, v)
	}
	return json.Unmarshal(data, v)
}

type Connection interface {
	ID() uint64
	Close()
	Send(msg *Message) error
	RemoteAddr() net.Addr
}

type CodecConnection struct {
	id     uint64
	conn   net.Conn
	ch     chan *Message
	closed bool
	mu     sync.Mutex
	codec  MessageCodec
}

func NewCodecConnection(id uint64, conn net.Conn) *CodecConnection {
	return &CodecConnection{
		id:    id,
		conn:  conn,
		ch:    make(chan *Message, 1024),
		codec: defaultCodec,
	}
}

func (c *CodecConnection) SetCodec(codec MessageCodec) { c.codec = codec }
func (c *CodecConnection) ID() uint64                  { return c.id }
func (c *CodecConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.conn.Close()
}
func (c *CodecConnection) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

func (c *CodecConnection) Send(msg *Message) error {
	data, err := c.codec.Encode(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	_, err = c.conn.Write(append(header[:], data...))
	return err
}

type TCPConnection = CodecConnection
