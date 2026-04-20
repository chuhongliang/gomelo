package plugin

import (
	"fmt"
	"sync"
)

type Plugin interface {
	Name() string
	Initialize() error
	AfterInitialize() error
	BeforeStart() error
	AfterStart() error
	BeforeStop() error
	AfterStop() error
}

type BasePlugin struct {
	name string
}

func (p *BasePlugin) Name() string { return p.name }

func (p *BasePlugin) Initialize() error      { return nil }
func (p *BasePlugin) AfterInitialize() error { return nil }
func (p *BasePlugin) BeforeStart() error     { return nil }
func (p *BasePlugin) AfterStart() error      { return nil }
func (p *BasePlugin) BeforeStop() error      { return nil }
func (p *BasePlugin) AfterStop() error       { return nil }

type PluginManager struct {
	plugins map[string]Plugin
	mu      sync.RWMutex
	order   []string
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[string]Plugin),
		order:   make([]string, 0),
	}
}

func (m *PluginManager) Install(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, ok := m.plugins[name]; ok {
		return fmt.Errorf("plugin %s already installed", name)
	}

	m.plugins[name] = p
	m.order = append(m.order, name)
	return nil
}

func (m *PluginManager) Uninstall(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plugins[name]; !ok {
		return fmt.Errorf("plugin %s not found", name)
	}

	delete(m.plugins, name)

	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}

	return nil
}

func (m *PluginManager) Get(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

func (m *PluginManager) GetAll() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Plugin, 0, len(m.order))
	for _, name := range m.order {
		result = append(result, m.plugins[name])
	}
	return result
}

func (m *PluginManager) Initialize() error {
	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	type pluginResult struct {
		name string
		err  error
	}
	results := make(chan pluginResult, len(plugins))
	var wg sync.WaitGroup

	for _, p := range plugins {
		wg.Add(1)
		go func(pl Plugin) {
			defer wg.Done()
			err := pl.Initialize()
			results <- pluginResult{name: pl.Name(), err: err}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("plugin %s initialize failed: %w", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (m *PluginManager) AfterInitialize() error {
	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	type pluginResult struct {
		name string
		err  error
	}
	results := make(chan pluginResult, len(plugins))
	var wg sync.WaitGroup

	for _, p := range plugins {
		wg.Add(1)
		go func(pl Plugin) {
			defer wg.Done()
			err := pl.AfterInitialize()
			results <- pluginResult{name: pl.Name(), err: err}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("plugin %s after initialize failed: %w", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (m *PluginManager) BeforeStart() error {
	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	type pluginResult struct {
		name string
		err  error
	}
	results := make(chan pluginResult, len(plugins))
	var wg sync.WaitGroup

	for _, p := range plugins {
		wg.Add(1)
		go func(pl Plugin) {
			defer wg.Done()
			err := pl.BeforeStart()
			results <- pluginResult{name: pl.Name(), err: err}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("plugin %s before start failed: %w", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (m *PluginManager) AfterStart() error {
	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	type pluginResult struct {
		name string
		err  error
	}
	results := make(chan pluginResult, len(plugins))
	var wg sync.WaitGroup

	for _, p := range plugins {
		wg.Add(1)
		go func(pl Plugin) {
			defer wg.Done()
			err := pl.AfterStart()
			results <- pluginResult{name: pl.Name(), err: err}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("plugin %s after start failed: %w", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (m *PluginManager) BeforeStop() error {
	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	type pluginResult struct {
		name string
		err  error
	}
	results := make(chan pluginResult, len(plugins))
	var wg sync.WaitGroup

	for i := len(plugins) - 1; i >= 0; i-- {
		wg.Add(1)
		go func(pl Plugin) {
			defer wg.Done()
			err := pl.BeforeStop()
			results <- pluginResult{name: pl.Name(), err: err}
		}(plugins[i])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("plugin %s before stop failed: %w", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (m *PluginManager) AfterStop() error {
	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	type pluginResult struct {
		name string
		err  error
	}
	results := make(chan pluginResult, len(plugins))
	var wg sync.WaitGroup

	for i := len(plugins) - 1; i >= 0; i-- {
		wg.Add(1)
		go func(pl Plugin) {
			defer wg.Done()
			err := pl.AfterStop()
			results <- pluginResult{name: pl.Name(), err: err}
		}(plugins[i])
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for result := range results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("plugin %s after stop failed: %w", result.name, result.err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (m *PluginManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}

func (m *PluginManager) Clear() {
	m.mu.Lock()
	m.plugins = make(map[string]Plugin)
	m.order = make([]string, 0)
	m.mu.Unlock()
}

type HookType int

const (
	HookBeforeInitialize HookType = iota
	HookAfterInitialize
	HookBeforeStart
	HookAfterStart
	HookBeforeStop
	HookAfterStop
)

type HookFunc func() error

type HookManager struct {
	hooks map[HookType][]HookFunc
	mu    sync.RWMutex
}

func NewHookManager() *HookManager {
	return &HookManager{
		hooks: make(map[HookType][]HookFunc),
	}
}

func (m *HookManager) Register(hookType HookType, fn HookFunc) {
	m.mu.Lock()
	m.hooks[hookType] = append(m.hooks[hookType], fn)
	m.mu.Unlock()
}

func (m *HookManager) Execute(hookType HookType) error {
	m.mu.RLock()
	funcs, ok := m.hooks[hookType]
	m.mu.RUnlock()

	if !ok || len(funcs) == 0 {
		return nil
	}

	for _, fn := range funcs {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

func (m *HookManager) Clear(hookType HookType) {
	m.mu.Lock()
	delete(m.hooks, hookType)
	m.mu.Unlock()
}

func (m *HookManager) ClearAll() {
	m.mu.Lock()
	m.hooks = make(map[HookType][]HookFunc)
	m.mu.Unlock()
}
