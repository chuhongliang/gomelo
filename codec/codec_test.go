package codec

import (
	"testing"

	"github.com/chuhongliang/gomelo/lib"
)

func TestNewJSONCodec(t *testing.T) {
	c := NewJSONCodec()
	if c == nil {
		t.Fatal("NewJSONCodec returned nil")
	}
}

func TestJSONCodec_EncodeDecode(t *testing.T) {
	c := NewJSONCodec()

	msg := &lib.Message{
		Type:  lib.Request,
		Route: "test.route",
		Seq:   12345,
		Body:  map[string]any{"name": "test", "age": 25},
	}

	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("expected Type=%d, got %d", msg.Type, decoded.Type)
	}
	if decoded.Route != msg.Route {
		t.Errorf("expected Route=%s, got %s", msg.Route, decoded.Route)
	}
	if decoded.Seq != msg.Seq {
		t.Errorf("expected Seq=%d, got %d", msg.Seq, decoded.Seq)
	}
}

func TestJSONCodec_EncodeDecode_StringBody(t *testing.T) {
	c := NewJSONCodec()

	msg := &lib.Message{
		Type:  lib.Notify,
		Route: "notify.route",
		Seq:   0,
		Body:  "simple string body",
	}

	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Route != msg.Route {
		t.Errorf("expected Route=%s, got %s", msg.Route, decoded.Route)
	}
}

func TestJSONCodec_DecodeInvalid(t *testing.T) {
	c := NewJSONCodec()

	_, err := c.Decode([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewProtobufCodec(t *testing.T) {
	c := NewProtobufCodec()
	if c == nil {
		t.Fatal("NewProtobufCodec returned nil")
	}
}

func TestProtobufCodec_RegisterRoute(t *testing.T) {
	c := NewProtobufCodec()

	id1 := c.RegisterRoute("route1")
	if id1 == 0 {
		t.Error("expected non-zero route ID")
	}

	id2 := c.RegisterRoute("route2")
	if id2 == id1 {
		t.Error("expected different route IDs")
	}

	id1Again := c.RegisterRoute("route1")
	if id1Again != id1 {
		t.Error("expected same route ID for same route")
	}
}

func TestProtobufCodec_GetRouteID(t *testing.T) {
	c := NewProtobufCodec()

	c.RegisterRoute("test.route")
	id, ok := c.GetRouteID("test.route")
	if !ok {
		t.Error("expected to find route")
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}

	_, ok = c.GetRouteID("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent route")
	}
}

func TestProtobufCodec_GetRoute(t *testing.T) {
	c := NewProtobufCodec()

	c.RegisterRoute("test.route")
	route, ok := c.GetRoute(1)
	if !ok {
		t.Error("expected to find route by ID")
	}
	if route != "test.route" {
		t.Errorf("expected 'test.route', got '%s'", route)
	}

	_, ok = c.GetRoute(999)
	if ok {
		t.Error("expected not found for nonexistent ID")
	}
}

func TestProtobufCodec_EncodeDecode_Request(t *testing.T) {
	c := NewProtobufCodec()
	c.RegisterRoute("test.route")

	msg := &lib.Message{
		Type:  lib.Request,
		Route: "test.route",
		Seq:   12345,
	}

	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := c.Decode(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("expected Type=%d, got %d", msg.Type, decoded.Type)
	}
	if decoded.Route != msg.Route {
		t.Errorf("expected Route=%s, got %s", msg.Route, decoded.Route)
	}
	if decoded.Seq != msg.Seq {
		t.Errorf("expected Seq=%d, got %d", msg.Seq, decoded.Seq)
	}
}

func TestProtobufCodec_EncodeDecode_WithoutRoute(t *testing.T) {
	c := NewProtobufCodec()

	msg := &lib.Message{
		Type:  lib.Request,
		Route: "unregistered.route",
		Seq:   12345,
	}

	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestProtobufCodec_Encode_RegisteredRouteWithNonProtoBody(t *testing.T) {
	c := NewProtobufCodec()
	c.RegisterRoute("test.route")

	msg := &lib.Message{
		Type:  lib.Request,
		Route: "test.route",
		Seq:   12345,
		Body:  "string body is not proto.Message",
	}

	_, err := c.Encode(msg)
	if err == nil {
		t.Error("expected error for non-proto.Message body with registered route")
	}
}

func TestProtobufCodec_Encode_WithoutRegisteredRoute(t *testing.T) {
	c := NewProtobufCodec()

	msg := &lib.Message{
		Type:  lib.Broadcast,
		Route: "unregistered.broadcast",
		Seq:   999,
	}

	data, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestProtobufCodec_Decode_TooShort(t *testing.T) {
	c := NewProtobufCodec()

	_, err := c.Decode([]byte("short"))
	if err == nil {
		t.Error("expected error for too short message")
	}
}

func TestNewCodec_JSON(t *testing.T) {
	c := NewCodec(CodecTypeJSON)
	if c == nil {
		t.Fatal("NewCodec returned nil")
	}
	_, ok := c.(*JSONCodec)
	if !ok {
		t.Error("expected *JSONCodec")
	}
}

func TestNewCodec_Protobuf(t *testing.T) {
	c := NewCodec(CodecTypeProtobuf)
	if c == nil {
		t.Fatal("NewCodec returned nil")
	}
	_, ok := c.(*ProtobufCodec)
	if !ok {
		t.Error("expected *ProtobufCodec")
	}
}

func TestNewCodec_Unknown(t *testing.T) {
	c := NewCodec("unknown")
	if c == nil {
		t.Fatal("NewCodec returned nil")
	}
	_, ok := c.(*JSONCodec)
	if !ok {
		t.Error("expected default *JSONCodec for unknown type")
	}
}

func TestMessageType_Constants(t *testing.T) {
	if TypeRequest != 0 {
		t.Errorf("expected TypeRequest=0, got %d", TypeRequest)
	}
	if TypeResponse != 1 {
		t.Errorf("expected TypeResponse=1, got %d", TypeResponse)
	}
	if TypeNotify != 2 {
		t.Errorf("expected TypeNotify=2, got %d", TypeNotify)
	}
	if TypeError != 3 {
		t.Errorf("expected TypeError=3, got %d", TypeError)
	}
}

func TestCodecType_Constants(t *testing.T) {
	if CodecTypeJSON != "json" {
		t.Errorf("expected CodecTypeJSON='json', got '%s'", CodecTypeJSON)
	}
	if CodecTypeProtobuf != "protobuf" {
		t.Errorf("expected CodecTypeProtobuf='protobuf', got '%s'", CodecTypeProtobuf)
	}
}

func TestProtobufCodec_Concurrent(t *testing.T) {
	c := NewProtobufCodec()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.RegisterRoute("route")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}