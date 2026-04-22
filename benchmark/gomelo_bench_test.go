package benchmark

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chuhongliang/gomelo/pool"
)

func encodeMessage(msgType int, route string, seq uint64, body interface{}) []byte {
	bodyBytes, _ := json.Marshal(body)
	routeBytes := []byte(route)

	headerLen := 1 + 1 + len(routeBytes) + 1 + 8
	totalLen := headerLen + len(bodyBytes)

	buf := make([]byte, 4+totalLen)
	binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))

	offset := 4
	buf[offset] = byte(msgType)
	offset++

	buf[offset] = byte(len(routeBytes))
	offset++
	copy(buf[offset:offset+len(routeBytes)], routeBytes)
	offset += len(routeBytes)

	buf[offset] = 0
	offset++

	binary.BigEndian.PutUint64(buf[offset:offset+8], seq)
	offset += 8

	copy(buf[offset:offset+len(bodyBytes)], bodyBytes)

	return buf
}

func BenchmarkEncodeMessage(b *testing.B) {
	b.StopTimer()
	route := "connector.entry"
	body := map[string]interface{}{"name": "Player1"}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_ = encodeMessage(0, route, uint64(i), body)
	}
}

func BenchmarkDecodeMessageHeader(b *testing.B) {
	b.StopTimer()
	data := encodeMessage(0, "connector.entry", 12345, map[string]interface{}{"name": "Player1"})
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		if len(data) < 5 {
			continue
		}
		_ = data[4]
		_ = binary.BigEndian.Uint64(data[14:22])
	}
	b.ReportAllocs()
}

func BenchmarkJSONMarshal(b *testing.B) {
	b.StopTimer()
	data := map[string]interface{}{"name": "Player1", "x": 100, "y": 200}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	b.StopTimer()
	bytes, _ := json.Marshal(map[string]interface{}{"name": "Player1", "x": 100, "y": 200})
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		var v interface{}
		_ = json.Unmarshal(bytes, &v)
	}
	b.ReportAllocs()
}

func BenchmarkPoolGetPut(b *testing.B) {
	b.StopTimer()
	p := pool.NewPool(func() (any, error) {
		return make([]byte, 1024), nil
	}, 100, 10, time.Second, time.Minute)
	defer p.Close()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		obj, _ := p.Get()
		p.Put(obj)
	}
}

func BenchmarkPoolParallel(b *testing.B) {
	b.StopTimer()
	p := pool.NewPool(func() (any, error) {
		return &bytes.Buffer{}, nil
	}, 100, 10, time.Second, time.Minute)
	defer p.Close()
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj, _ := p.Get()
			p.Put(obj)
		}
	})
}

func BenchmarkWorkerPool(b *testing.B) {
	b.StopTimer()
	wp := pool.NewWorkerPool(10, 1000)
	defer wp.Close()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		wp.Submit(func() {})
	}
}

func BenchmarkConcurrentMap(b *testing.B) {
	b.StopTimer()
	var m sync.Map
	m.Store("key1", 1)
	m.Store("key2", 2)
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Load("key1")
			m.Store("key3", 3)
		}
	})
}

func BenchmarkAtomicCounter(b *testing.B) {
	b.StopTimer()
	var counter atomic.Int64
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
			counter.Load()
		}
	})
}

func BenchmarkChannel(b *testing.B) {
	b.StopTimer()
	ch := make(chan int, 100)
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			select {
			case ch <- 1:
			default:
			}
			select {
			case <-ch:
			default:
			}
		}
	})
}

func BenchmarkHTTPServer(b *testing.B) {
	b.StopTimer()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	b.StartTimer()

	client := &http.Client{Timeout: time.Second}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, _ := client.Get(server.URL)
			if resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}
	})
}

func BenchmarkWebSocketMessage(b *testing.B) {
	b.StopTimer()
	route := "connector.entry"
	body := map[string]interface{}{"name": "Player1", "x": 100, "y": 200}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		msg := encodeMessage(0, route, uint64(i), body)
		if len(msg) < 4 {
			continue
		}
		_ = binary.BigEndian.Uint32(msg[0:4])
	}
}

type SimpleBuffer struct {
	data []byte
}

func BenchmarkBufferAllocation(b *testing.B) {
	b.StopTimer()
	pool := sync.Pool{
		New: func() interface{} {
			return &SimpleBuffer{data: make([]byte, 1024)}
		},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get().(*SimpleBuffer)
		buf.data = buf.data[:1024]
		pool.Put(buf)
	}
	b.ReportAllocs()
}