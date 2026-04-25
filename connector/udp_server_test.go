package connector

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/chuhongliang/gomelo/lib"
)

func TestNewUDPServer(t *testing.T) {
	opts := &UDPServerOptions{
		Host:              "127.0.0.1",
		Port:              0,
		MaxConns:          100,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  90 * time.Second,
	}

	s := NewUDPServer(opts)
	if s == nil {
		t.Fatal("NewUDPServer returned nil")
	}
	if s.opts.Host != "127.0.0.1" {
		t.Errorf("expected host=127.0.0.1, got %s", s.opts.Host)
	}
	if s.opts.MaxConns != 100 {
		t.Errorf("expected maxConns=100, got %d", s.opts.MaxConns)
	}
}

func TestNewUDPServer_Defaults(t *testing.T) {
	s := NewUDPServer(nil)
	if s == nil {
		t.Fatal("NewUDPServer returned nil")
	}
	if s.opts.MaxConns != 10000 {
		t.Errorf("expected default maxConns=10000, got %d", s.opts.MaxConns)
	}
}

func TestUDPServer_SetApp(t *testing.T) {
	s := NewUDPServer(nil)
	s.SetApp(nil)
}

func TestUDPServer_SetType(t *testing.T) {
	s := NewUDPServer(nil)
	s.SetType("connector-udp")
}

func TestUDPServer_SetMaxConns(t *testing.T) {
	s := NewUDPServer(nil)
	s.SetMaxConns(500)
}

func TestUDPServer_SetHeartbeat(t *testing.T) {
	s := NewUDPServer(nil)
	s.SetHeartbeat(60*time.Second, 180*time.Second)
}

func TestUDPServer_OnConnect(t *testing.T) {
	s := NewUDPServer(nil)
	s.OnConnect(func(session *lib.Session) {
	})
}

func TestUDPServer_OnMessage(t *testing.T) {
	s := NewUDPServer(nil)
	s.OnMessage(func(session *lib.Session, msg *lib.Message) {
	})
}

func TestUDPServer_OnClose(t *testing.T) {
	s := NewUDPServer(nil)
	s.OnClose(func(session *lib.Session) {
	})
}

func TestUDPServer_GetSessionCount(t *testing.T) {
	s := NewUDPServer(nil)
	count := s.GetSessionCount()
	if count != 0 {
		t.Errorf("expected 0 sessions, got %d", count)
	}
}

func TestUDPServer_Broadcast(t *testing.T) {
	s := NewUDPServer(nil)
	msg := &lib.Message{}
	if err := s.Broadcast("route", msg); err != nil {
		t.Errorf("Broadcast error: %v", err)
	}
}

func TestUDPServer_GetSession(t *testing.T) {
	s := NewUDPServer(nil)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	session := s.GetSession(addr)
	if session != nil {
		t.Error("expected nil for non-existent session")
	}
}

func TestUDPServer_RemoveSession(t *testing.T) {
	s := NewUDPServer(nil)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	s.RemoveSession(addr)
}

func TestUDPServer_SetForwarder(t *testing.T) {
	s := NewUDPServer(nil)
	s.SetForwarder(nil)
}

func TestUDPServer_SetForwardSelector(t *testing.T) {
	s := NewUDPServer(nil)
	s.SetForwardSelector(nil)
}

func TestUDPServer_Forward_NoForwarder(t *testing.T) {
	s := NewUDPServer(nil)
	msg := &lib.Message{}
	err := s.Forward("route", msg)
	if err == nil {
		t.Error("expected error when no forwarder configured")
	}
}

func TestUDPServer_Stop(t *testing.T) {
	s := NewUDPServer(nil)
	s.Stop()
}

func TestUDPServer_StopTwice(t *testing.T) {
	s := NewUDPServer(nil)
	s.Stop()
	s.Stop()
}

func TestSessionKey(t *testing.T) {
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	key := sessionKey(addr)
	if key != "127.0.0.1:12345" {
		t.Errorf("expected '127.0.0.1:12345', got '%s'", key)
	}
}

func TestUDPServer_Concurrent(t *testing.T) {
	s := NewUDPServer(nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SetMaxConns(100)
			s.GetSessionCount()
		}()
	}
	wg.Wait()
}
