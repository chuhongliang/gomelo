package rpc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"
)

type RPCServer interface {
	Register(service string, impl any) error
	Start() error
	Stop()
	Addrs() map[string]string
}

type rpcHandler struct {
	receiver any
	method   reflect.Method
}

type ServerOptions struct {
	Timeout time.Duration
}

type rpcServer struct {
	addr      string
	listener  net.Listener
	handlers  map[string]map[string]*rpcHandler
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	timeout   time.Duration
	semaphore chan struct{}
}

func NewServer(addr string) RPCServer {
	return NewServerWithOptions(addr, nil)
}

func NewServerWithOptions(addr string, opts *ServerOptions) RPCServer {
	if opts == nil {
		opts = &ServerOptions{Timeout: 30 * time.Second}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &rpcServer{
		addr:      addr,
		handlers:  make(map[string]map[string]*rpcHandler),
		ctx:       ctx,
		cancel:    cancel,
		stopCh:    make(chan struct{}),
		timeout:   opts.Timeout,
		semaphore: make(chan struct{}, 1000),
	}
}

func (s *rpcServer) Register(service string, impl any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handlers[service] != nil {
		return fmt.Errorf("service %s already registered", service)
	}

	t := reflect.TypeOf(impl)
	v := reflect.ValueOf(impl)

	methods := make(map[string]*rpcHandler)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		if m.Type.NumIn() != 3 || m.Type.NumOut() != 1 {
			continue
		}

		if m.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() {
			continue
		}

		methods[m.Name] = &rpcHandler{
			receiver: v.Interface(),
			method:   m,
		}
	}

	if len(methods) == 0 {
		return fmt.Errorf("no valid methods found for service %s", service)
	}

	s.handlers[service] = methods
	return nil
}

func (s *rpcServer) Start() error {
	if s.running {
		return nil
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.listener = ln

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

func (s *rpcServer) acceptLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()
			if !running {
				return
			}
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *rpcServer) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	acquired := false
	for !acquired {
		select {
		case s.semaphore <- struct{}{}:
			acquired = true
		case <-s.ctx.Done():
			return
		}
	}
	defer func() { <-s.semaphore }()

	for {
		header := make([]byte, 4)
		if err := s.readFull(conn, header); err != nil {
			return
		}

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		size := binary.BigEndian.Uint32(header)
		if size > 1024*1024 {
			return
		}

		body := make([]byte, size)
		if err := s.readFull(conn, body); err != nil {
			return
		}

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		s.handleRequest(conn, body)
	}
}

func (s *rpcServer) readFull(conn net.Conn, buf []byte) error {
	n := 0
	for n < len(buf) {
		nn, err := conn.Read(buf[n:])
		if err != nil {
			return err
		}
		n += nn
	}
	return nil
}

func (s *rpcServer) handleRequest(conn net.Conn, body []byte) {
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	s.mu.RLock()
	service, ok := s.handlers[req.Service]
	s.mu.RUnlock()

	if !ok || service == nil {
		resp := rpcResponse{
			Seq:   req.Seq,
			Error: "service not found",
		}
		s.sendResponse(conn, req.Seq, resp)
		return
	}

	handler, ok := service[req.Method]
	if !ok || handler == nil {
		resp := rpcResponse{
			Seq:   req.Seq,
			Error: "method not found",
		}
		s.sendResponse(conn, req.Seq, resp)
		return
	}

	var args any
	if req.Args != nil {
		args = req.Args
	}

	if handler.receiver == nil {
		resp := rpcResponse{
			Seq:   req.Seq,
			Error: "handler receiver is nil",
		}
		s.sendResponse(conn, req.Seq, resp)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	var result []reflect.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				result = nil
			}
		}()
		result = handler.method.Func.Call([]reflect.Value{
			reflect.ValueOf(handler.receiver),
			reflect.ValueOf(ctx),
			reflect.ValueOf(args),
		})
	}()

	if result == nil {
		resp := rpcResponse{
			Seq:   req.Seq,
			Error: "handler panic",
		}
		s.sendResponse(conn, req.Seq, resp)
		return
	}

	var reply any
	if len(result) > 0 {
		v := result[0]
		if v.Kind() == reflect.Ptr && !v.IsNil() {
			reply = v.Interface()
		}
	}

	resp := rpcResponse{
		Seq:   req.Seq,
		Reply: reply,
	}

	s.sendResponse(conn, req.Seq, resp)
}

func (s *rpcServer) sendResponse(conn net.Conn, seq uint64, resp rpcResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	_, err = conn.Write(append(header, data...))
	return err
}

func (s *rpcServer) Stop() {
	if !s.running {
		return
	}

	s.running = false
	s.cancel()
	close(s.stopCh)

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
}

func (s *rpcServer) Addrs() map[string]string {
	return map[string]string{"rpc": s.addr}
}
