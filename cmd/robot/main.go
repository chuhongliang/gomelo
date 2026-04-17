package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gomelo/protocol"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	host     = flag.String("host", "localhost:3010", "Server address")
	users    = flag.Int("users", 100, "Concurrent users")
	duration = flag.Int("duration", 60, "Test duration in seconds")
	interval = flag.Int("interval", 100, "Request interval in ms")
	route    = flag.String("route", "connector.entry", "Route to test")
)

type Robot struct {
	id     int
	conn   net.Conn
	sendOK int32
	recvOK int32
	err    int32
}

type Message struct {
	Type  int            `json:"type"`
	Route string         `json:"route"`
	Seq   uint64         `json:"seq"`
	Body  map[string]any `json:"body"`
}

func main() {
	flag.Parse()

	fmt.Printf("=== Gomelo Robot ===\n")
	fmt.Printf("Target: %s\n", *host)
	fmt.Printf("Users: %d\n", *users)
	fmt.Printf("Duration: %d seconds\n", *duration)
	fmt.Printf("Route: %s\n\n", *route)

	start := time.Now()
	endTime := start.Add(time.Duration(*duration) * time.Second)

	totalSend := int32(0)
	totalRecv := int32(0)
	totalErr := int32(0)

	var wg sync.WaitGroup
	for i := 0; i < *users; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r := newRobot(id, endTime)
			atomic.AddInt32(&totalSend, r.sendOK)
			atomic.AddInt32(&totalRecv, r.recvOK)
			atomic.AddInt32(&totalErr, r.err)
		}(i)
	}

	wg.Wait()

	elapsed := time.Since(start).Seconds()

	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Total Requests: %d\n", totalSend)
	fmt.Printf("Total Responses: %d\n", totalRecv)
	fmt.Printf("Errors: %d\n", totalErr)
	fmt.Printf("QPS: %.2f\n", float64(totalSend)/elapsed)
	if totalSend > 0 {
		fmt.Printf("Success Rate: %.2f%%\n", float64(totalRecv)/float64(totalSend)*100)
	}
}

func newRobot(id int, endTime time.Time) *Robot {
	r := &Robot{id: id}

	conn, err := net.Dial("tcp", *host)
	if err != nil {
		atomic.AddInt32(&r.err, 1)
		return r
	}
	r.conn = conn

	buf := make([]byte, 4096)
	ticker := time.NewTicker(time.Duration(*interval) * time.Millisecond)
	seq := uint64(0)

	for {
		select {
		case <-ticker.C:
			if time.Now().After(endTime) {
				return r
			}

			seq++
			msg := protocol.NewRequest(*route, seq, map[string]any{"uid": fmt.Sprintf("robot%d", id), "token": "test"})

			data, _ := json.Marshal(msg)
			_, err := conn.Write(protocol.EncodeFrame(data))
			if err != nil {
				atomic.AddInt32(&r.err, 1)
				return r
			}
			atomic.AddInt32(&r.sendOK, 1)

			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, err = conn.Read(buf)
			if err != nil {
				atomic.AddInt32(&r.err, 1)
			} else {
				atomic.AddInt32(&r.recvOK, 1)
			}
		case <-time.After(time.Second):
			if time.Now().After(endTime) {
				return r
			}
		}
	}
}

type Scenario struct {
	name    string
	actions []Action
}

type Action struct {
	Route string
	Delay int
	Body  map[string]any
}

func newScenario(name string) *Scenario {
	return &Scenario{name: name, actions: make([]Action, 0)}
}

func (s *Scenario) Add(route string, delay int, body map[string]any) {
	s.actions = append(s.actions, Action{Route: route, Delay: delay, Body: body})
}

func (s *Scenario) Run(r *Robot) {
	for _, a := range s.actions {
		time.Sleep(time.Duration(a.Delay) * time.Millisecond)
		r.doAction(a.Route, a.Body)
	}
}

func (r *Robot) doAction(route string, body map[string]any) {
	msg := protocol.NewPush(route, body)
	data, _ := json.Marshal(msg)
	r.conn.Write(protocol.EncodeFrame(data))
}

type ScenarioSuite struct {
	scenarios []*Scenario
}

func (s *ScenarioSuite) Add(scenario *Scenario) {
	s.scenarios = append(s.scenarios, scenario)
}

func (s *ScenarioSuite) RunAll(userCount int) {
	var wg sync.WaitGroup
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			si := s.scenarios[idx%len(s.scenarios)]
			r := newRobot(idx, time.Now().Add(time.Duration(*duration)*time.Second))
			si.Run(r)
		}(i)
	}
	wg.Wait()
}

func CreateChatScenario() *Scenario {
	s := newScenario("chat")
	s.Add("connector.entry", 0, map[string]any{"token": "test"})
	s.Add("chat.chatSend", 100, map[string]any{"content": randomMsg()})
	s.Add("chat.chatSend", 500, map[string]any{"content": randomMsg()})
	return s
}

func CreateBattleScenario() *Scenario {
	s := newScenario("battle")
	s.Add("connector.entry", 0, map[string]any{"token": "test"})
	s.Add("battle.join", 100, map[string]any{"roomId": rand.Intn(100)})
	s.Add("battle.action", 200, map[string]any{"action": "attack"})
	return s
}

func randomMsg() string {
	msgs := []string{
		"Hello everyone!",
		"Nice game!",
		"GL HF!",
		"gg wp",
	}
	return msgs[rand.Intn(len(msgs))]
}
