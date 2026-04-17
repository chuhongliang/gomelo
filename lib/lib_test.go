package lib

import (
	"sync"
	"testing"
	"time"
)

func TestEventEmitter(t *testing.T) {
	emitter := NewEventEmitter()

	var mu sync.Mutex
	calls := 0
	handler := func(args ...any) {
		mu.Lock()
		calls++
		mu.Unlock()
	}

	id := emitter.On("test", handler)
	if id == 0 {
		t.Error("expected non-zero event id")
	}

	emitter.Emit("test")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	mu.Unlock()

	emitter.Emit("test")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
	mu.Unlock()

	emitter.Off("test", id)
	emitter.Emit("test")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if calls != 2 {
		t.Errorf("after off, expected 2 calls, got %d", calls)
	}
	mu.Unlock()
}

func TestEventEmitterOnce(t *testing.T) {
	emitter := NewEventEmitter()

	var mu sync.Mutex
	calls := 0
	emitter.Once("test", func(args ...any) {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	emitter.Emit("test")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	mu.Unlock()

	emitter.Emit("test")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if calls != 1 {
		t.Errorf("once should only trigger once, got %d", calls)
	}
	mu.Unlock()
}

func TestEventEmitterClear(t *testing.T) {
	emitter := NewEventEmitter()

	calls := 0
	emitter.On("test", func(args ...any) { calls++ })

	emitter.Clear("test")
	emitter.Emit("test")
	time.Sleep(10 * time.Millisecond)
	if calls != 0 {
		t.Error("clear should remove all handlers")
	}
}

func TestSessionStorage(t *testing.T) {
	storage := &SessionStorage{
		kv: map[string]any{
			"name": "test",
			"age":  25,
		},
	}

	if storage.Get("name") != "test" {
		t.Errorf("expected 'test', got %v", storage.Get("name"))
	}

	storage.Set("name", "updated")
	if storage.Get("name") != "updated" {
		t.Error("set failed")
	}

	cp := storage.DeepCopy()
	if cp.Get("name") != "updated" {
		t.Error("deep copy failed")
	}

	cp.Set("name", "modified")
	if storage.Get("name") != "updated" {
		t.Error("deep copy should be independent")
	}
}

func TestSession(t *testing.T) {
	session := NewSession()
	session.SetID(123)
	session.SetUID("user-001")
	session.Set("key", "value")

	if session.ID() != 123 {
		t.Errorf("expected id 123, got %d", session.ID())
	}

	if session.UID() != "user-001" {
		t.Errorf("expected uid 'user-001', got %s", session.UID())
	}

	if session.Get("key") != "value" {
		t.Error("get failed")
	}

	if session.IsClosed() {
		t.Error("new session should not be closed")
	}
}

func TestMessage(t *testing.T) {
	msg := &Message{
		Type:  Request,
		Route: "connector.entryHandler.entry",
		Seq:   1,
		Body:  []byte(`{"name":"test"}`),
	}

	var req struct {
		Name string `json:"name"`
	}

	err := msg.DecodeBody(&req)
	if err != nil {
		t.Errorf("decode failed: %v", err)
	}

	if req.Name != "test" {
		t.Errorf("expected 'test', got %s", req.Name)
	}
}

func TestContext(t *testing.T) {
	app := NewApp()
	session := NewSession()
	session.SetID(1)

	ctx := NewContext(app)
	ctx.SetSession(session)
	ctx.Route = "test.route"

	if ctx.Session() != session {
		t.Error("session not set correctly")
	}

	if ctx.Route != "test.route" {
		t.Error("route not set correctly")
	}
}

func TestPipelineUse(t *testing.T) {
	pipeline := NewPipeline()

	pipeline.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			c.Set("middleware", true)
			next(c)
		}
	})

	if len(pipeline.middlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(pipeline.middlewares))
	}
}

func TestApp(t *testing.T) {
	app := NewApp()

	if app.state != StateInited {
		t.Errorf("expected state %d, got %d", StateInited, app.state)
	}

	app.SetServerType("gate")
	if app.GetServerType() != "gate" {
		t.Error("set server type failed")
	}
}

func TestNextEventID(t *testing.T) {
	id1 := NextEventID()
	id2 := NextEventID()

	if id1 >= id2 {
		t.Error("next event id should increment")
	}
}
