package connector

import (
	"sync"
	"testing"
	"time"

	"github.com/chuhongliang/gomelo/lib"
)

func TestNewServer(t *testing.T) {
	opts := &ServerOptions{
		Type:              "tcp",
		Host:              "127.0.0.1",
		Port:              0,
		MaxConns:          100,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  90 * time.Second,
	}

	s := NewServer(opts)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.opts.Type != "tcp" {
		t.Errorf("expected type=tcp, got %s", s.opts.Type)
	}
	if s.opts.MaxConns != 100 {
		t.Errorf("expected maxConns=100, got %d", s.opts.MaxConns)
	}
}

func TestNewServer_Defaults(t *testing.T) {
	s := NewServer(nil)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.opts.MaxConns != 10000 {
		t.Errorf("expected default maxConns=10000, got %d", s.opts.MaxConns)
	}
}

func TestServer_SetForwarder(t *testing.T) {
	s := NewServer(nil)
	s.SetForwarder(nil)
}

func TestServer_SetForwardSelector(t *testing.T) {
	s := NewServer(nil)
	s.SetForwardSelector(nil)
}

func TestServer_OnConnect(t *testing.T) {
	s := NewServer(nil)
	s.OnConnect(func(session *lib.Session) {
	})
	if s.onConnect == nil {
		t.Error("OnConnect handler not set")
	}
}

func TestServer_OnClose(t *testing.T) {
	s := NewServer(nil)
	s.OnClose(func(session *lib.Session) {
	})
	if s.onClose == nil {
		t.Error("OnClose handler not set")
	}
}

func TestServer_Handle(t *testing.T) {
	s := NewServer(nil)
	s.Handle("test.route", func(session *lib.Session, msg *lib.Message) (any, error) {
		return nil, nil
	})
	if s.handlers == nil {
		t.Error("handlers map not initialized")
	}
	if len(s.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(s.handlers))
	}
}

func TestServer_Name(t *testing.T) {
	s := NewServer(nil)
	name := s.Name()
	if name != "connector" {
		t.Errorf("expected name=connector, got %s", name)
	}
}

func TestServer_AddToBlackList(t *testing.T) {
	s := NewServer(nil)
	s.AddToBlackList("192.168.1.1")
	s.RemoveFromBlackList("192.168.1.1")
}

func TestServer_GetConnectionCount(t *testing.T) {
	s := NewServer(nil)
	count := s.GetConnectionCount()
	if count != 0 {
		t.Errorf("expected 0 connections, got %d", count)
	}
}

func TestSessionData_Concurrent(t *testing.T) {
	sd := &sessionData{
		heart: time.Now(),
		msgCh: make(chan *lib.Message, 10),
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sd.mu.Lock()
			sd.heart = time.Now()
			sd.mu.Unlock()
		}()
	}
	wg.Wait()
}
