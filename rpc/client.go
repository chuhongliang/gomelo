package rpc

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

	"gomelo/server_registry"
)

type RPCClient interface {
	Close()
	Invoke(service, method string, args, reply any) error
	InvokeCtx(ctx context.Context, service, method string, args, reply any) error
	Notify(service, method string, args any) error
}

type ClientOptions struct {
	Host      string
	Port      int
	MaxConns  int
	MinConns  int
	KeepAlive time.Duration
	IdleTime  time.Duration
	Timeout   time.Duration
}

type rpcRequest struct {
	Seq     uint64
	Type    string
	Service string
	Method  string
	Args    any
}

type rpcResponse struct {
	Seq   uint64
	Error string
	Reply any
}

type rpcFuture struct {
	reply any
	err   error
	done  chan struct{}
}

type ClientPool interface {
	GetClient() (RPCClient, error)
	Close()
	Addr() string
}

type poolClient struct {
	addr       string
	opts       *ClientOptions
	conns      []net.Conn
	seq        uint64
	mu         sync.RWMutex
	cond       *sync.Cond
	closed     bool
	minConns   int
	maxConns   int
	totalConns atomic.Int64
	inFlight   sync.WaitGroup
}

func newPoolClient(addr string, opts *ClientOptions) *poolClient {
	if opts == nil {
		opts = &ClientOptions{
			MaxConns:  10,
			MinConns:  1,
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
			IdleTime:  300 * time.Second,
		}
	}

	p := &poolClient{
		addr:     addr,
		opts:     opts,
		conns:    make([]net.Conn, 0, opts.MaxConns),
		minConns: opts.MinConns,
		maxConns: opts.MaxConns,
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *poolClient) Addr() string {
	return p.addr
}

func (p *poolClient) GetClient() (RPCClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("pool closed")
	}

	for {
		if len(p.conns) > 0 {
			conn := p.conns[len(p.conns)-1]
			p.conns = p.conns[:len(p.conns)-1]
			p.totalConns.Add(1)
			return &clientConn{pool: p, conn: conn, seq: &p.seq}, nil
		}

		if p.maxConns > 0 && p.totalConns.Load() >= int64(p.maxConns) {
			waitTimeout := 30 * time.Second
			deadline := time.Now().Add(waitTimeout)
			for len(p.conns) == 0 && time.Now().Before(deadline) {
				p.cond.Wait()
				if p.closed {
					return nil, fmt.Errorf("pool closed")
				}
			}
			if p.closed {
				return nil, fmt.Errorf("pool closed")
			}
			if len(p.conns) > 0 {
				conn := p.conns[len(p.conns)-1]
				p.conns = p.conns[:len(p.conns)-1]
				p.totalConns.Add(1)
				return &clientConn{pool: p, conn: conn, seq: &p.seq}, nil
			}
			return nil, fmt.Errorf("pool timeout: no available connections after %v", waitTimeout)
		}

		break
	}

	conn, err := net.DialTimeout("tcp", p.addr, p.opts.Timeout)
	if err != nil {
		return nil, err
	}
	p.totalConns.Add(1)
	return &clientConn{pool: p, conn: conn, seq: &p.seq}, nil
}

func (p *poolClient) returnClient(conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalConns.Add(-1)
	conn.Close()
	if p.closed {
		return
	}

	if len(p.conns) < p.maxConns {
		p.conns = append(p.conns, conn)
		p.cond.Signal()
	}
}

func (p *poolClient) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = nil
	p.mu.Unlock()

	p.inFlight.Wait()
}

type clientConn struct {
	pool *poolClient
	conn net.Conn
	mu   sync.Mutex
	seq  *uint64
}

func (c *clientConn) Invoke(service, method string, args, reply any) error {
	return c.InvokeCtx(context.Background(), service, method, args, reply)
}

func (c *clientConn) InvokeCtx(ctx context.Context, service, method string, args, reply any) error {
	c.pool.inFlight.Add(1)
	defer c.pool.inFlight.Done()

	seq := atomic.AddUint64(c.seq, 1)

	req := &rpcRequest{
		Seq:     seq,
		Type:    "invoke",
		Service: service,
		Method:  method,
		Args:    args,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(c.pool.opts.Timeout))

	_, err = c.conn.Write(append(header, data...))
	if err != nil {
		return err
	}

	respData := make([]byte, 4)
	c.conn.SetReadDeadline(time.Now().Add(c.pool.opts.Timeout))
	_, err = c.conn.Read(respData)
	if err != nil {
		return err
	}

	length := binary.BigEndian.Uint32(respData)
	if length > 1024*1024 {
		return fmt.Errorf("response too large: %d", length)
	}

	body := make([]byte, length)
	_, err = io.ReadFull(c.conn, body)
	if err != nil {
		return err
	}

	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}

	if reply != nil && resp.Reply != nil {
		replyData, err := json.Marshal(resp.Reply)
		if err != nil {
			return fmt.Errorf("marshal reply: %w", err)
		}
		if err := json.Unmarshal(replyData, reply); err != nil {
			return fmt.Errorf("unmarshal reply: %w", err)
		}
	}

	return nil
}

func (c *clientConn) Notify(service, method string, args any) error {
	c.pool.inFlight.Add(1)
	defer c.pool.inFlight.Done()

	seq := atomic.AddUint64(c.seq, 1)

	req := &rpcRequest{
		Seq:     seq,
		Type:    "notify",
		Service: service,
		Method:  method,
		Args:    args,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(c.pool.opts.Timeout))

	_, err = c.conn.Write(append(header, data...))
	return err
}

func (c *clientConn) Close() {
	c.pool.returnClient(c.conn)
}

type Selector interface {
	Select(serverType string) server_registry.ServerInfo
	SelectMultiple(serverType string, count int) []server_registry.ServerInfo
}

type RPCClientManager interface {
	GetClient(serverType string) (RPCClient, error)
	Close()
}

type rpcClientManager struct {
	pools    map[string]ClientPool
	registry server_registry.ServerRegistry
	selector Selector
	opts     *ClientOptions
	mu       sync.RWMutex
	closed   bool
}

func NewClientManager(registry server_registry.ServerRegistry, selector Selector, opts *ClientOptions) (RPCClientManager, error) {
	if opts == nil {
		opts = &ClientOptions{
			MaxConns:  10,
			MinConns:  1,
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
			IdleTime:  300 * time.Second,
		}
	}

	return &rpcClientManager{
		pools:    make(map[string]ClientPool),
		registry: registry,
		selector: selector,
		opts:     opts,
	}, nil
}

func (m *rpcClientManager) GetClient(serverType string) (RPCClient, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("client manager closed")
	}

	pool, ok := m.pools[serverType]
	m.mu.RUnlock()

	if ok {
		return pool.GetClient()
	}

	server := m.selector.Select(serverType)
	if server.ID == "" {
		return nil, fmt.Errorf("no server found for type: %s", serverType)
	}

	addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", server.Port))

	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, ok := m.pools[serverType]; ok {
		return pool.GetClient()
	}

	newPool := newPoolClient(addr, m.opts)
	m.pools[serverType] = newPool

	return newPool.GetClient()
}

func (m *rpcClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	for _, pool := range m.pools {
		pool.Close()
	}
	m.pools = nil
}

func NewClient(opts *ClientOptions) (RPCClient, error) {
	if opts == nil {
		opts = &ClientOptions{
			MaxConns:  10,
			MinConns:  1,
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
			IdleTime:  300 * time.Second,
		}
	}

	addr := net.JoinHostPort(opts.Host, fmt.Sprintf("%d", opts.Port))
	conn, err := net.DialTimeout("tcp", addr, opts.Timeout)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &singleClient{
		addr:    addr,
		opts:    opts,
		conn:    conn,
		pending: make(map[uint64]*rpcFuture),
		ctx:     ctx,
		cancel:  cancel,
	}

	go c.receiveLoop()
	return c, nil
}

type singleClient struct {
	addr    string
	opts    *ClientOptions
	conn    net.Conn
	seq     uint64
	pending map[uint64]*rpcFuture
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
}

func (c *singleClient) Invoke(service, method string, args, reply any) error {
	return c.InvokeCtx(context.Background(), service, method, args, reply)
}

func (c *singleClient) InvokeCtx(ctx context.Context, service, method string, args, reply any) error {
	seq := atomic.AddUint64(&c.seq, 1)
	future := &rpcFuture{reply: reply, done: make(chan struct{})}

	c.mu.Lock()
	c.pending[seq] = future
	c.mu.Unlock()

	req := &rpcRequest{
		Seq:     seq,
		Type:    "invoke",
		Service: service,
		Method:  method,
		Args:    args,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client closed")
	}
	_, err = c.conn.Write(append(header, data...))
	c.mu.RUnlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, seq)
		c.mu.Unlock()
		return err
	}

	select {
	case <-future.done:
		return future.err
	case <-c.ctx.Done():
		return fmt.Errorf("connection closed")
	case <-time.After(c.opts.Timeout):
		c.mu.Lock()
		delete(c.pending, seq)
		c.mu.Unlock()
		return fmt.Errorf("timeout")
	}
}

func (c *singleClient) Notify(service, method string, args any) error {
	seq := atomic.AddUint64(&c.seq, 1)

	req := &rpcRequest{
		Seq:     seq,
		Type:    "notify",
		Service: service,
		Method:  method,
		Args:    args,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client closed")
	}

	_, err = c.conn.Write(append(header, data...))
	return err
}

func (c *singleClient) receiveLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(c.opts.Timeout))
		data, err := c.readPacket()
		if err != nil {
			select {
			case <-c.ctx.Done():
				return
			default:
			}
			if c.closed {
				return
			}
			time.Sleep(time.Millisecond)
			continue
		}
		c.handleResponse(data)
	}
}

func (c *singleClient) readPacket() ([]byte, error) {
	header := make([]byte, 4)
	if _, err := c.readFull(header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)

	if length > 1024*1024 {
		return nil, fmt.Errorf("packet too large: %d", length)
	}

	data := make([]byte, length)
	if _, err := c.readFull(data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *singleClient) readFull(buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		c.conn.SetReadDeadline(time.Now().Add(c.opts.Timeout))
		nn, err := c.conn.Read(buf[n:])
		if err != nil {
			return n, err
		}
		n += nn
	}
	return n, nil
}

func (c *singleClient) handleResponse(data []byte) {
	var resp rpcResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	c.mu.Lock()
	future, ok := c.pending[resp.Seq]
	if !ok {
		c.mu.Unlock()
		return
	}
	delete(c.pending, resp.Seq)

	if future.reply != nil {
		if resp.Error != "" {
			future.err = fmt.Errorf(resp.Error)
		} else if resp.Reply != nil {
			replyData, err := json.Marshal(resp.Reply)
			if err != nil {
				future.err = fmt.Errorf("marshal reply: %w", err)
			} else if err := json.Unmarshal(replyData, future.reply); err != nil {
				future.err = fmt.Errorf("unmarshal reply: %w", err)
			}
		}
	}
	close(future.done)
	c.mu.Unlock()
}

func (c *singleClient) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	for _, f := range c.pending {
		f.err = fmt.Errorf("client closed")
		close(f.done)
	}
	c.pending = make(map[uint64]*rpcFuture)
	c.mu.Unlock()
	c.cancel()
	c.conn.Close()
}
