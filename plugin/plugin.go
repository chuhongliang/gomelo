package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	for _, p := range plugins {
		done := make(chan error, 1)
		go func() {
			done <- p.Initialize()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("plugin %s initialize timeout", p.Name())
		case err := <-done:
			if err != nil {
				return fmt.Errorf("plugin %s initialize failed: %w", p.Name(), err)
			}
		}
	}

	return nil
}

func (m *PluginManager) AfterInitialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	for _, p := range plugins {
		done := make(chan error, 1)
		go func() {
			done <- p.AfterInitialize()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("plugin %s after initialize timeout", p.Name())
		case err := <-done:
			if err != nil {
				return fmt.Errorf("plugin %s after initialize failed: %w", p.Name(), err)
			}
		}
	}

	return nil
}

func (m *PluginManager) BeforeStart() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	for _, p := range plugins {
		done := make(chan error, 1)
		go func() {
			done <- p.BeforeStart()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("plugin %s before start timeout", p.Name())
		case err := <-done:
			if err != nil {
				return fmt.Errorf("plugin %s before start failed: %w", p.Name(), err)
			}
		}
	}

	return nil
}

func (m *PluginManager) AfterStart() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	for _, p := range plugins {
		done := make(chan error, 1)
		go func() {
			done <- p.AfterStart()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("plugin %s after start timeout", p.Name())
		case err := <-done:
			if err != nil {
				return fmt.Errorf("plugin %s after start failed: %w", p.Name(), err)
			}
		}
	}

	return nil
}

func (m *PluginManager) BeforeStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	for i := len(plugins) - 1; i >= 0; i-- {
		done := make(chan error, 1)
		go func() {
			done <- plugins[i].BeforeStop()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("plugin %s before stop timeout", plugins[i].Name())
		case err := <-done:
			if err != nil {
				return fmt.Errorf("plugin %s before stop failed: %w", plugins[i].Name(), err)
			}
		}
	}

	return nil
}

func (m *PluginManager) AfterStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.mu.RLock()
	plugins := m.GetAll()
	m.mu.RUnlock()

	for i := len(plugins) - 1; i >= 0; i-- {
		done := make(chan error, 1)
		go func() {
			done <- plugins[i].AfterStop()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("plugin %s after stop timeout", plugins[i].Name())
		case err := <-done:
			if err != nil {
				return fmt.Errorf("plugin %s after stop failed: %w", plugins[i].Name(), err)
			}
		}
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
