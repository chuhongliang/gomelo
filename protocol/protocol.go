package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

const (
	TypeRequest  = 0
	TypeNotify   = 1
	TypeResponse = 2
	TypePush     = 3
)

type Message struct {
	Type  int             `json:"type"`
	Route string          `json:"route"`
	Seq   uint64          `json:"seq,omitempty"`
	Body  json.RawMessage `json:"body,omitempty"`
	Code  int             `json:"code,omitempty"`
	Msg   string          `json:"msg,omitempty"`
}

func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func Decode(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

type Request struct {
	Route string         `json:"route"`
	Seq   uint64         `json:"seq"`
	Data  map[string]any `json:"data"`
}

func (r *Request) Encode() []byte {
	data, _ := json.Marshal(r)
	return EncodeFrame(data)
}

type Response struct {
	Seq  uint64         `json:"seq"`
	Code int            `json:"code"`
	Msg  string         `json:"msg,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

func (r *Response) Encode() []byte {
	data, _ := json.Marshal(r)
	return EncodeFrame(data)
}

type Push struct {
	Route string         `json:"route"`
	Data  map[string]any `json:"data"`
}

func (p *Push) Encode() []byte {
	data, _ := json.Marshal(p)
	return EncodeFrame(data)
}

func EncodeFrame(data []byte) []byte {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	return append(header, data...)
}

func DecodeFrame(buf []byte) ([]byte, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("buffer too short")
	}
	size := binary.BigEndian.Uint32(buf[:4])
	if int(size) > len(buf)-4 {
		return nil, fmt.Errorf("incomplete frame")
	}
	return buf[4 : 4+size], nil
}

func ReadMessage(conn interface {
	Read([]byte) (int, error)
	SetReadDeadline(time.Time) error
}) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := conn.Read(header); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header)
	if size > 64*1024 {
		return nil, fmt.Errorf("message too large")
	}
	data := make([]byte, size)
	if _, err := conn.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}

func WriteMessage(conn interface {
	Write([]byte) (int, error)
	SetWriteDeadline(time.Time) error
}, msg *Message) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}
	_, err = conn.Write(EncodeFrame(data))
	return err
}

type HandshakeReq struct {
	Token string `json:"token"`
}

type HandshakeResp struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	User map[string]any `json:"user"`
}

type HeartbeatReq struct {
	TS int64 `json:"ts"`
}

func NewRequest(route string, seq uint64, data map[string]any) *Message {
	body, _ := json.Marshal(data)
	return &Message{
		Type:  TypeRequest,
		Route: route,
		Seq:   seq,
		Body:  body,
	}
}

func NewNotify(route string, data map[string]any) *Message {
	body, _ := json.Marshal(data)
	return &Message{
		Type:  TypeNotify,
		Route: route,
		Body:  body,
	}
}

func NewResponse(seq uint64, code int, msg string, data map[string]any) *Message {
	body, _ := json.Marshal(data)
	return &Message{
		Type: TypeResponse,
		Seq:  seq,
		Code: code,
		Msg:  msg,
		Body: body,
	}
}

func NewPush(route string, data map[string]any) *Message {
	body, _ := json.Marshal(data)
	return &Message{
		Type:  TypePush,
		Route: route,
		Body:  body,
	}
}
