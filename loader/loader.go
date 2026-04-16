package loader

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"gomelo/lib"
)

type Handler interface {
	Init(app *lib.App)
}

type Remote interface {
	Init(app *lib.App)
}

type BeforeFilter interface {
	Before(ctx *lib.Context) bool
}

type AfterFilter interface {
	After(ctx *lib.Context)
}

type FilterInfo struct {
	Name   string
	Before func(*lib.Context) bool
	After  func(*lib.Context)
}

type HandlerMethod struct {
	Handler any
	Method  reflect.Method
}

type RemoteService struct {
	Name    string
	Methods map[string]reflect.Method
}

type CronMethod struct {
	Cron   any
	Method reflect.Method
}

type Loader struct {
	basePath string
	app      *lib.App

	handlers map[string]map[string]*HandlerMethod
	remotes  map[string]map[string]*RemoteService
	filters  map[string][]FilterInfo
	crons    map[string]map[string]*CronMethod

	loaded bool
	mu     sync.RWMutex
}

var globalLoader *Loader
var globalMu sync.RWMutex

func NewLoader(basePath string) *Loader {
	if basePath == "" {
		basePath = "game-server/app/servers"
	}
	return &Loader{
		basePath: basePath,
		handlers: make(map[string]map[string]*HandlerMethod),
		remotes:  make(map[string]map[string]*RemoteService),
		filters:  make(map[string][]FilterInfo),
		crons:    make(map[string]map[string]*CronMethod),
	}
}

func GlobalLoader() *Loader {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLoader
}

func SetGlobalLoader(l *Loader) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLoader = l
}

func (l *Loader) SetApp(app *lib.App) {
	l.app = app
}

func (l *Loader) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.loaded {
		return nil
	}

	serverTypes := l.discoverServerTypes()
	for _, st := range serverTypes {
		if err := l.loadHandlers(st); err != nil {
			return fmt.Errorf("load handlers for %s: %w", st, err)
		}
		if err := l.loadRemotes(st); err != nil {
			return fmt.Errorf("load remotes for %s: %w", st, err)
		}
		if err := l.loadFilters(st); err != nil {
			return fmt.Errorf("load filters for %s: %w", st, err)
		}
		if err := l.loadCrons(st); err != nil {
			return fmt.Errorf("load crons for %s: %w", st, err)
		}
	}
	l.loaded = true
	return nil
}

func (l *Loader) Reload() error {
	l.mu.Lock()

	oldHandlers := l.handlers
	oldRemotes := l.remotes
	oldFilters := l.filters
	oldCrons := l.crons
	oldLoaded := l.loaded

	l.loaded = false
	l.handlers = make(map[string]map[string]*HandlerMethod)
	l.remotes = make(map[string]map[string]*RemoteService)
	l.filters = make(map[string][]FilterInfo)
	l.crons = make(map[string]map[string]*CronMethod)

	l.mu.Unlock()

	err := l.Load()
	if err != nil {
		l.mu.Lock()
		l.handlers = oldHandlers
		l.remotes = oldRemotes
		l.filters = oldFilters
		l.crons = oldCrons
		l.loaded = oldLoaded
		l.mu.Unlock()
	}
	return err
}

func (l *Loader) discoverServerTypes() []string {
	entries, err := filepath.Glob(filepath.Join(l.basePath, "*"))
	if err != nil {
		return []string{}
	}

	types := make([]string, 0)
	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(entry)
		if strings.HasPrefix(name, ".") {
			continue
		}
		types = append(types, name)
	}
	sort.Strings(types)
	return types
}

type HandlerRegisterFunc func(l *Loader, serverType string)
type RemoteRegisterFunc func(l *Loader, serverType string)
type FilterRegisterFunc func(l *Loader, serverType string)
type CronRegisterFunc func(l *Loader, serverType string)

var (
	handlerRegFuncs = make(map[string]HandlerRegisterFunc)
	remoteRegFuncs  = make(map[string]RemoteRegisterFunc)
	filterRegFuncs  = make(map[string]FilterRegisterFunc)
	cronRegFuncs    = make(map[string]CronRegisterFunc)
	regMu           sync.Mutex
)

func RegisterHandler(filePath string, fn HandlerRegisterFunc) {
	regMu.Lock()
	defer regMu.Unlock()
	handlerRegFuncs[filePath] = fn
}

func RegisterRemote(filePath string, fn RemoteRegisterFunc) {
	regMu.Lock()
	defer regMu.Unlock()
	remoteRegFuncs[filePath] = fn
}

func RegisterFilter(filePath string, fn FilterRegisterFunc) {
	regMu.Lock()
	defer regMu.Unlock()
	filterRegFuncs[filePath] = fn
}

func RegisterCron(filePath string, fn CronRegisterFunc) {
	regMu.Lock()
	defer regMu.Unlock()
	cronRegFuncs[filePath] = fn
}

func (l *Loader) loadHandlers(serverType string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.handlers[serverType] = make(map[string]*HandlerMethod)

	handlerPath := filepath.Join(l.basePath, serverType, "handler")
	entries, err := filepath.Glob(handlerPath + "/*.go")
	if err != nil || len(entries) == 0 {
		return nil
	}

	for _, file := range entries {
		base := filepath.Base(file)
		base = strings.TrimSuffix(base, ".go")
		key := serverType + "/handler/" + base

		if fn, ok := handlerRegFuncs[key]; ok {
			fn(l, serverType)
		}
	}

	return nil
}

func (l *Loader) loadRemotes(serverType string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.remotes[serverType] = make(map[string]*RemoteService)

	remotePath := filepath.Join(l.basePath, serverType, "remote")
	entries, err := filepath.Glob(remotePath + "/*.go")
	if err != nil || len(entries) == 0 {
		return nil
	}

	for _, file := range entries {
		base := filepath.Base(file)
		base = strings.TrimSuffix(base, ".go")
		key := serverType + "/remote/" + base

		if fn, ok := remoteRegFuncs[key]; ok {
			fn(l, serverType)
		}
	}

	return nil
}

func (l *Loader) loadFilters(serverType string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.filters[serverType] = make([]FilterInfo, 0)

	filterPath := filepath.Join(l.basePath, serverType, "filter")
	entries, err := filepath.Glob(filterPath + "/*.go")
	if err != nil || len(entries) == 0 {
		return nil
	}

	for _, file := range entries {
		base := filepath.Base(file)
		base = strings.TrimSuffix(base, ".go")
		key := serverType + "/filter/" + base

		if fn, ok := filterRegFuncs[key]; ok {
			fn(l, serverType)
		}
	}

	return nil
}

func (l *Loader) loadCrons(serverType string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.crons[serverType] = make(map[string]*CronMethod)

	cronPath := filepath.Join(l.basePath, serverType, "cron")
	entries, err := filepath.Glob(cronPath + "/*.go")
	if err != nil || len(entries) == 0 {
		return nil
	}

	for _, file := range entries {
		base := filepath.Base(file)
		base = strings.TrimSuffix(base, ".go")
		key := serverType + "/cron/" + base

		if fn, ok := cronRegFuncs[key]; ok {
			fn(l, serverType)
		}
	}

	return nil
}

func (l *Loader) RegisterHandlerMethod(serverType, route string, handler any, method reflect.Method) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.handlers[serverType] == nil {
		l.handlers[serverType] = make(map[string]*HandlerMethod)
	}

	l.handlers[serverType][route] = &HandlerMethod{
		Handler: handler,
		Method:  method,
	}
}

func (l *Loader) RegisterRemoteMethod(serverType, svcName, methodName string, receiver any, method reflect.Method) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.remotes[serverType] == nil {
		l.remotes[serverType] = make(map[string]*RemoteService)
	}

	if l.remotes[serverType][svcName] == nil {
		l.remotes[serverType][svcName] = &RemoteService{
			Name:    svcName,
			Methods: make(map[string]reflect.Method),
		}
	}

	l.remotes[serverType][svcName].Methods[methodName] = method
}

func (l *Loader) RegisterCronMethod(serverType, cronName, methodName string, cron any, method reflect.Method) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.crons[serverType] == nil {
		l.crons[serverType] = make(map[string]*CronMethod)
	}

	key := cronName + "." + methodName
	l.crons[serverType][key] = &CronMethod{
		Cron:   cron,
		Method: method,
	}
}

func (l *Loader) RegisterFilter(serverType, name string, before func(*lib.Context) bool, after func(*lib.Context)) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.filters[serverType] == nil {
		l.filters[serverType] = make([]FilterInfo, 0)
	}

	l.filters[serverType] = append(l.filters[serverType], FilterInfo{
		Name:   name,
		Before: before,
		After:  after,
	})
}

func (l *Loader) GetHandler(serverType, route string) *HandlerMethod {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if handlers, ok := l.handlers[serverType]; ok {
		if h, ok := handlers[route]; ok {
			return h
		}
	}
	return nil
}

func (l *Loader) GetRemote(serverType, serviceName string) *RemoteService {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if remotes, ok := l.remotes[serverType]; ok {
		if r, ok := remotes[serviceName]; ok {
			return r
		}
	}
	return nil
}

func (l *Loader) GetAllHandlers() map[string]map[string]*HandlerMethod {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]map[string]*HandlerMethod, len(l.handlers))
	for k, v := range l.handlers {
		result[k] = make(map[string]*HandlerMethod, len(v))
		for kk, vv := range v {
			result[k][kk] = &HandlerMethod{
				Handler: vv.Handler,
				Method:  vv.Method,
			}
		}
	}
	return result
}

func (l *Loader) GetAllRemotes() map[string]map[string]*RemoteService {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]map[string]*RemoteService, len(l.remotes))
	for k, v := range l.remotes {
		result[k] = make(map[string]*RemoteService, len(v))
		for kk, vv := range v {
			methodsCopy := make(map[string]reflect.Method, len(vv.Methods))
			for mm, method := range vv.Methods {
				methodsCopy[mm] = method
			}
			result[k][kk] = &RemoteService{
				Name:    vv.Name,
				Methods: methodsCopy,
			}
		}
	}
	return result
}

func (l *Loader) GetFilters(serverType string) []FilterInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if filters, ok := l.filters[serverType]; ok {
		copied := make([]FilterInfo, len(filters))
		copy(copied, filters)
		return copied
	}
	return nil
}

func (l *Loader) GetAllFilters() map[string][]FilterInfo {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string][]FilterInfo, len(l.filters))
	for k, v := range l.filters {
		copied := make([]FilterInfo, len(v))
		copy(copied, v)
		result[k] = copied
	}
	return result
}

func (l *Loader) GetCron(serverType, cronName, methodName string) *CronMethod {
	l.mu.RLock()
	defer l.mu.RUnlock()

	key := cronName + "." + methodName
	if crons, ok := l.crons[serverType]; ok {
		if c, ok := crons[key]; ok {
			return &CronMethod{
				Cron:   c.Cron,
				Method: c.Method,
			}
		}
	}
	return nil
}

func (l *Loader) GetAllCrons() map[string]map[string]*CronMethod {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]map[string]*CronMethod, len(l.crons))
	for k, v := range l.crons {
		result[k] = make(map[string]*CronMethod, len(v))
		for kk, vv := range v {
			result[k][kk] = &CronMethod{
				Cron:   vv.Cron,
				Method: vv.Method,
			}
		}
	}
	return result
}

func (l *Loader) InvokeHandler(serverType, route string, ctx *lib.Context) {
	l.mu.RLock()
	hm := l.handlers[serverType]
	if hm == nil {
		l.mu.RUnlock()
		return
	}
	h := hm[route]
	if h == nil {
		l.mu.RUnlock()
		return
	}
	handler := h.Handler
	method := h.Method
	l.mu.RUnlock()

	if handler == nil || method.Type == nil {
		return
	}

	args := []reflect.Value{
		reflect.ValueOf(handler),
		reflect.ValueOf(ctx),
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("handler panic: serverType=%s, route=%s, err=%v", serverType, route, r)
		}
	}()
	method.Func.Call(args)
}

func BuildRoute(serverType, handlerName, methodName string) string {
	handlerName = strings.ToLower(handlerName)
	methodName = strings.ToLower(methodName)
	return fmt.Sprintf("%s.%s.%s", serverType, handlerName, methodName)
}

func IsHandlerMethod(m reflect.Method) bool {
	if m.Type.NumIn() != 2 {
		return false
	}
	if m.Type.In(1) != reflect.TypeOf((*lib.Context)(nil)).Elem() {
		return false
	}
	return true
}

func IsRemoteMethod(m reflect.Method) bool {
	if m.Type.NumIn() != 3 {
		return false
	}
	if m.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		return false
	}
	return true
}

func IsCronMethod(m reflect.Method) bool {
	numIn := m.Type.NumIn()
	if numIn != 1 && numIn != 2 {
		return false
	}
	if numIn == 2 && m.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		return false
	}
	return true
}

func ReflectValueOf(v any) reflect.Value {
	return reflect.ValueOf(v)
}

type MessageHandler func(*lib.Session, *lib.Message) (any, error)

type HandlerRegistry struct {
	handlers map[string]MessageHandler
	mu       sync.RWMutex
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]MessageHandler),
	}
}

func (r *HandlerRegistry) Register(route string, h MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[route] = h
}

func (r *HandlerRegistry) Get(route string) (MessageHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[route]
	return h, ok
}

func (r *HandlerRegistry) RegisterFromLoader(load *Loader, serverType string) {
	allHandlers := load.GetAllHandlers()[serverType]
	if allHandlers == nil {
		return
	}

	for route, hm := range allHandlers {
		wrapper := makeHandlerWrapper(hm)
		r.Register(route, wrapper)
	}
}

func makeHandlerWrapper(hm *HandlerMethod) MessageHandler {
	return func(s *lib.Session, msg *lib.Message) (any, error) {
		ctx := lib.NewContext(nil)
		ctx.SetSession(s)
		ctx.Route = msg.Route
		ctx.SetRequest(msg)

		args := []reflect.Value{
			reflect.ValueOf(hm.Handler),
			reflect.ValueOf(ctx),
		}
		hm.Method.Func.Call(args)

		if ctx.Resp != nil {
			return ctx.Resp.Body, nil
		}
		return nil, nil
	}
}
