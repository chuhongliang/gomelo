package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrConfigNotFound = errors.New("config file not found")

type Watcher struct {
	path         string
	onChange     func(*Config)
	onError      func(error)
	pollInterval time.Duration
	stopCh       chan struct{}
	lastMod      time.Time
	mu           sync.RWMutex
	running      bool
}

func NewWatcher(path string) (*Watcher, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("stat config: %w", err)
	}

	return &Watcher{
		path:         absPath,
		pollInterval: 5 * time.Second,
		stopCh:       make(chan struct{}),
	}, nil
}

func (w *Watcher) OnChange(fn func(*Config)) *Watcher {
	w.onChange = fn
	return w
}

func (w *Watcher) OnError(fn func(error)) *Watcher {
	w.onError = fn
	return w
}

func (w *Watcher) SetPollInterval(d time.Duration) *Watcher {
	w.pollInterval = d
	return w
}

func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return errors.New("watcher already running")
	}
	w.running = true
	w.mu.Unlock()

	if err := w.loadAndNotify(); err != nil {
		return err
	}

	go w.watchLoop()

	return nil
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
}

func (w *Watcher) watchLoop() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.checkAndReload(); err != nil {
				if w.onError != nil {
					w.onError(err)
				}
			}
		}
	}
}

func (w *Watcher) checkAndReload() error {
	info, err := os.Stat(w.path)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	modTime := info.ModTime()
	if !modTime.After(w.lastMod) {
		return nil
	}

	w.lastMod = modTime
	return w.loadAndNotify()
}

func (w *Watcher) loadAndNotify() error {
	cfg, err := Load(w.path)
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}

	if w.onChange != nil {
		w.onChange(cfg)
	}

	return nil
}

func (w *Watcher) Reload() error {
	return w.loadAndNotify()
}

type ConfigManager struct {
	watchers  map[string]*Watcher
	onChanges map[string]func(*Config)
	mu        sync.RWMutex
	basePath  string
}

func NewConfigManager(basePath string) *ConfigManager {
	if basePath == "" {
		basePath = "config"
	}
	return &ConfigManager{
		watchers:  make(map[string]*Watcher),
		onChanges: make(map[string]func(*Config)),
		basePath:  basePath,
	}
}

func (cm *ConfigManager) Watch(name string, path string, onChange func(*Config)) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, ok := cm.watchers[name]; ok {
		return fmt.Errorf("watcher %s already exists", name)
	}

	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(cm.basePath, path)
	}

	w, err := NewWatcher(fullPath)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	w.OnChange(onChange)
	w.OnError(func(err error) {
		log.Printf("config watcher error for %s: %v", name, err)
	})

	cm.watchers[name] = w
	cm.onChanges[name] = onChange

	return w.Start()
}

func (cm *ConfigManager) Unwatch(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if w, ok := cm.watchers[name]; ok {
		w.Stop()
		delete(cm.watchers, name)
		delete(cm.onChanges, name)
	}
}

func (cm *ConfigManager) Get(name string) (*Config, error) {
	cm.mu.RLock()
	w, ok := cm.watchers[name]
	cm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("watcher %s not found", name)
	}

	return Load(w.path)
}

func (cm *ConfigManager) Reload(name string) error {
	cm.mu.RLock()
	w, ok := cm.watchers[name]
	cm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("watcher %s not found", name)
	}

	return w.Reload()
}

func (cm *ConfigManager) ReloadAll() error {
	cm.mu.RLock()
	watchers := make(map[string]*Watcher)
	for name, w := range cm.watchers {
		watchers[name] = w
	}
	cm.mu.RUnlock()

	var errs []error
	for name, w := range watchers {
		if err := w.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("reload errors: %v", errs)
	}

	return nil
}

func (cm *ConfigManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, w := range cm.watchers {
		w.Stop()
	}
	cm.watchers = make(map[string]*Watcher)
}

type JSONConfig struct {
	data map[string]any
	mu   sync.RWMutex
	file string
}

func LoadJSONConfig(path string) (*JSONConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return &JSONConfig{
		data: m,
		file: path,
	}, nil
}

func (jc *JSONConfig) Get(key string) (any, bool) {
	jc.mu.RLock()
	defer jc.mu.RUnlock()

	val, ok := jc.data[key]
	return val, ok
}

func (jc *JSONConfig) GetString(key string) (string, bool) {
	val, ok := jc.Get(key)
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

func (jc *JSONConfig) GetInt(key string) (int, bool) {
	val, ok := jc.Get(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	}
	return 0, false
}

func (jc *JSONConfig) GetBool(key string) (bool, bool) {
	val, ok := jc.Get(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

func (jc *JSONConfig) GetMap(key string) (map[string]any, bool) {
	val, ok := jc.Get(key)
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]any)
	return m, ok
}

func (jc *JSONConfig) Set(key string, value any) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	jc.data[key] = value
}

func (jc *JSONConfig) Save() error {
	jc.mu.RLock()
	defer jc.mu.RUnlock()

	data, err := json.MarshalIndent(jc.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(jc.file, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (jc *JSONConfig) Reload() error {
	jc.mu.Lock()
	defer jc.mu.Unlock()

	data, err := os.ReadFile(jc.file)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	jc.data = m
	return nil
}
