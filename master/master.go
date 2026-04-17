package master

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type ServerInfo struct {
	ID         string
	ServerType string
	Host       string
	Port       int
	Frontend   bool
	State      int
	Count      int
	RegisterAt int64
	LastUpdate int64
}

type ServerTypeConfig struct {
	Path      string   `json:"path" yaml:"path"`
	Args      []string `json:"args,omitempty" yaml:"args,omitempty"`
	Env       []string `json:"env,omitempty" yaml:"env,omitempty"`
	Instances int      `json:"instances" yaml:"instances"`
}

type MasterServer interface {
	AddServer(info *ServerInfo) error
	RemoveServer(id string) error
	GetServers() map[string][]*ServerInfo
	GetServersByType(serverType string) []*ServerInfo
	GetServer(id string) (*ServerInfo, bool)
	Start() error
	Stop()
	OnRegister(callback func(*ServerInfo))
	OnUnregister(callback func(string))
	OnStateChange(callback func(id string, oldState, newState int))
}

type masterServer struct {
	addr     string
	listener net.Listener

	servers   map[string]*ServerInfo
	byType    map[string][]*ServerInfo
	serverIDs []string

	onRegister    []func(*ServerInfo)
	onUnregister  []func(string)
	onStateChange []func(id string, oldState, newState int)

	heartbeats map[string]time.Time

	processMgr   ProcessManager
	serverCfgs   map[string]ServerTypeConfig
	autoStart    bool
	restartDelay time.Duration

	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	stats   struct {
		totalRegister   int64
		totalUnregister int64
	}
}

func New(addr string) MasterServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &masterServer{
		addr:         addr,
		servers:      make(map[string]*ServerInfo),
		byType:       make(map[string][]*ServerInfo),
		heartbeats:   make(map[string]time.Time),
		processMgr:   NewProcessManager(),
		serverCfgs:   make(map[string]ServerTypeConfig),
		autoStart:    true,
		restartDelay: 5 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func NewWithConfig(addr string, serverCfgs map[string]ServerTypeConfig, autoStart bool) MasterServer {
	ctx, cancel := context.WithCancel(context.Background())
	m := &masterServer{
		addr:         addr,
		servers:      make(map[string]*ServerInfo),
		byType:       make(map[string][]*ServerInfo),
		heartbeats:   make(map[string]time.Time),
		processMgr:   NewProcessManager(),
		serverCfgs:   serverCfgs,
		autoStart:    autoStart,
		restartDelay: 5 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
	}
	return m
}

func (m *masterServer) Start() error {
	if m.running {
		return nil
	}

	ln, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("master listen failed: %w", err)
	}

	m.listener = ln
	m.running = true

	m.wg.Add(1)
	go m.acceptLoop()

	m.wg.Add(1)
	go m.heartbeatCheck()

	return nil
}

func (m *masterServer) acceptLoop() {
	defer m.wg.Done()

	for {
		conn, err := m.listener.Accept()
		if err != nil {
			select {
			case <-m.ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				if !m.running {
					return
				}
				continue
			}
		}

		m.wg.Add(1)
		go m.handleConn(conn)
	}
}

func (m *masterServer) handleConn(conn net.Conn) {
	defer m.wg.Done()
	defer conn.Close()

	readBuf := make([]byte, 0, 4096)
	for {
		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		readBuf = append(readBuf, buf[:n]...)
		m.processMessages(conn, &readBuf)
	}
}

func (m *masterServer) processMessages(conn net.Conn, buf *[]byte) {
	for len(*buf) >= 4 {
		length := binary.BigEndian.Uint32((*buf)[:4])
		if length > 64*1024 {
			*buf = (*buf)[len(*buf):]
			continue
		}

		if int(length)+4 > len(*buf) {
			return
		}

		data := (*buf)[4 : 4+length]
		*buf = (*buf)[4+length:]

		var msg masterMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "register":
			m.handleRegister(conn, msg.Data)
		case "unregister":
			m.handleUnregister(msg.Data)
		case "heartbeat":
			m.handleHeartbeatConn(conn, msg.Data)
		case "query":
			m.handleQuery(conn)
		}
	}
}

type masterMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type registerData struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Frontend   bool   `json:"frontend"`
	ServerType string `json:"serverType"`
}

func (m *masterServer) handleRegister(conn net.Conn, data json.RawMessage) {
	var reg registerData
	if err := json.Unmarshal(data, &reg); err != nil {
		return
	}

	info := &ServerInfo{
		ID:         reg.ID,
		ServerType: reg.Type,
		Host:       reg.Host,
		Port:       reg.Port,
		State:      1,
		Count:      0,
		RegisterAt: time.Now().Unix(),
		LastUpdate: time.Now().Unix(),
	}

	m.mu.Lock()
	m.servers[info.ID] = info
	m.byType[info.ServerType] = append(m.byType[info.ServerType], info)
	m.serverIDs = append(m.serverIDs, info.ID)
	m.heartbeats[info.ID] = time.Now()
	m.mu.Unlock()

	atomic.AddInt64(&m.stats.totalRegister, 1)

	for _, cb := range m.onRegister {
		go cb(info)
	}

	resp, _ := json.Marshal(map[string]string{"status": "ok"})
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(resp)))
	conn.Write(lenBuf)
	conn.Write(resp)
}

func (m *masterServer) handleUnregister(data json.RawMessage) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}

	m.mu.Lock()
	if info, ok := m.servers[req.ID]; ok {
		delete(m.servers, req.ID)

		if st, ok := m.byType[info.ServerType]; ok {
			for i, s := range st {
				if s.ID == req.ID {
					m.byType[info.ServerType] = append(st[:i], st[i+1:]...)
					break
				}
			}
		}

		delete(m.heartbeats, req.ID)
	}
	m.mu.Unlock()

	atomic.AddInt64(&m.stats.totalUnregister, 1)

	for _, cb := range m.onUnregister {
		go cb(req.ID)
	}
}

func (m *masterServer) handleHeartbeatConn(conn net.Conn, data json.RawMessage) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}

	m.mu.Lock()
	m.heartbeats[req.ID] = time.Now()
	if info, ok := m.servers[req.ID]; ok {
		info.LastUpdate = time.Now().Unix()
	}
	m.mu.Unlock()

	resp, _ := json.Marshal(map[string]string{"status": "ok"})
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(resp)))
	conn.Write(lenBuf)
	conn.Write(resp)
}

func (m *masterServer) handleQuery(conn net.Conn) {
	m.mu.RLock()
	serversCopy := make(map[string]*ServerInfo, len(m.servers))
	for k, v := range m.servers {
		serversCopy[k] = &ServerInfo{
			ID:         v.ID,
			ServerType: v.ServerType,
			Host:       v.Host,
			Port:       v.Port,
			State:      v.State,
			Count:      v.Count,
			RegisterAt: v.RegisterAt,
			LastUpdate: v.LastUpdate,
		}
	}
	count := len(m.servers)
	m.mu.RUnlock()

	result := map[string]any{
		"servers": serversCopy,
		"count":   count,
	}

	data, _ := json.Marshal(result)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	conn.Write(lenBuf)
	conn.Write(data)
}

func (m *masterServer) heartbeatCheck() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkHeartbeats()
		}
	}
}

func (m *masterServer) checkHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	timeout := 30 * time.Second

	for id, last := range m.heartbeats {
		if now.Sub(last) > timeout {
			info, hasInfo := m.servers[id]
			if hasInfo {
				oldState := info.State
				info.State = 3

				for _, cb := range m.onStateChange {
					go cb(id, oldState, info.State)
				}
			}

			delete(m.servers, id)
			delete(m.heartbeats, id)

			if hasInfo {
				if st, stOk := m.byType[info.ServerType]; stOk {
					for i, s := range st {
						if s.ID == id {
							m.byType[info.ServerType] = append(st[:i], st[i+1:]...)
							break
						}
					}
				}
			}

			for _, cb := range m.onUnregister {
				go cb(id)
			}
		}
	}
}

func (m *masterServer) Stop() {
	if !m.running {
		return
	}

	m.running = false
	m.cancel()

	if m.processMgr != nil {
		m.processMgr.Close()
	}

	if m.listener != nil {
		m.listener.Close()
	}

	m.wg.Wait()
}

func (m *masterServer) SetServerCfgs(cfgs map[string]ServerTypeConfig) {
	m.mu.Lock()
	m.serverCfgs = cfgs
	m.mu.Unlock()
}

func (m *masterServer) SetAutoStart(auto bool) {
	m.mu.Lock()
	m.autoStart = auto
	m.mu.Unlock()
}

func (m *masterServer) StartServers(servers []map[string]any) error {
	if !m.autoStart {
		return nil
	}

	m.mu.RLock()
	cfgs := m.serverCfgs
	delay := m.restartDelay
	m.mu.RUnlock()

	eventCh := make(chan ProcessEvent, 10)
	started := make(map[string]bool)

	for _, srv := range servers {
		id, _ := srv["id"].(string)
		serverType, _ := srv["serverType"].(string)
		host, _ := srv["host"].(string)
		port, _ := srv["port"].(float64)

		cfg, ok := cfgs[serverType]
		if !ok || cfg.Path == "" {
			continue
		}

		if _, exists := started[id]; exists {
			continue
		}

		instances := cfg.Instances
		if instances <= 0 {
			instances = 1
		}

		for i := 0; i < instances; i++ {
			instanceID := id
			if instances > 1 {
				instanceID = fmt.Sprintf("%s-%d", id, i)
			}

			env := append(cfg.Env,
				fmt.Sprintf("GOMELO_SERVER_ID=%s", instanceID),
				fmt.Sprintf("GOMELO_SERVER_TYPE=%s", serverType),
				fmt.Sprintf("GOMELO_MASTER_HOST=%s", m.addr),
				fmt.Sprintf("GOMELO_HOST=%s", host),
				fmt.Sprintf("GOMELO_PORT=%d", int(port)),
			)

			args := append([]string{}, cfg.Args...)
			proc, err := m.processMgr.Spawn(instanceID, serverType, cfg.Path, args, env)
			if err != nil {
				continue
			}

			m.processMgr.Watch(proc, eventCh)
			started[instanceID] = true
		}
	}

	go m.watchProcessEvents(eventCh, cfgs, delay)

	return nil
}

func (m *masterServer) watchProcessEvents(ch chan ProcessEvent, cfgs map[string]ServerTypeConfig, delay time.Duration) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-ch:
			if event.Event == "crashed" {
				cfg, ok := cfgs[event.ServerType]
				if !ok {
					continue
				}

				go func() {
					time.Sleep(delay)

					env := append(cfg.Env,
						fmt.Sprintf("GOMELO_SERVER_ID=%s", event.ServerID),
						fmt.Sprintf("GOMELO_SERVER_TYPE=%s", event.ServerType),
						fmt.Sprintf("GOMELO_MASTER_HOST=%s", m.addr),
						fmt.Sprintf("GOMELO_HOST=%s", event.Host),
						fmt.Sprintf("GOMELO_PORT=%d", event.Port),
					)

					args := append([]string{}, cfg.Args...)
					proc, err := m.processMgr.Spawn(event.ServerID, event.ServerType, cfg.Path, args, env)
					if err != nil {
						return
					}

					m.processMgr.Watch(proc, ch)
				}()
			}
		}
	}
}

func (m *masterServer) SpawnServer(id, serverType string, cfg ServerTypeConfig) error {
	if m.processMgr == nil {
		return fmt.Errorf("process manager not initialized")
	}

	env := append(cfg.Env,
		fmt.Sprintf("GOMELO_SERVER_ID=%s", id),
		fmt.Sprintf("GOMELO_SERVER_TYPE=%s", serverType),
		fmt.Sprintf("GOMELO_MASTER_HOST=%s", m.addr),
	)

	args := append([]string{}, cfg.Args...)
	proc, err := m.processMgr.Spawn(id, serverType, cfg.Path, args, env)
	if err != nil {
		return err
	}

	eventCh := make(chan ProcessEvent)
	m.processMgr.Watch(proc, eventCh)

	return nil
}

func (m *masterServer) StopServers() {
	if m.processMgr != nil {
		m.processMgr.Close()
	}
}

func (m *masterServer) GetProcessList() []*ProcessInfo {
	if m.processMgr == nil {
		return nil
	}
	return m.processMgr.List()
}

func (m *masterServer) AddServer(info *ServerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.servers[info.ID] = info
	m.byType[info.ServerType] = append(m.byType[info.ServerType], info)
	m.serverIDs = append(m.serverIDs, info.ID)
	m.heartbeats[info.ID] = time.Now()

	return nil
}

func (m *masterServer) RemoveServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, ok := m.servers[id]; ok {
		delete(m.servers, id)

		if st, ok := m.byType[info.ServerType]; ok {
			for i, s := range st {
				if s.ID == id {
					m.byType[info.ServerType] = append(st[:i], st[i+1:]...)
					break
				}
			}
		}

		delete(m.heartbeats, id)
	}

	return nil
}

func (m *masterServer) GetServers() map[string][]*ServerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*ServerInfo)
	for t, ss := range m.byType {
		result[t] = make([]*ServerInfo, len(ss))
		copy(result[t], ss)
	}

	return result
}

func (m *masterServer) GetServersByType(serverType string) []*ServerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if st, ok := m.byType[serverType]; ok {
		result := make([]*ServerInfo, len(st))
		copy(result, st)
		return result
	}

	return nil
}

func (m *masterServer) GetServer(id string) (*ServerInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, ok := m.servers[id]
	return info, ok
}

func (m *masterServer) OnRegister(callback func(*ServerInfo)) {
	m.mu.Lock()
	m.onRegister = append(m.onRegister, callback)
	m.mu.Unlock()
}

func (m *masterServer) OnUnregister(callback func(string)) {
	m.mu.Lock()
	m.onUnregister = append(m.onUnregister, callback)
	m.mu.Unlock()
}

func (m *masterServer) OnStateChange(callback func(id string, oldState, newState int)) {
	m.mu.Lock()
	m.onStateChange = append(m.onStateChange, callback)
	m.mu.Unlock()
}

type MasterClient interface {
	Register(id, serverType, host string, port int, frontend bool) error
	Unregister() error
	Heartbeat() error
	QueryServers() (map[string][]*ServerInfo, error)
	Close()
}

type masterClient struct {
	id         string
	serverType string
	addr       string
	conn       net.Conn
	mu         sync.Mutex
}

func NewClient(addr, id, serverType string) (MasterClient, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &masterClient{
		id:         id,
		serverType: serverType,
		addr:       addr,
		conn:       conn,
	}, nil
}

func (c *masterClient) Register(id, serverType, host string, port int, frontend bool) error {
	data, _ := json.Marshal(map[string]any{
		"id":         id,
		"type":       serverType,
		"host":       host,
		"port":       port,
		"frontend":   frontend,
		"serverType": serverType,
	})

	msg := masterMessage{
		Type: "register",
		Data: data,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b, _ := json.Marshal(msg)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(b)))
	c.conn.Write(lenBuf)
	c.conn.Write(b)

	header := make([]byte, 4)
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(header)
	resp := make([]byte, length)
	if _, err := io.ReadFull(c.conn, resp); err != nil {
		return err
	}

	var result map[string]string
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if result["status"] != "ok" {
		return fmt.Errorf("register failed")
	}

	return nil
}

func (c *masterClient) Unregister() error {
	data, _ := json.Marshal(map[string]string{"id": c.id})

	msg := masterMessage{
		Type: "unregister",
		Data: data,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b, _ := json.Marshal(msg)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(b)))
	c.conn.Write(lenBuf)
	c.conn.Write(b)

	return nil
}

func (c *masterClient) Heartbeat() error {
	data, _ := json.Marshal(map[string]string{"id": c.id})

	msg := masterMessage{
		Type: "heartbeat",
		Data: data,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b, _ := json.Marshal(msg)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(b)))
	c.conn.Write(lenBuf)
	c.conn.Write(b)

	header := make([]byte, 4)
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(header)
	resp := make([]byte, length)
	if _, err := io.ReadFull(c.conn, resp); err != nil {
		return err
	}

	var result map[string]string
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if result["status"] != "ok" {
		return fmt.Errorf("heartbeat failed")
	}

	return nil
}

func (c *masterClient) QueryServers() (map[string][]*ServerInfo, error) {
	msg := masterMessage{
		Type: "query",
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	b, _ := json.Marshal(msg)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(b)))
	c.conn.Write(lenBuf)
	c.conn.Write(b)

	header := make([]byte, 4)
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	resp := make([]byte, length)
	if _, err := io.ReadFull(c.conn, resp); err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	servers := make(map[string][]*ServerInfo)
	if serversRaw, ok := result["servers"].(map[string]any); ok {
		for stype, val := range serversRaw {
			if arr, ok := val.([]any); ok {
				var list []*ServerInfo
				for _, item := range arr {
					if m, ok := item.(map[string]any); ok {
						si := &ServerInfo{}
						if id, ok := m["id"].(string); ok {
							si.ID = id
						}
						if t, ok := m["type"].(string); ok {
							si.ServerType = t
						}
						if h, ok := m["host"].(string); ok {
							si.Host = h
						}
						if p, ok := m["port"].(float64); ok {
							si.Port = int(p)
						}
						list = append(list, si)
					}
				}
				servers[stype] = list
			}
		}
	}

	return servers, nil
}

func (c *masterClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
