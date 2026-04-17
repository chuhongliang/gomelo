package pool

import (
	"testing"
	"time"
)

func TestPoolClose(t *testing.T) {
	factory := func() (any, error) {
		return &testConn{id: time.Now().UnixNano()}, nil
	}

	p := NewPool(factory, 5, 0, 5*time.Second, 300*time.Second)
	time.Sleep(10 * time.Millisecond)

	conns := make([]any, 3)
	for i := 0; i < 3; i++ {
		c, err := p.Get()
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		conns[i] = c
	}

	for _, c := range conns {
		p.Put(c)
	}

	p.Close()
	time.Sleep(10 * time.Millisecond)
}

type testConn struct {
	id int64
}

func (c *testConn) Close() error {
	return nil
}
