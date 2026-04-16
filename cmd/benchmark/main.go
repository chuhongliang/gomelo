package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var (
	host     = flag.String("host", "localhost:3010", "Server address")
	users    = flag.Int("users", 100, "Number of concurrent users")
	duration = flag.Int("duration", 10, "Test duration in seconds")
)

type Benchmark struct {
	conns    int32
	sends    int32
	recvs    int32
	errors   int32
	latency  int64
	latencyN int32
}

type Message struct {
	Type  int            `json:"type"`
	Route string         `json:"route"`
	Seq   uint64         `json:"seq"`
	Body  map[string]any `json:"body"`
}

func main() {
	flag.Parse()

	fmt.Printf("=== Pomelo Benchmark ===\n")
	fmt.Printf("Target: %s\n", *host)
	fmt.Printf("Users: %d\n", *users)
	fmt.Printf("Duration: %d seconds\n\n", *duration)

	b := &Benchmark{}

	start := time.Now()
	endTime := start.Add(time.Duration(*duration) * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < *users; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			b.runClient(id, endTime)
		}(i)
	}

	wg.Wait()

	b.print()
}

func (b *Benchmark) runClient(id int, endTime time.Time) {
	conn, err := net.Dial("tcp", *host)
	if err != nil {
		atomic.AddInt32(&b.errors, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt32(&b.conns, 1)

	buf := make([]byte, 4096)
	seq := uint64(0)
	for time.Now().Before(endTime) {
		seq++
		req := Message{
			Type:  0,
			Route: "connector.entry",
			Seq:   seq,
			Body:  map[string]any{"token": fmt.Sprintf("test%d", id)},
		}

		data, _ := json.Marshal(req)
		header := make([]byte, 4)
		binary.BigEndian.PutUint32(header, uint32(len(data)))

		ts := time.Now()
		_, err = conn.Write(append(header, data...))
		if err != nil {
			atomic.AddInt32(&b.errors, 1)
			return
		}
		atomic.AddInt32(&b.sends, 1)

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, err = conn.Read(buf)
		if err != nil {
			atomic.AddInt32(&b.errors, 1)
			return
		}
		atomic.AddInt32(&b.recvs, 1)

		lat := time.Since(ts).Microseconds()
		atomic.AddInt64(&b.latency, lat)
		atomic.AddInt32(&b.latencyN, 1)
	}
}

func (b *Benchmark) print() {
	conns := atomic.LoadInt32(&b.conns)
	sends := atomic.LoadInt32(&b.sends)
	errors := atomic.LoadInt32(&b.errors)
	latency := atomic.LoadInt64(&b.latency)
	latencyN := atomic.LoadInt32(&b.latencyN)

	runtime := runtime.NumCPU()

	qps := float64(sends) / float64(*duration)
	if latencyN > 0 {
		avgLat := float64(latency) / float64(latencyN) / 1000
		fmt.Printf("Results:\n")
		fmt.Printf("  Connections: %d\n", conns)
		fmt.Printf("  Requests:   %d\n", sends)
		fmt.Printf("  QPS:       %.2f\n", qps)
		fmt.Printf("  Avg Latency: %.2f ms\n", avgLat)
		fmt.Printf("  Errors:    %d\n", errors)
		fmt.Printf("  CPUs:      %d\n", runtime)
	}
}
