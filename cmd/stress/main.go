package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/chuhongliang/gomelo/protocol"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	host        = flag.String("host", "127.0.0.1", "Server host")
	port        = flag.Int("port", 3010, "Server port")
	conns       = flag.Int("conns", 100, "Number of connections")
	msgs        = flag.Int("msgs", 1000, "Messages per connection")
	duration    = flag.Int("duration", 60, "Test duration in seconds")
	concurrency = flag.Int("concurrency", 10, "Concurrent senders")
)

type Stats struct {
	success   int64
	failed    int64
	latencies []int64
	mu        sync.Mutex
}

func (s *Stats) record(latency int64, ok bool) {
	if ok {
		atomic.AddInt64(&s.success, 1)
	} else {
		atomic.AddInt64(&s.failed, 1)
	}
	s.mu.Lock()
	s.latencies = append(s.latencies, latency)
	s.mu.Unlock()
}

func (s *Stats) report() {
	success := atomic.LoadInt64(&s.success)
	failed := atomic.LoadInt64(&s.failed)
	total := success + failed

	fmt.Printf("\n=== Stress Test Results ===\n")
	fmt.Printf("Total Requests: %d\n", total)
	fmt.Printf("Success: %d\n", success)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.2f%%\n", float64(success)/float64(total)*100)

	if len(s.latencies) > 0 {
		var sum, min, max int64
		p99 := int64(0)
		p95 := int64(0)

		for _, l := range s.latencies {
			sum += l
			if min == 0 || l < min {
				min = l
			}
			if l > max {
				max = l
			}
		}
		avg := sum / int64(len(s.latencies))

		sorted := make([]int64, len(s.latencies))
		copy(sorted, s.latencies)
		for i := 0; i < len(sorted)/2; i++ {
			j := len(sorted) - 1 - i
			sorted[i], sorted[j] = sorted[j], sorted[i]
		}
		p99Index := int(float64(len(sorted)) * 0.99)
		p95Index := int(float64(len(sorted)) * 0.95)
		if p99Index < len(sorted) {
			p99 = sorted[p99Index]
		}
		if p95Index < len(sorted) {
			p95 = sorted[p95Index]
		}

		fmt.Printf("\n=== Latency Stats (microseconds) ===\n")
		fmt.Printf("Avg: %d µs\n", avg)
		fmt.Printf("Min: %d µs\n", min)
		fmt.Printf("Max: %d µs\n", max)
		fmt.Printf("P95: %d µs\n", p95)
		fmt.Printf("P99: %d µs\n", p99)
	}
}

type ConnectorClient struct {
	conn net.Conn
	id   uint64
	seq  uint64
	mu   sync.Mutex
}

func NewConnectorClient(addr string) (*ConnectorClient, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return &ConnectorClient{conn: conn}, nil
}

func (c *ConnectorClient) Send(route string, data []byte) error {
	c.mu.Lock()
	seq := c.seq
	c.seq++
	c.mu.Unlock()

	var body map[string]any
	json.Unmarshal(data, &body)

	msg := protocol.NewRequest(route, seq, body)
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(protocol.EncodeFrame(msgBytes))
	return err
}

func (c *ConnectorClient) Close() error {
	return c.conn.Close()
}

func stressTest() {
	addr := fmt.Sprintf("%s:%d", *host, *port)
	stats := &Stats{}

	var wg sync.WaitGroup
	connCh := make(chan *ConnectorClient, *conns)

	fmt.Printf("Connecting to %s...\n", addr)
	for i := 0; i < *conns; i++ {
		client, err := NewConnectorClient(addr)
		if err != nil {
			fmt.Printf("Failed to connect %d: %v\n", i+1, err)
			continue
		}
		connCh <- client
		if (i+1)%10 == 0 {
			fmt.Printf("Connected %d/%d\n", i+1, *conns)
		}
	}

	connected := len(connCh)
	fmt.Printf("Connected %d clients\n", connected)

	if connected == 0 {
		fmt.Println("No connections established, exiting")
		return
	}

	fmt.Printf("Starting stress test: %d msgs per client, %d concurrency...\n", *msgs, *concurrency)

	start := time.Now()
	timeout := time.After(time.Duration(*duration) * time.Second)
	done := make(chan struct{})

	go func() {
		for i := 0; i < *concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				msgCount := *msgs / *concurrency
				for j := 0; j < msgCount; j++ {
					client := <-connCh
					if client == nil {
						continue
					}

					msgStart := time.Now()
					err := client.Send("connector.entry", []byte(`{"name":"stress"}`))
					latency := time.Since(msgStart).Microseconds()

					if err != nil {
						stats.record(latency, false)
					} else {
						stats.record(latency, true)
					}

					connCh <- client

					if (j+1)%100 == 0 {
						fmt.Printf("Worker %d: %d/%d messages\n", workerID, j+1, msgCount)
					}
				}
			}(i)
		}
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Printf("\nTest completed in %v\n", time.Since(start))
	case <-timeout:
		fmt.Printf("\nTest timed out after %v\n", time.Since(start))
	}

	stats.report()
}

func rpcStressTest() {
	addr := fmt.Sprintf("%s:%d", *host, *port)
	stats := &Stats{}

	fmt.Printf("RPC stress test to %s\n", addr)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	var wg sync.WaitGroup
	seq := uint64(0)
	seqMu := sync.Mutex{}

	start := time.Now()
	timeout := time.After(time.Duration(*duration) * time.Second)

	go func() {
		buf := make([]byte, 4096)
		for {
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
			if n > 4 {
				_ = buf[4:n]
			}
		}
	}()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-timeout:
					return
				default:
				}

				seqMu.Lock()
				s := seq
				seq++
				seqMu.Unlock()

				req := protocol.NewRequest("test.ping", s, map[string]any{})

				msgStart := time.Now()
				msgBytes, _ := json.Marshal(req)
				_, err := conn.Write(protocol.EncodeFrame(msgBytes))
				latency := time.Since(msgStart).Microseconds()

				if err != nil {
					stats.record(latency, false)
				} else {
					stats.record(latency, true)
				}

				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	fmt.Printf("\nRPC test completed in %v\n", time.Since(start))
	stats.report()
}

func benchmarkTest() {
	addr := fmt.Sprintf("%s:%d", *host, *port)

	fmt.Printf("Running benchmark test...\n")

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	start := time.Now()
	count := int64(0)

	go func() {
		buf := make([]byte, 4096)
		for {
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, err := conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
			atomic.AddInt64(&count, 1)
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start).Seconds()
			total := atomic.LoadInt64(&count)
			fmt.Printf("Time: %.1fs, Throughput: %.2f msg/s\n", elapsed, float64(total)/elapsed)
		case <-time.After(time.Duration(*duration) * time.Second):
			elapsed := time.Since(start).Seconds()
			total := atomic.LoadInt64(&count)
			fmt.Printf("\nFinal: %.1fs, Total: %d, Avg: %.2f msg/s\n", elapsed, total, float64(total)/elapsed)
			return
		}
	}
}

func main() {
	flag.Parse()

	fmt.Println("=== Gomelo Stress Test Tool ===")
	fmt.Println("1. Connector stress test")
	fmt.Println("2. RPC stress test")
	fmt.Println("3. Benchmark test")
	fmt.Println()

	testType := 1
	fmt.Scanf("%d", &testType)

	switch testType {
	case 1:
		stressTest()
	case 2:
		rpcStressTest()
	case 3:
		benchmarkTest()
	default:
		stressTest()
	}
}
