package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/chuhongliang/gomelo/plugin"
	"github.com/chuhongliang/gomelo/rpc"
)

const (
	StateInited  int = 1
	StateStart   int = 2
	StateStarted int = 3
	StateStopped int = 4
)

type Component interface {
	Name() string
	Start(app *App) error
	Stop() error
}

type Filter interface {
	Name() string
	Process(ctx *Context) bool
	After(ctx *Context)
}

type HandlerFunc func(*Context)

type Middleware func(HandlerFunc) HandlerFunc

type RouteHandler func(*Context, string) string

type ConnectHandler func(*Session)
type MessageHandler func(*Session, *Message)
type CloseHandler func(*Session)

type ServerOption struct {
	Env        string
	Host       string
	Port       int
	Timeout    time.Duration
	StackTrace bool
	ServerID   string
	MasterAddr string
}

type Server struct {
	app         *App
	serverType  string
	frontend    bool
	port        int
	host        string
	onConnect   ConnectHandler
	onMessage   MessageHandler
	onClose     CloseHandler
	connections int64
	maxConns    int
}

func (s *Server) SetFrontend(v bool)     { s.frontend = v }
func (s *Server) SetPort(port int)       { s.port = port }
func (s *Server) SetHost(host string)    { s.host = host }
func (s *Server) SetServerType(t string) { s.serverType = t }
func (s *Server) Frontend() bool         { return s.frontend }
func (s *Server) Port() int              { return s.port }
func (s *Server) Host() string           { return s.host }
func (s *Server) ServerType() string     { return s.serverType }

func (a *App) SetHost(host string)        { a.Set("host", host) }
func (a *App) SetPort(port int)          { a.Set("port", port) }
func (a *App) SetMasterAddr(addr string) { a.Set("masterAddr", addr) }

func (s *Server) OnConnection(fn ConnectHandler) { s.onConnect = fn }
func (s *Server) OnMessage(fn MessageHandler)    { s.onMessage = fn }
func (s *Server) OnClose(fn CloseHandler)        { s.onClose = fn }

func (s *Server) Name() string {
	if s.serverType != "" {
		return s.serverType
	}
	return "server"
}

func (s *Server) Start(app *App) error {
	s.app = app
	return nil
}

func (s *Server) Stop() {}

type AppOption func(*ServerOption)

func WithEnv(env string) AppOption {
	return func(o *ServerOption) { o.Env = env }
}

func WithHost(host string) AppOption {
	return func(o *ServerOption) { o.Host = host }
}

func WithPort(port int) AppOption {
	return func(o *ServerOption) { o.Port = port }
}

func WithServerID(id string) AppOption {
	return func(o *ServerOption) { o.ServerID = id }
}

func WithMasterAddr(addr string) AppOption {
	return func(o *ServerOption) { o.MasterAddr = addr }
}

type MasterConfig struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type MasterConfigFile struct {
	Development *MasterConfig `json:"development,omitempty"`
	Production  *MasterConfig `json:"production,omitempty"`
}

func LoadMasterConfig(path string, env string) (*MasterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read master.json failed: %w", err)
	}

	var cfg MasterConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse master.json failed: %w", err)
	}

	switch env {
	case "development":
		if cfg.Development != nil {
			return cfg.Development, nil
		}
	case "production":
		if cfg.Production != nil {
			return cfg.Production, nil
		}
	}

	return nil, fmt.Errorf("master config not found for env: %s", env)
}

func (a *App) AutoSetup(configDir string) error {
	env := os.Getenv("GOMELO_ENV")
	if env == "" {
		env = "development"
	}
	a.Set("env", env)

	serverID := os.Getenv("GOMELO_SERVER_ID")
	if serverID == "" {
		return fmt.Errorf("GOMELO_SERVER_ID environment variable is required")
	}

	masterPath := filepath.Join(configDir, "master.json")
	masterCfg, err := LoadMasterConfig(masterPath, env)
	if err != nil {
		return fmt.Errorf("load master config failed: %w", err)
	}
	a.SetMasterAddr(fmt.Sprintf("%s:%d", masterCfg.Host, masterCfg.Port))

	serversPath := filepath.Join(configDir, "servers.json")
	servers, err := LoadServersConfig(serversPath, env)
	if err != nil {
		return fmt.Errorf("load servers config failed: %w", err)
	}

	var curServer map[string]any
	for _, s := range servers {
		if s["id"] == serverID {
			curServer = s
			break
		}
	}
	if curServer == nil {
		return fmt.Errorf("server %s not found in servers.json", serverID)
	}

	if host, ok := curServer["host"].(string); ok {
		a.SetHost(host)
	}
	if port, ok := curServer["port"].(float64); ok {
		a.SetPort(int(port))
	}
	a.SetServerId(serverID)
	if st, ok := curServer["serverType"].(string); ok {
		a.SetServerType(st)
	}

	a.SetCurServer(curServer)

	return nil
}

func (a *App) AutoConfigure(fn func(*Server)) {
	st := a.GetServerType()
	if st == "" {
		return
	}
	a.Configure(st, st)(fn)
}

type Context struct {
	app      *App
	session  *Session
	request  *Message
	Resp     *Message
	Route    string
	Type     MessageType
	handlers []HandlerFunc
	index    int
}

func NewContext(app *App) *Context {
	return &Context{app: app, index: -1}
}

func (c *Context) App() *App         { return c.app }
func (c *Context) Session() *Session { return c.session }
func (c *Context) Request() *Message { return c.request }
func (c *Context) RouteName() string { return c.Route }

func (c *Context) SetSession(s *Session)  { c.session = s }
func (c *Context) SetRequest(r *Message)  { c.request = r }
func (c *Context) SetResponse(r *Message) { c.Resp = r }

func (c *Context) Bind(v any) error {
	if c.request == nil || c.request.Body == nil {
		return nil
	}
	if data, ok := c.request.Body.([]byte); ok {
		return json.Unmarshal(data, v)
	}
	return nil
}

func (c *Context) Response(body any) {
	c.Resp = &Message{Type: Response, Route: c.Route, Body: body}
}

func (c *Context) ResponseOK(data any) {
	c.Response(map[string]any{"code": 0, "msg": "ok", "data": data})
}

func (c *Context) ResponseError(code int, msg string) {
	c.Response(map[string]any{"code": code, "msg": msg})
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

func (c *Context) Set(key string, value any) {
	if c.session != nil {
		c.session.Set(key, value)
	}
}

func (c *Context) Get(key string) any {
	if c.session != nil {
		return c.session.Get(key)
	}
	return nil
}

type App struct {
	loaded       []Component
	components   map[string]Component
	settings     map[string]any
	state        int
	base         string
	startTimeout time.Duration
	stopTimeout  time.Duration

	event *EventEmitter

	serverId   string
	serverType string
	curServer  map[string]any
	startTime  int64

	master         map[string]any
	servers        map[string]map[string]any
	serverTypeMaps map[string][]map[string]any
	serverTypes    []string

	router    *Router
	pipeline  *Pipeline
	pluginMgr *plugin.PluginManager
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	stopWg    sync.WaitGroup

	rpcMgr RPCClientManager
}

func NewApp(opts ...AppOption) *App {
	o := &ServerOption{
		Timeout:    10 * time.Second,
		StackTrace: true,
		Env:        "development",
	}
	for _, opt := range opts {
		opt(o)
	}

	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		loaded:         make([]Component, 0),
		components:     make(map[string]Component),
		settings:       make(map[string]any),
		state:          StateInited,
		startTimeout:   30 * time.Second,
		event:          NewEventEmitter(),
		servers:        make(map[string]map[string]any),
		serverTypeMaps: make(map[string][]map[string]any),
		serverTypes:    make([]string, 0),
		router:         NewRouter(),
		pipeline:       NewPipeline(),
		ctx:            ctx,
		cancel:         cancel,
	}
	app.Set("env", o.Env)
	return app
}

func (a *App) GetBase() string { return a.base }
func (a *App) SetBase(base string) {
	a.base = base
	a.Set("base", base)
}
func (a *App) GetServerId() string                   { return a.serverId }
func (a *App) SetServerId(id string)                 { a.serverId = id }
func (a *App) GetServerType() string                 { return a.serverType }
func (a *App) SetServerType(t string)                { a.serverType = t }
func (a *App) GetCurServer() map[string]any          { return a.curServer }
func (a *App) SetCurServer(server map[string]any)    { a.curServer = server }
func (a *App) GetMaster() map[string]any             { return a.master }
func (a *App) SetMaster(master map[string]any)       { a.master = master }
func (a *App) Event() *EventEmitter                  { return a.event }
func (a *App) GetServers() map[string]map[string]any { return a.servers }

func (a *App) SetServers(servers map[string]map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.servers = servers
	a.serverTypeMaps = make(map[string][]map[string]any)
	a.serverTypes = make([]string, 0)
	for _, server := range servers {
		serverType, ok := server["serverType"].(string)
		if !ok {
			continue
		}
		a.serverTypeMaps[serverType] = append(a.serverTypeMaps[serverType], server)
		if !containsString(a.serverTypes, serverType) {
			a.serverTypes = append(a.serverTypes, serverType)
		}
	}
}

func (a *App) GetServerTypes() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.serverTypes
}

func (a *App) GetServersByType(serverType string) []map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.serverTypeMaps[serverType]
}

func (a *App) GetServerById(serverId string) (map[string]any, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.servers[serverId]
	return s, ok
}
func (a *App) IsFrontend() bool {
	if a.curServer == nil {
		return false
	}
	if v, ok := a.curServer["frontend"].(bool); ok {
		return v
	}
	if v, ok := a.curServer["frontend"].(string); ok {
		return v == "true"
	}
	return false
}

func (a *App) IsBackend() bool { return !a.IsFrontend() }
func (a *App) IsMaster() bool  { return a.serverType == "master" }

func (a *App) AddServers(servers []map[string]any) {
	a.mu.Lock()
	for _, item := range servers {
		id, ok := item["id"].(string)
		if !ok {
			continue
		}
		serverType, ok := item["serverType"].(string)
		if !ok {
			continue
		}
		a.servers[id] = item
		slist := a.serverTypeMaps[serverType]
		if slist == nil {
			slist = make([]map[string]any, 0)
			a.serverTypeMaps[serverType] = slist
		}
		replaceServer(&slist, item)
		a.serverTypeMaps[serverType] = slist
		if !containsString(a.serverTypes, serverType) {
			a.serverTypes = append(a.serverTypes, serverType)
		}
	}
	serversCopy := copyServers(servers)
	a.event.Emit("add_servers", serversCopy)
	a.mu.Unlock()
}

func copyServers(servers []map[string]any) []map[string]any {
	result := make([]map[string]any, len(servers))
	for i, s := range servers {
		m := make(map[string]any)
		for k, v := range s {
			m[k] = v
		}
		result[i] = m
	}
	return result
}

func (a *App) RemoveServers(ids []string) {
	a.mu.Lock()
	for _, id := range ids {
		if item, ok := a.servers[id]; ok {
			delete(a.servers, id)
			serverType, ok := item["serverType"].(string)
			if !ok {
				continue
			}
			slist := a.serverTypeMaps[serverType]
			removeServer(&slist, id)
			a.serverTypeMaps[serverType] = slist
		}
	}
	idsCopy := make([]string, len(ids))
	copy(idsCopy, ids)
	a.event.Emit("remove_servers", idsCopy)
	a.mu.Unlock()
}

func (a *App) ReplaceServers(servers map[string]map[string]any) {
	a.mu.Lock()
	a.servers = servers
	a.serverTypeMaps = make(map[string][]map[string]any)
	a.serverTypes = make([]string, 0)
	for _, server := range servers {
		serverType, ok := server["serverType"].(string)
		if !ok {
			continue
		}
		slist := a.serverTypeMaps[serverType]
		if slist == nil {
			slist = make([]map[string]any, 0)
		}
		slist = append(slist, server)
		a.serverTypeMaps[serverType] = slist
		if !containsString(a.serverTypes, serverType) {
			a.serverTypes = append(a.serverTypes, serverType)
		}
	}
	serversCopy := copyServersMap(servers)
	a.event.Emit("replace_servers", serversCopy)
	a.mu.Unlock()
}

func copyServersMap(servers map[string]map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for k, v := range servers {
		m := make(map[string]any)
		for kk, vv := range v {
			m[kk] = vv
		}
		result[k] = m
	}
	return result
}

func (a *App) Set(setting string, val any) {
	a.settings[setting] = val
}

func (a *App) Get(setting string) any          { return a.settings[setting] }
func (a *App) Enable(setting string)           { a.Set(setting, true) }
func (a *App) Disable(setting string)          { a.Set(setting, false) }
func (a *App) Enabled(setting string) bool     { return a.Get(setting) == true }
func (a *App) Disabled(setting string) bool    { return a.Get(setting) == false }
func (a *App) SetStartTimeout(d time.Duration) { a.startTimeout = d }
func (a *App) SetStopTimeout(d time.Duration)  { a.stopTimeout = d }

type ServersConfig struct {
	Development map[string][]map[string]any `json:"development"`
	Production  map[string][]map[string]any `json:"production"`
}

func LoadServersConfig(path string, env string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read servers.json failed: %w", err)
	}

	var cfg ServersConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse servers.json failed: %w", err)
	}

	var servers []map[string]any
	switch env {
	case "development":
		for st, list := range cfg.Development {
			for _, s := range list {
				s["serverType"] = st
				servers = append(servers, s)
			}
		}
	case "production":
		for st, list := range cfg.Production {
			for _, s := range list {
				s["serverType"] = st
				servers = append(servers, s)
			}
		}
	default:
		return nil, fmt.Errorf("unknown env: %s", env)
	}

	return servers, nil
}

func (a *App) LoadServers(path string) error {
	env := "development"
	if e := a.Get("env"); e != nil {
		env = e.(string)
	}
	servers, err := LoadServersConfig(path, env)
	if err != nil {
		return err
	}

	grouped := make(map[string]map[string]any)
	for _, s := range servers {
		st, ok := s["serverType"].(string)
		if !ok {
			continue
		}
		if grouped[st] == nil {
			grouped[st] = make(map[string]any)
		}
		id, _ := s["id"].(string)
		grouped[st][id] = s
	}

	serverMaps := make(map[string]map[string]any)
	for st, m := range grouped {
		serverMaps[st] = make(map[string]any)
		for id, sv := range m {
			serverMaps[st][id] = sv
		}
	}
	a.SetServers(serverMaps)
	return nil
}

func (a *App) Configure(env string, serverType ...string) func(fn func(*Server)) {
	return func(fn func(*Server)) {
		currentEnv := a.Get("env").(string)
		currentType := a.serverType
		st := ""
		if len(serverType) > 0 {
			st = serverType[0]
		}
		if env == "" || env == "all" || currentEnv == env {
			if st == "" || st == "all" || currentType == st {
				fn(&Server{app: a, serverType: st, frontend: false, port: 0})
			}
		}
	}
}

func (a *App) Register(name string, comp Component) {
	a.mu.Lock()
	a.components[name] = comp
	a.loaded = append(a.loaded, comp)
	a.mu.Unlock()
}

func (a *App) GetComponent(name string) (Component, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c, ok := a.components[name]
	return c, ok
}

func (a *App) Load(name string, comp Component) {
	if name == "" {
		name = comp.Name()
	}
	a.mu.Lock()
	if a.components[name] != nil {
		a.mu.Unlock()
		return
	}
	a.loaded = append(a.loaded, comp)
	a.components[name] = comp
	a.mu.Unlock()
}

func (a *App) SetRoute(serverType string, handler RouteHandler) {
	a.router.SetRoute(serverType, handler)
}
func (a *App) Route(serverType string) (RouteHandler, bool) { return a.router.GetRoute(serverType) }
func (a *App) Use(m Middleware)                             { a.pipeline.Use(m) }
func (a *App) On(route string, h HandlerFunc)               { a.pipeline.On(route, h) }

func (a *App) Before(f Filter) {
	a.mu.Lock()
	filters, _ := a.settings["beforeFilter"].([]Filter)
	if filters == nil {
		filters = make([]Filter, 0)
	}
	a.settings["beforeFilter"] = append(filters, f)
	a.mu.Unlock()
}

func (a *App) After(f Filter) {
	a.mu.Lock()
	filters, _ := a.settings["afterFilter"].([]Filter)
	if filters == nil {
		filters = make([]Filter, 0)
	}
	a.settings["afterFilter"] = append(filters, f)
	a.mu.Unlock()
}

func (a *App) GlobalBefore(f Filter) {
	a.mu.Lock()
	filters, _ := a.settings["globalBeforeFilter"].([]Filter)
	if filters == nil {
		filters = make([]Filter, 0)
	}
	a.settings["globalBeforeFilter"] = append(filters, f)
	a.mu.Unlock()
}

func (a *App) GlobalAfter(f Filter) {
	a.mu.Lock()
	filters, _ := a.settings["globalAfterFilter"].([]Filter)
	if filters == nil {
		filters = make([]Filter, 0)
	}
	a.settings["globalAfterFilter"] = append(filters, f)
	a.mu.Unlock()
}

func (a *App) RpcBefore(f Filter) {
	a.mu.Lock()
	filters, _ := a.settings["rpcBeforeFilter"].([]Filter)
	if filters == nil {
		filters = make([]Filter, 0)
	}
	a.settings["rpcBeforeFilter"] = append(filters, f)
	a.mu.Unlock()
}

func (a *App) RpcAfter(f Filter) {
	a.mu.Lock()
	filters, _ := a.settings["rpcAfterFilter"].([]Filter)
	if filters == nil {
		filters = make([]Filter, 0)
	}
	a.settings["rpcAfterFilter"] = append(filters, f)
	a.mu.Unlock()
}

func (a *App) LoadConfig(key string, val any) { a.Set(key, val) }

func (a *App) Transaction(name string, before func() bool, handlers ...func() error) error {
	if before != nil && !before() {
		return nil
	}
	var lastErr error
	for i := 0; i < 3; i++ {
		for _, handler := range handlers {
			if err := handler(); err != nil {
				lastErr = err
				break
			}
		}
		if lastErr == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return lastErr
}

func (a *App) Start(cb func(err error)) {
	if a.state > StateInited {
		if cb != nil {
			cb(nil)
		}
		return
	}

	if a.pluginMgr != nil {
		if err := a.pluginMgr.BeforeStart(); err != nil {
			if cb != nil {
				cb(err)
			}
			return
		}
	}

	a.startTime = time.Now().UnixMilli()
	a.startComponents(func(err error) {
		if err != nil {
			if cb != nil {
				cb(err)
			}
			return
		}
		a.state = StateStart

		if a.pluginMgr != nil {
			if err := a.pluginMgr.AfterStart(); err != nil {
				if cb != nil {
					cb(err)
				}
				return
			}
		}

		a.afterStart(cb)
	})
}

func (a *App) startComponents(cb func(err error)) {
	a.mu.Lock()
	components := make([]Component, len(a.loaded))
	copy(components, a.loaded)
	a.mu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	started := make([]Component, 0)

	timeout := a.startTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	wg.Add(len(components))
	for _, c := range components {
		go func(comp Component) {
			defer wg.Done()
			if err := comp.Start(a); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			started = append(started, comp)
			mu.Unlock()
		}(c)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		mu.Lock()
		if firstErr == nil {
			firstErr = fmt.Errorf("component startup timeout after %v", timeout)
		}
		mu.Unlock()
		for _, comp := range started {
			comp.Stop()
		}
	}

	if firstErr != nil {
		for _, comp := range started {
			comp.Stop()
		}
		cb(firstErr)
		return
	}
	cb(nil)
}

func (a *App) afterStart(cb func(err error)) {
	a.mu.Lock()
	a.state = StateStarted
	usedTime := time.Now().UnixMilli() - a.startTime
	os.Stdout.WriteString(a.serverId + " startup in " + strconv.FormatInt(usedTime, 10) + " ms\n")
	serverId := a.serverId
	a.mu.Unlock()
	a.event.Emit("start_server", serverId)
	if cb != nil {
		cb(nil)
	}
}

func (a *App) Stop(force bool) error {
	a.mu.Lock()
	if a.state > StateStarted {
		a.mu.Unlock()
		return nil
	}
	a.state = StateStopped
	components := make([]Component, len(a.loaded))
	copy(components, a.loaded)
	a.mu.Unlock()

	var errs []error

	if a.pluginMgr != nil {
		if err := a.pluginMgr.BeforeStop(); err != nil {
			errs = append(errs, err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(len(components))
	for i := len(components) - 1; i >= 0; i-- {
		go func(comp Component) {
			defer wg.Done()
			if err := comp.Stop(); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(components[i])
	}

	timeout := a.stopTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		errs = append(errs, fmt.Errorf("stop timeout after %v", timeout))
	}

	if a.pluginMgr != nil {
		if err := a.pluginMgr.AfterStop(); err != nil {
			errs = append(errs, err)
		}
	}

	if force {
		os.Exit(0)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (a *App) Wait() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	os.Stdout.WriteString("\nShutting down...\n")
	a.Stop(false)
}

func (a *App) Context() context.Context { return a.ctx }
func (a *App) Router() *Router          { return a.router }
func (a *App) Pipeline() *Pipeline      { return a.pipeline }

func (a *App) SetPluginManager(pm *plugin.PluginManager) { a.pluginMgr = pm }
func (a *App) PluginManager() *plugin.PluginManager      { return a.pluginMgr }

type RPCClientManager interface {
	GetClient(serverType string) (rpc.RPCClient, error)
	Close()
}

func (a *App) RPCTo(ctx context.Context, serverType, method string, args, reply any) error {
	if a.rpcMgr == nil {
		return fmt.Errorf("rpc client manager not initialized")
	}
	client, err := a.rpcMgr.GetClient(serverType)
	if err != nil {
		return fmt.Errorf("get rpc client for %s: %w", serverType, err)
	}
	return client.InvokeCtx(ctx, serverType, method, args, reply)
}

func (a *App) SetRPCClientManager(mgr RPCClientManager) {
	a.rpcMgr = mgr
}

func replaceServer(slist *[]map[string]any, info map[string]any) {
	for i, s := range *slist {
		if s["id"] == info["id"] {
			(*slist)[i] = info
			return
		}
	}
	*slist = append(*slist, info)
}

func removeServer(slist *[]map[string]any, id string) {
	for i, s := range *slist {
		if s["id"] == id {
			*slist = append((*slist)[:i], (*slist)[i+1:]...)
			return
		}
	}
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func CreateApp(opts ...AppOption) *App {
	return NewApp(opts...)
}
