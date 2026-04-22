package benchmark

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chuhongliang/gomelo/lib"
	"github.com/chuhongliang/gomelo/pool"
)

type Message struct {
	Type  int
	Route string
	Seq   uint64
	Body  interface{}
}

func (m *Message) Encode() []byte {
	bodyBytes, _ := json.Marshal(m.Body)
	routeBytes := []byte(m.Route)

	headerLen := 1 + 1 + len(routeBytes) + 1 + 8
	totalLen := headerLen + len(bodyBytes)

	buf := make([]byte, 4+totalLen)
	binary.BigEndian.PutUint32(buf[0:4], uint32(totalLen))

	offset := 4
	buf[offset] = byte(m.Type)
	offset++

	buf[offset] = byte(len(routeBytes))
	offset++
	copy(buf[offset:offset+len(routeBytes)], routeBytes)
	offset += len(routeBytes)

	buf[offset] = 0
	offset++

	binary.BigEndian.PutUint64(buf[offset:offset+8], m.Seq)
	offset += 8

	copy(buf[offset:offset+len(bodyBytes)], bodyBytes)

	return buf
}

func (m *Message) Decode(data []byte) error {
	if len(data) < 5 {
		return fmt.Errorf("message too short")
	}

	length := binary.BigEndian.Uint32(data[0:4])
	if int(length)+4 > len(data) || length == 0 {
		return fmt.Errorf("invalid message length")
	}

	m.Type = int(data[4])
	offset := 5

	if offset >= len(data) {
		return fmt.Errorf("invalid header")
	}

	routeLen := int(data[offset])
	offset++
	if offset+routeLen > len(data) {
		return fmt.Errorf("invalid route")
	}
	m.Route = string(data[offset : offset+routeLen])
	offset += routeLen + 1

	if offset+8 > len(data) {
		return fmt.Errorf("invalid seq")
	}
	m.Seq = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	if offset < len(data) {
		json.Unmarshal(data[offset:], &m.Body)
	}

	return nil
}

func BenchmarkMessageEncode(b *testing.B) {
	b.StopTimer()
	msg := &Message{
		Type:  0,
		Route: "connector.entry",
		Seq:   12345,
		Body:  map[string]interface{}{"name": "Player1"},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_ = msg.Encode()
	}
}

func BenchmarkMessageDecode(b *testing.B) {
	b.StopTimer()
	msg := &Message{
		Type:  0,
		Route: "connector.entry",
		Seq:   12345,
		Body:  map[string]interface{}{"name": "Player1"},
	}
	data := msg.Encode()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		m := &Message{}
		m.Decode(data)
	}
	b.ReportAllocs()
}

func BenchmarkMessageEncodeDecode(b *testing.B) {
	b.StopTimer()
	msg := &Message{
		Type:  0,
		Route: "connector.entry",
		Seq:   12345,
		Body:  map[string]interface{}{"name": "Player1", "x": 100, "y": 200},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		data := msg.Encode()
		m := &Message{}
		m.Decode(data)
	}
	b.ReportAllocs()
}

func BenchmarkPoolAllocation(b *testing.B) {
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
	b.ReportAllocs()
}

func BenchmarkPoolNoAlloc(b *testing.B) {
	b.StopTimer()
	p := pool.NewPool(func() (any, error) {
		return make([]byte, 1024), nil
	}, 100, 10, time.Second, time.Minute)
	defer p.Close()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		obj, _ := p.Get()
		_ = obj.([]byte)
		p.Put(obj)
	}
}

func BenchmarkWorkerPoolThroughput(b *testing.B) {
	b.StopTimer()
	wp := pool.NewWorkerPool(10, 10000)
	defer wp.Close()

	var count atomic.Int64
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wp.Submit(func() {
				count.Add(1)
			})
		}
	})

	b.ReportMetric(float64(count.Load()), "ops")
}

func BenchmarkSessionCreation(b *testing.B) {
	b.StopTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		session := lib.NewSession()
		_ = session
	}
	b.ReportAllocs()
}

func BenchmarkRouteMatching(b *testing.B) {
	b.StopTimer()
	routes := []string{
		"connector.entry",
		"player.move",
		"chat.send",
		"game.start",
		"battle.attack",
	}
	routeMap := make(map[string]int)
	for i, r := range routes {
		routeMap[r] = i
	}
	target := "player.move"
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, ok := routeMap[target]
		_ = ok
	}
}

func BenchmarkJSONSerializeMap(b *testing.B) {
	b.StopTimer()
	data := map[string]interface{}{
		"name":   "Player1",
		"level":  99,
		"hp":     1000,
		"mp":     500,
		"x":      100.5,
		"y":      200.5,
		"items":  []interface{}{1, 2, 3, 4, 5},
		"skills": []interface{}{"fire", "ice", "lightning"},
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

func BenchmarkJSONDeserializeMap(b *testing.B) {
	b.StopTimer()
	bytes, _ := json.Marshal(map[string]interface{}{
		"name":   "Player1",
		"level":  99,
		"hp":     1000,
		"items":  []interface{}{1, 2, 3, 4, 5},
	})
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		var v map[string]interface{}
		_ = json.Unmarshal(bytes, &v)
	}
	b.ReportAllocs()
}

func BenchmarkBytesBufferWrite(b *testing.B) {
	b.StopTimer()
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	data := []byte("hello world message for testing")
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.Write(data)
	}
	b.ReportAllocs()
}

func BenchmarkStringConcat(b *testing.B) {
	b.StopTimer()
	parts := []string{"player", "move", "to", "100", "200"}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_ = strings.Join(parts, ".")
	}
}

func BenchmarkChannelNonBlocking(b *testing.B) {
	b.StopTimer()
	ch := make(chan struct{}, 100)
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			select {
			case ch <- struct{}{}:
			default:
			}
			select {
			case <-ch:
			default:
			}
		}
	})
}

func BenchmarkChannelBlocking(b *testing.B) {
	b.StopTimer()
	ch := make(chan struct{}, 100)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(100)

	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case ch <- struct{}{}:
				case <-done:
					return
				}
			}
		}()
	}
	b.StartTimer()

	time.Sleep(time.Second)
	close(done)
	wg.Wait()
}

type MockSession struct {
	id     uint64
	route  string
	data   map[string]interface{}
}

func (s *MockSession) ID() uint64 {
	return s.id
}

func BenchmarkSessionMap(b *testing.B) {
	b.StopTimer()
	sessions := make(map[uint64]*MockSession)
	for i := uint64(0); i < 10000; i++ {
		sessions[i] = &MockSession{id: i}
	}
	b.StartTimer()

	id := uint64(5000)
	for i := 0; i < b.N; i++ {
		s := sessions[id]
		_ = s
	}
}

func BenchmarkHTTPHandler(b *testing.B) {
	b.StopTimer()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0}`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	b.StartTimer()

	client := &http.Client{Timeout: time.Second}
	for i := 0; i < b.N; i++ {
		resp, _ := client.Post(server.URL, "application/json", strings.NewReader(`{"name":"test"}`))
		if resp != nil {
			resp.Body.Close()
		}
	}
}