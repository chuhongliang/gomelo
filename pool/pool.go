package pool

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrPoolExhausted = errors.New("pool exhausted")
	ErrPoolClosed    = errors.New("pool closed")
)

type Pool interface {
	Get() (any, error)
	Put(any)
	Close()
	Stats() (total, idle, active int64)
}

type factory func() (any, error)

type pool struct {
	factory     factory
	maxConns    int
	minConns    int
	maxWait     time.Duration
	idleTimeout time.Duration

	conns     chan any
	active    int64
	total     int64
	mu        sync.RWMutex
	closed    bool
	cleanupCh chan struct{}
	cleanupWg sync.WaitGroup
}

func NewPool(factory factory, maxConns, minConns int, maxWait, idleTimeout time.Duration) Pool {
	if maxConns <= 0 {
		maxConns = 10
	}
	if minConns <= 0 {
		minConns = 1
	}
	if maxWait <= 0 {
		maxWait = 5 * time.Second
	}
	if idleTimeout <= 0 {
		idleTimeout = 5 * time.Minute
	}

	p := &pool{
		factory:     factory,
		maxConns:    maxConns,
		minConns:    minConns,
		maxWait:     maxWait,
		idleTimeout: idleTimeout,
		conns:       make(chan any, maxConns+10),
		cleanupCh:   make(chan struct{}),
	}

	for i := 0; i < minConns; i++ {
		if c, err := factory(); err == nil {
			p.conns <- c
			atomic.AddInt64(&p.total, 1)
			atomic.AddInt64(&p.active, 1)
		}
	}

	go func() {
		p.cleanupWg.Add(1)
		p.cleanupLoop()
		p.cleanupWg.Done()
	}()
	return p
}

func (p *pool) cleanupLoop() {
	ticker := time.NewTicker(p.idleTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-p.cleanupCh:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.conns)
	close(p.cleanupCh)
	p.mu.Unlock()

	p.cleanupWg.Wait()
}

func (p *pool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	currentIdle := len(p.conns)
	for currentIdle > p.minConns {
		select {
		case conn := <-p.conns:
			if conn == nil {
				break
			}
			if closer, ok := conn.(interface{ Close() error }); ok {
				closer.Close()
			}
			atomic.AddInt64(&p.total, -1)
			currentIdle--
		default:
			break
		}
	}
}

func (p *pool) Get() (any, error) {
	if p.closed {
		return nil, ErrPoolClosed
	}

	select {
	case conn := <-p.conns:
		atomic.AddInt64(&p.active, 1)
		return conn, nil
	default:
	}

	p.mu.Lock()
	if atomic.LoadInt64(&p.total) >= int64(p.maxConns) {
		p.mu.Unlock()
		return nil, ErrPoolExhausted
	}
	atomic.AddInt64(&p.total, 1)
	p.mu.Unlock()

	conn, err := p.factory()
	if err != nil {
		atomic.AddInt64(&p.total, -1)
		return nil, err
	}

	atomic.AddInt64(&p.active, 1)
	return conn, nil
}

func (p *pool) Put(conn any) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	atomic.AddInt64(&p.active, -1)

	select {
	case p.conns <- conn:
	default:
	}
	p.mu.Unlock()
}

func (p *pool) Stats() (total, idle, active int64) {
	total = atomic.LoadInt64(&p.total)
	active = atomic.LoadInt64(&p.active)
	idle = int64(len(p.conns))
	return
}

type RPCClientPool struct {
	addr       string
	maxConns   int
	minConns   int
	timeout    time.Duration
	pool       *rpcPool
	mu         sync.RWMutex
	closed     bool
	cleanupCh  chan struct{}
	cleanupWg  sync.WaitGroup
	totalConns int64
}

type rpcPool struct {
	conns chan *RPCConn
	mu    sync.RWMutex
}

type RPCConn struct {
	ID      uint64
	conn    any
	inUse   bool
	lastUse time.Time
}

func NewRPCClientPool(addr string, maxConns, minConns int, timeout time.Duration) (*RPCClientPool, error) {
	if maxConns <= 0 {
		maxConns = 10
	}
	if minConns <= 0 {
		minConns = 1
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	p := &RPCClientPool{
		addr:     addr,
		maxConns: maxConns,
		minConns: minConns,
		timeout:  timeout,
		pool: &rpcPool{
			conns: make(chan *RPCConn, maxConns+10),
		},
		cleanupCh: make(chan struct{}),
	}

	for i := 0; i < minConns; i++ {
		c, err := p.createConn()
		if err == nil {
			p.pool.conns <- c
		}
	}

	go func() {
		p.cleanupWg.Add(1)
		p.cleanupLoop()
		p.cleanupWg.Done()
	}()
	return p, nil
}

func (p *RPCClientPool) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.cleanupCh:
			return
		case <-ticker.C:
			p.cleanupIdleConnections()
		}
	}
}

func (p *RPCClientPool) cleanupIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	currentIdle := len(p.pool.conns)
	for currentIdle > p.minConns {
		select {
		case conn := <-p.pool.conns:
			if conn.conn != nil {
				if tc, ok := conn.conn.(net.Conn); ok {
					tc.Close()
				}
			}
			atomic.AddInt64(&p.totalConns, -1)
			currentIdle--
		default:
			break
		}
	}
}

func (p *RPCClientPool) createConn() (*RPCConn, error) {
	conn, err := net.DialTimeout("tcp", p.addr, p.timeout)
	if err != nil {
		return nil, err
	}

	return &RPCConn{
		ID:      0,
		conn:    conn,
		inUse:   false,
		lastUse: time.Now(),
	}, nil
}

func (p *RPCClientPool) Get() (*RPCConn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	select {
	case conn := <-p.pool.conns:
		conn.inUse = true
		p.mu.Unlock()
		return conn, nil
	default:
	}

	if atomic.LoadInt64(&p.totalConns) >= int64(p.maxConns) {
		p.mu.Unlock()
		return nil, ErrPoolExhausted
	}
	atomic.AddInt64(&p.totalConns, 1)
	p.mu.Unlock()

	conn, err := p.createConn()
	if err != nil {
		atomic.AddInt64(&p.totalConns, -1)
		return nil, err
	}

	conn.inUse = true
	return conn, nil
}

func (p *RPCClientPool) Put(conn *RPCConn) {
	if conn == nil {
		return
	}

	conn.inUse = false
	conn.lastUse = time.Now()

	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		if conn.conn != nil {
			if tc, ok := conn.conn.(net.Conn); ok {
				tc.Close()
			}
		}
		atomic.AddInt64(&p.totalConns, -1)
		return
	}

	select {
	case p.pool.conns <- conn:
	default:
		if conn.conn != nil {
			if tc, ok := conn.conn.(net.Conn); ok {
				tc.Close()
			}
		}
		atomic.AddInt64(&p.totalConns, -1)
	}
}

func (p *RPCClientPool) Close() {
	p.mu.Lock()
	p.closed = true
	close(p.pool.conns)
	close(p.cleanupCh)
	p.mu.Unlock()

	p.cleanupWg.Wait()
}

func (p *RPCClientPool) Stats() (total, idle, active int) {
	l := len(p.pool.conns)
	return p.maxConns, l, p.maxConns - l
}

type WorkerPool struct {
	jobs    chan func()
	workers int
	wg      sync.WaitGroup
	mu      sync.RWMutex
	closed  bool
}

func NewWorkerPool(workers, queueSize int) *WorkerPool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 1024
	}

	p := &WorkerPool{
		jobs:    make(chan func(), queueSize),
		workers: workers,
	}

	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	return p
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for fn := range p.jobs {
		fn()
	}
}

func (p *WorkerPool) Submit(fn func()) error {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		return ErrPoolClosed
	}

	select {
	case p.jobs <- fn:
		return nil
	default:
		return ErrPoolExhausted
	}
}

func (p *WorkerPool) SubmitWithContext(ctx context.Context, fn func()) error {
	done := make(chan struct{}, 1)
	fnWithDone := func() {
		fn()
		close(done)
	}

	if err := p.Submit(fnWithDone); err != nil {
		return err
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *WorkerPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.jobs)
	p.mu.Unlock()

	p.wg.Wait()
}

func (p *WorkerPool) Workers() int {
	return p.workers
}

func (p *WorkerPool) QueueSize() int {
	return cap(p.jobs)
}

func (p *WorkerPool) Pending() int {
	return len(p.jobs)
}
