package rpc

import (
	"testing"
	"time"
)

func TestClientOptions_GetMaxResponseSize(t *testing.T) {
	opts := &ClientOptions{MaxResponseSize: 0}
	if got := opts.getMaxResponseSize(); got != 1024*1024 {
		t.Errorf("expected 1MB, got %d", got)
	}

	opts = &ClientOptions{MaxResponseSize: 2048}
	if got := opts.getMaxResponseSize(); got != 2048 {
		t.Errorf("expected 2048, got %d", got)
	}
}

func TestClientOptions_Defaults(t *testing.T) {
	opts := &ClientOptions{}
	if opts.MaxConns != 0 {
		t.Errorf("expected MaxConns=0, got %d", opts.MaxConns)
	}
	if opts.Timeout != 0 {
		t.Errorf("expected Timeout=0, got %v", opts.Timeout)
	}
}

func TestRPCRequest(t *testing.T) {
	req := &rpcRequest{
		Seq:     1,
		Type:    "invoke",
		Service: "TestService",
		Method:  "TestMethod",
		Args:    map[string]string{"key": "value"},
	}

	if req.Seq != 1 {
		t.Errorf("expected Seq=1, got %d", req.Seq)
	}
	if req.Service != "TestService" {
		t.Errorf("expected Service=TestService, got %s", req.Service)
	}
}

func TestRPCResponse(t *testing.T) {
	resp := &rpcResponse{
		Seq:   1,
		Error: "",
		Reply: map[string]string{"result": "ok"},
	}

	if resp.Seq != 1 {
		t.Errorf("expected Seq=1, got %d", resp.Seq)
	}
	if resp.Error != "" {
		t.Errorf("expected empty error, got %s", resp.Error)
	}
}

func TestRPCFuture(t *testing.T) {
	f := &rpcFuture{
		reply: "result",
		err:   nil,
		done:  make(chan struct{}),
	}

	if f.reply != "result" {
		t.Errorf("expected reply=result, got %v", f.reply)
	}
	if f.err != nil {
		t.Errorf("expected nil err, got %v", f.err)
	}
}

func TestNewPoolClient_Defaults(t *testing.T) {
	p := newPoolClient("localhost:8080", nil)
	if p == nil {
		t.Fatal("newPoolClient returned nil")
	}
	if p.maxConns != 10 {
		t.Errorf("expected maxConns=10, got %d", p.maxConns)
	}
	if p.minConns != 1 {
		t.Errorf("expected minConns=1, got %d", p.minConns)
	}
}

func TestNewPoolClient_CustomOpts(t *testing.T) {
	opts := &ClientOptions{
		MaxConns:  20,
		MinConns:  5,
		Timeout:   10 * time.Second,
		KeepAlive: 120 * time.Second,
		IdleTime:  600 * time.Second,
	}

	p := newPoolClient("localhost:8080", opts)
	if p == nil {
		t.Fatal("newPoolClient returned nil")
	}
	if p.maxConns != 20 {
		t.Errorf("expected maxConns=20, got %d", p.maxConns)
	}
	if p.minConns != 5 {
		t.Errorf("expected minConns=5, got %d", p.minConns)
	}
}

func TestPoolClient_Close(t *testing.T) {
	p := newPoolClient("localhost:8080", nil)
	p.Close()
}

func TestPoolClient_CloseTwice(t *testing.T) {
	p := newPoolClient("localhost:8080", nil)
	p.Close()
	p.Close()
}

func TestPoolClient_Addr(t *testing.T) {
	p := newPoolClient("localhost:8080", nil)
	if addr := p.Addr(); addr != "localhost:8080" {
		t.Errorf("expected localhost:8080, got %s", addr)
	}
}
