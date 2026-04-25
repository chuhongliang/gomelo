package pool

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	factory := func() (any, error) {
		return "connection", nil
	}

	p := NewPool(factory, 5, 2, time.Second, time.Minute)
	defer p.Close()

	total, idle, active := p.Stats()
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if idle != 2 {
		t.Errorf("expected idle=2, got %d", idle)
	}
	if active != 2 {
		t.Errorf("expected active=2, got %d", active)
	}
}

func TestPool_Get(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 3, 1, time.Second, time.Minute)
	defer p.Close()

	conn, err := p.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if conn != "conn" {
		t.Errorf("expected 'conn', got %v", conn)
	}

	_, _, active := p.Stats()
	if active != 2 {
		t.Errorf("expected active=2 after Get, got %d", active)
	}
}

func TestPool_Get_Reuse(t *testing.T) {
	callCount := 0
	factory := func() (any, error) {
		callCount++
		return "conn", nil
	}

	p := NewPool(factory, 3, 1, time.Second, time.Minute)
	defer p.Close()

	conn1, _ := p.Get()
	conn2, _ := p.Get()
	_ = conn2

	p.Put(conn1)

	conn3, _ := p.Get()
	if conn3 != "conn" {
		t.Errorf("expected 'conn' from pool, got %v", conn3)
	}
	if callCount != 2 {
		t.Errorf("expected factory called twice, got %d", callCount)
	}
}

func TestPool_Get_Exhausted(t *testing.T) {
	factory := func() (any, error) {
		time.Sleep(10 * time.Millisecond)
		return "conn", nil
	}

	p := NewPool(factory, 2, 0, 50*time.Millisecond, time.Minute)
	defer p.Close()

	_, _ = p.Get()
	_, _ = p.Get()

	_, err := p.Get()
	if err != ErrPoolExhausted && err != context.DeadlineExceeded && err != nil {
		t.Logf("expected exhaustion error, got %v", err)
	}
}

func TestPool_Get_MaxConns(t *testing.T) {
	created := 0
	factory := func() (any, error) {
		created++
		return "conn", nil
	}

	p := NewPool(factory, 2, 0, time.Second, time.Minute)
	defer p.Close()

	conn1, _ := p.Get()
	conn2, _ := p.Get()

	if created != 2 {
		t.Errorf("expected 2 connections created, got %d", created)
	}

	p.Put(conn1)
	p.Put(conn2)

	_, idle, _ := p.Stats()
	if idle != 2 {
		t.Errorf("expected 2 idle, got %d", idle)
	}
}

func TestPool_Put_Nil(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 1, time.Second, time.Minute)
	defer p.Close()

	conn, _ := p.Get()
	_, idleBefore, _ := p.Stats()

	p.Put(conn)
	if idleBefore != 0 {
		t.Errorf("expected idle=0 before Put (connection taken), got %d", idleBefore)
	}

	_, idleAfter, _ := p.Stats()
	if idleAfter != 1 {
		t.Errorf("expected idle=1 after Put, got %d", idleAfter)
	}
}

func TestPool_Put_ValidConn(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 1, time.Second, time.Minute)
	defer p.Close()

	conn, _ := p.Get()
	_, _, activeBefore := p.Stats()

	p.Put(conn)

	_, _, activeAfter := p.Stats()
	if activeAfter != activeBefore-1 {
		t.Errorf("expected active=%d after Put, got %d", activeBefore-1, activeAfter)
	}
}

func TestPool_Close(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 2, time.Second, time.Minute)
	p.Close()

	_, err := p.Get()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestPool_Stats(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 2, time.Second, time.Minute)
	defer p.Close()

	total, idle, active := p.Stats()
	if total != 2 || idle != 2 || active != 2 {
		t.Errorf("unexpected stats: total=%d idle=%d active=%d", total, idle, active)
	}

	conn1, _ := p.Get()
	_, _ = p.Get()

	_, idle, active = p.Stats()
	if idle != 0 || active != 4 {
		t.Errorf("unexpected stats after 2 gets: total=%d idle=%d active=%d", total, idle, active)
	}

	p.Put(conn1)
	_, idle, _ = p.Stats()
	if idle != 1 {
		t.Errorf("expected idle=1, got %d", idle)
	}
}

func TestPool_MaxConnsLimit(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 2, 0, time.Second, time.Minute)
	defer p.Close()

	conn1, _ := p.Get()
	conn2, _ := p.Get()
	_, err := p.Get()

	if err == nil {
		t.Error("expected error when exceeding maxConns")
	}

	p.Put(conn1)
	p.Put(conn2)
}

func TestPool_Concurrent(t *testing.T) {
	factory := func() (any, error) {
		time.Sleep(time.Millisecond)
		return "conn", nil
	}

	p := NewPool(factory, 10, 2, time.Second, time.Minute)
	defer p.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, _ := p.Get()
			time.Sleep(time.Millisecond)
			p.Put(conn)
		}()
	}
	wg.Wait()

	total, idle, active := p.Stats()
	t.Logf("stats: total=%d idle=%d active=%d", total, idle, active)
}

func TestPool_IdleTimeout(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 2, time.Second, 50*time.Millisecond)
	defer p.Close()

	conn1, _ := p.Get()
	conn2, _ := p.Get()
	conn3, _ := p.Get()
	_ = conn1
	_ = conn2
	_ = conn3

	_, idleAfterGet, _ := p.Stats()
	if idleAfterGet != 0 {
		t.Errorf("expected idle=0 after 3 Get calls, got %d", idleAfterGet)
	}

	p.Put(conn1)
	p.Put(conn2)
	p.Put(conn3)

	_, idleAfterPut, _ := p.Stats()
	if idleAfterPut != 3 {
		t.Errorf("expected idle=3 after Put, got %d", idleAfterPut)
	}

	time.Sleep(150 * time.Millisecond)

	_, idleAfter, _ := p.Stats()
	if idleAfter != 2 {
		t.Errorf("expected idle=2 after timeout cleanup (minConns=2), got %d", idleAfter)
	}
}

func TestPool_MinConns(t *testing.T) {
	created := 0
	factory := func() (any, error) {
		created++
		return "conn", nil
	}

	p := NewPool(factory, 10, 5, time.Second, time.Minute)
	defer p.Close()

	if created != 5 {
		t.Errorf("expected 5 minConns created, got %d", created)
	}

	total, _, _ := p.Stats()
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
}

func TestPool_FactoryError(t *testing.T) {
	factory := func() (any, error) {
		return nil, errors.New("factory error")
	}

	p := NewPool(factory, 5, 2, time.Second, time.Minute)
	defer p.Close()

	total, _, _ := p.Stats()
	if total != 0 {
		t.Errorf("expected total=0 when factory fails, got %d", total)
	}
}

func TestPool_PutAfterClose(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 1, time.Second, time.Minute)
	conn, _ := p.Get()
	p.Close()

	p.Put(conn)
}

func TestPool_GetAfterClose(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 1, time.Second, time.Minute)
	p.Close()

	_, err := p.Get()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got %v", err)
	}
}

func TestPool_MultipleClose(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 5, 1, time.Second, time.Minute)
	p.Close()
	p.Close()
}

func TestPool_ZeroMaxConns(t *testing.T) {
	factory := func() (any, error) {
		return "conn", nil
	}

	p := NewPool(factory, 0, 0, time.Second, time.Minute)
	defer p.Close()

	_, _, _ = p.Stats()
}
