package reload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/chuhongliang/gomelo/config"
)

type Reloader interface {
	Reload() error
	Start() error
	Stop()
}

type ConfigReloader struct {
	configPath string
	pollInterval time.Duration
	onReload    func(*config.Config) error

	current *config.Config
	mu      sync.RWMutex
	done    chan struct{}
}

func NewConfigReloader(configPath string, onReload func(*config.Config) error) (*ConfigReloader, error) {
	if configPath == "" {
		return nil, errors.New("config path is empty")
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	cfg, err := config.Load(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &ConfigReloader{
		configPath:  absPath,
		pollInterval: 5 * time.Second,
		onReload:    onReload,
		current:     cfg,
		done:       make(chan struct{}),
	}, nil
}

func (r *ConfigReloader) Get() *config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current
}

func (r *ConfigReloader) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	newCfg, err := config.Load(r.configPath)
	if err != nil {
		return fmt.Errorf("reload failed: %w", err)
	}

	if r.onReload != nil {
		if err := r.onReload(newCfg); err != nil {
			return fmt.Errorf("reload callback failed: %w", err)
		}
	}

	r.current = newCfg
	return nil
}

func (r *ConfigReloader) Start() error {
	go r.watchLoop()
	go r.signalHandler()
	return nil
}

func (r *ConfigReloader) Stop() {
	close(r.done)
}

func (r *ConfigReloader) watchLoop() {
	var lastModTime time.Time
	for {
		select {
		case <-r.done:
			return
		case <-time.After(r.pollInterval):
			info, err := os.Stat(r.configPath)
			if err != nil {
				continue
			}

			modTime := info.ModTime()
			if lastModTime.IsZero() {
				lastModTime = modTime
				continue
			}

			if modTime.After(lastModTime) {
				lastModTime = modTime
				if err := r.Reload(); err != nil {
					fmt.Printf("Config reload error: %v\n", err)
				} else {
					fmt.Printf("Config reloaded successfully\n")
				}
			}
		}
	}
}

func (r *ConfigReloader) signalHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGUSR1)

	for {
		select {
		case <-r.done:
			return
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP, syscall.SIGUSR1:
				if err := r.Reload(); err != nil {
					fmt.Printf("Signal reload error: %v\n", err)
				} else {
					fmt.Printf("Reloaded via signal: %v\n", sig)
				}
			}
		}
	}
}

type MultiReloader struct {
	reloaders []Reloader
	mu        sync.Mutex
}

func NewMultiReloader() *MultiReloader {
	return &MultiReloader{}
}

func (m *MultiReloader) Add(r Reloader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloaders = append(m.reloaders, r)
}

func (m *MultiReloader) ReloadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.reloaders {
		if err := r.Reload(); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiReloader) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.reloaders {
		if err := r.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiReloader) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.reloaders {
		r.Stop()
	}
}

type HandlerReloader struct {
	handlersPath string
	loader       func() error
	pollInterval time.Duration
	done         chan struct{}
}

func NewHandlerReloader(handlersPath string, loader func() error) *HandlerReloader {
	return &HandlerReloader{
		handlersPath: handlersPath,
		loader:       loader,
		pollInterval: 10 * time.Second,
		done:        make(chan struct{}),
	}
}

func (r *HandlerReloader) Reload() error {
	if r.loader == nil {
		return errors.New("no loader function set")
	}
	return r.loader()
}

func (r *HandlerReloader) Start() error {
	go r.watchLoop()
	return nil
}

func (r *HandlerReloader) Stop() {
	close(r.done)
}

func (r *HandlerReloader) watchLoop() {
	var lastHash int64
	for {
		select {
		case <-r.done:
			return
		case <-time.After(r.pollInterval):
			hash := r.scanHandlersDir()
			if hash != 0 && hash != lastHash {
				lastHash = hash
				if err := r.Reload(); err != nil {
					fmt.Printf("Handler reload error: %v\n", err)
				} else {
					fmt.Printf("Handlers reloaded\n")
				}
			}
		}
	}
}

func (r *HandlerReloader) scanHandlersDir() int64 {
	var hash int64
	filepath.Walk(r.handlersPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			hash += info.Size() + info.ModTime().UnixNano()
		}
		return nil
	})
	return hash
}

type RouteReloadRequest struct {
	ServerType string `json:"serverType"`
	Route      string `json:"route"`
	Handler    string `json:"handler"`
}

type RouteReloader struct {
	routes     map[string]*RouteReloadRequest
	mu         sync.RWMutex
	onRouteReload func(*RouteReloadRequest) error
	done       chan struct{}
}

func NewRouteReloader(onReload func(*RouteReloadRequest) error) *RouteReloader {
	return &RouteReloader{
		routes:       make(map[string]*RouteReloadRequest),
		onRouteReload: onReload,
		done:        make(chan struct{}),
	}
}

func (r *RouteReloader) AddRoute(req *RouteReloadRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := req.ServerType + "." + req.Route
	r.routes[key] = req
}

func (r *RouteReloader) ReloadRoute(serverType, route string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := serverType + "." + route
	req, ok := r.routes[key]
	if !ok {
		return fmt.Errorf("route not found: %s.%s", serverType, route)
	}

	if r.onRouteReload != nil {
		return r.onRouteReload(req)
	}
	return nil
}

func (r *RouteReloader) Start(ctx context.Context) error {
	go r.watchLoop(ctx)
	return nil
}

func (r *RouteReloader) Stop() {
	close(r.done)
}

func (r *RouteReloader) watchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.done:
			return
		}
	}
}

func LoadJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func SaveJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}