package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	Server    ServerConfig    `json:"server" yaml:"server"`
	RPC       RPCConfig       `json:"rpc" yaml:"rpc"`
	Registry  RegistryConfig  `json:"registry" yaml:"registry"`
	Scheduler SchedulerConfig `json:"scheduler" yaml:"scheduler"`
	Log       LogConfig       `json:"log" yaml:"log"`
	Cluster   ClusterConfig   `json:"cluster" yaml:"cluster"`
	Master    MasterConfig    `json:"master" yaml:"master"`
	Custom    map[string]any  `json:"custom" yaml:"custom"`
}

type MasterConfig struct {
	Development *MasterServerConfig `json:"development,omitempty" yaml:"development,omitempty"`
	Production  *MasterServerConfig `json:"production,omitempty" yaml:"production,omitempty"`
}

type MasterServerConfig struct {
	ID           string             `json:"id" yaml:"id"`
	Host         string             `json:"host" yaml:"host"`
	Port         int                `json:"port" yaml:"port"`
	Servers      *ServerStartConfig `json:"servers,omitempty" yaml:"servers,omitempty"`
	AutoStart    bool               `json:"autoStart" yaml:"autoStart"`
	RestartDelay int                `json:"restartDelay" yaml:"restartDelay"`
}

type ServerStartConfig struct {
	Development map[string]ServerTypeConfig `json:"development,omitempty" yaml:"development,omitempty"`
	Production  map[string]ServerTypeConfig `json:"production,omitempty" yaml:"production,omitempty"`
}

type ServerTypeConfig struct {
	Path      string   `json:"path" yaml:"path"`
	Args      []string `json:"args,omitempty" yaml:"args,omitempty"`
	Env       []string `json:"env,omitempty" yaml:"env,omitempty"`
	Instances int      `json:"instances" yaml:"instances"`
}

type ServerConfig struct {
	Host       string `json:"host" yaml:"host"`
	Port       int    `json:"port" yaml:"port"`
	Mode       string `json:"mode" yaml:"mode"`
	Env        string `json:"env" yaml:"env"`
	ServerID   string `json:"serverId" yaml:"serverId"`
	ServerType string `json:"serverType" yaml:"serverType"`
	Frontend   bool   `json:"frontend" yaml:"frontend"`
	Timeout    int    `json:"timeout" yaml:"timeout"`
	MaxConns   int    `json:"maxConns" yaml:"maxConns"`
}

type RPCConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	MaxConns    int    `json:"maxConns" yaml:"maxConns"`
	MinConns    int    `json:"minConns" yaml:"minConns"`
	Timeout     int    `json:"timeout" yaml:"timeout"`
	MaxWaitTime int    `json:"maxWaitTime" yaml:"maxWaitTime"`
}

type RegistryConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Type    string `json:"type" yaml:"type"`
	Host    string `json:"host" yaml:"host"`
	Port    int    `json:"port" yaml:"port"`
}

type SchedulerConfig struct {
	Enabled   bool `json:"enabled" yaml:"enabled"`
	Workers   int  `json:"workers" yaml:"workers"`
	QueueSize int  `json:"queueSize" yaml:"queueSize"`
}

type LogConfig struct {
	Level      string                    `json:"level" yaml:"level"`
	Path       string                    `json:"path" yaml:"path"`
	Console    bool                      `json:"console" yaml:"console"`
	Format     string                    `json:"format" yaml:"format"`
	Rotate     *RotateConfig             `json:"rotate,omitempty" yaml:"rotate,omitempty"`
	Categories map[string]CategoryConfig `json:"categories,omitempty" yaml:"categories,omitempty"`
}

type RotateConfig struct {
	Enabled  bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	MaxSize  int64 `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
	MaxFiles int   `json:"maxFiles,omitempty" yaml:"maxFiles,omitempty"`
	MaxAge   int   `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
}

type CategoryConfig struct {
	Level string `json:"level" yaml:"level"`
}

type ClusterConfig struct {
	MasterHost string         `json:"masterHost" yaml:"masterHost"`
	MasterPort int            `json:"masterPort" yaml:"masterPort"`
	Servers    []ServerConfig `json:"servers" yaml:"servers"`
}

type Loader struct {
	mu       sync.RWMutex
	config   *Config
	path     string
	watchers []func(*Config)
	done     chan struct{}
	watching bool
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	cfg := &Config{}
	cfg.Custom = make(map[string]any)

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}

	return cfg, nil
}

func LoadStrict(path string) (*Loader, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	l := &Loader{
		config: cfg,
		path:   path,
		done:   make(chan struct{}),
	}

	return l, nil
}

func (l *Loader) Watch(callback func(*Config)) {
	l.mu.Lock()
	l.watchers = append(l.watchers, callback)
	if l.watching {
		l.mu.Unlock()
		return
	}
	l.watching = true
	l.mu.Unlock()

	go l.watchLoop()
}

func (l *Loader) watchLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastMod int64

	if info, err := os.Stat(l.path); err == nil {
		lastMod = info.ModTime().UnixMilli()
	}

	for {
		select {
		case <-l.done:
			return
		case <-ticker.C:
			info, err := os.Stat(l.path)
			if err != nil {
				continue
			}

			mod := info.ModTime().UnixMilli()
			if mod > lastMod {
				lastMod = mod

				if newCfg, err := Load(l.path); err == nil {
					l.mu.Lock()
					l.config = newCfg
					watchers := make([]func(*Config), len(l.watchers))
					copy(watchers, l.watchers)
					l.mu.Unlock()

					for _, cb := range watchers {
						go cb(newCfg)
					}
				}
			}
		}
	}
}

func (l *Loader) Get() *Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

func (l *Loader) Set(cfg *Config) {
	l.mu.Lock()
	l.config = cfg
	l.mu.Unlock()
}

func (l *Loader) Stop() {
	close(l.done)
}

func Merge(base, overlay *Config) *Config {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	result := *base

	if overlay.Server.Host != "" {
		result.Server.Host = overlay.Server.Host
	}
	if overlay.Server.Port != 0 {
		result.Server.Port = overlay.Server.Port
	}
	if overlay.Server.Env != "" {
		result.Server.Env = overlay.Server.Env
	}
	if overlay.Server.Mode != "" {
		result.Server.Mode = overlay.Server.Mode
	}
	if overlay.Server.ServerID != "" {
		result.Server.ServerID = overlay.Server.ServerID
	}
	if overlay.Server.ServerType != "" {
		result.Server.ServerType = overlay.Server.ServerType
	}
	if overlay.Server.Frontend {
		result.Server.Frontend = overlay.Server.Frontend
	}
	if overlay.Server.Timeout != 0 {
		result.Server.Timeout = overlay.Server.Timeout
	}
	if overlay.Server.MaxConns != 0 {
		result.Server.MaxConns = overlay.Server.MaxConns
	}

	if overlay.RPC.Enabled {
		result.RPC.Enabled = overlay.RPC.Enabled
	}
	if overlay.RPC.Host != "" {
		result.RPC.Host = overlay.RPC.Host
	}
	if overlay.RPC.Port != 0 {
		result.RPC.Port = overlay.RPC.Port
	}
	if overlay.RPC.MaxConns != 0 {
		result.RPC.MaxConns = overlay.RPC.MaxConns
	}
	if overlay.RPC.MinConns != 0 {
		result.RPC.MinConns = overlay.RPC.MinConns
	}
	if overlay.RPC.Timeout != 0 {
		result.RPC.Timeout = overlay.RPC.Timeout
	}
	if overlay.RPC.MaxWaitTime != 0 {
		result.RPC.MaxWaitTime = overlay.RPC.MaxWaitTime
	}

	if overlay.Registry.Enabled {
		result.Registry.Enabled = overlay.Registry.Enabled
	}
	if overlay.Registry.Type != "" {
		result.Registry.Type = overlay.Registry.Type
	}
	if overlay.Registry.Host != "" {
		result.Registry.Host = overlay.Registry.Host
	}
	if overlay.Registry.Port != 0 {
		result.Registry.Port = overlay.Registry.Port
	}

	if overlay.Scheduler.Enabled {
		result.Scheduler.Enabled = overlay.Scheduler.Enabled
	}
	if overlay.Scheduler.Workers != 0 {
		result.Scheduler.Workers = overlay.Scheduler.Workers
	}
	if overlay.Scheduler.QueueSize != 0 {
		result.Scheduler.QueueSize = overlay.Scheduler.QueueSize
	}

	if overlay.Log.Level != "" {
		result.Log.Level = overlay.Log.Level
	}
	if overlay.Log.Path != "" {
		result.Log.Path = overlay.Log.Path
	}
	if overlay.Log.Rotate != nil && overlay.Log.Rotate.MaxSize != 0 {
		if result.Log.Rotate == nil {
			result.Log.Rotate = &RotateConfig{}
		}
		result.Log.Rotate.MaxSize = overlay.Log.Rotate.MaxSize
	}
	if overlay.Log.Rotate != nil && overlay.Log.Rotate.MaxFiles != 0 {
		if result.Log.Rotate == nil {
			result.Log.Rotate = &RotateConfig{}
		}
		result.Log.Rotate.MaxFiles = overlay.Log.Rotate.MaxFiles
	}
	if overlay.Log.Console {
		result.Log.Console = overlay.Log.Console
	}
	if overlay.Log.Format != "" {
		result.Log.Format = overlay.Log.Format
	}

	if overlay.Cluster.MasterHost != "" {
		result.Cluster.MasterHost = overlay.Cluster.MasterHost
	}
	if overlay.Cluster.MasterPort != 0 {
		result.Cluster.MasterPort = overlay.Cluster.MasterPort
	}
	if len(overlay.Cluster.Servers) > 0 {
		result.Cluster.Servers = overlay.Cluster.Servers
	}

	if result.Custom == nil {
		result.Custom = make(map[string]any)
	}
	for k, v := range overlay.Custom {
		result.Custom[k] = v
	}

	return &result
}

func LoadWithEnv(basePath, env string) (*Config, error) {
	base, err := Load(basePath)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(basePath)
	name := filepath.Base(basePath)
	ext := filepath.Ext(name)
	prefix := name[:len(name)-len(ext)]

	envPath := filepath.Join(dir, fmt.Sprintf("%s.%s%s", prefix, env, ext))

	if _, err := os.Stat(envPath); err == nil {
		envCfg, err := Load(envPath)
		if err != nil {
			return nil, err
		}
		return Merge(base, envCfg), nil
	}

	return base, nil
}

func (c *Config) GetString(key string) string {
	switch key {
	case "server.host":
		return c.Server.Host
	case "server.port":
		return fmt.Sprintf("%d", c.Server.Port)
	case "server.mode":
		return c.Server.Mode
	case "server.env":
		return c.Server.Env
	case "log.level":
		return c.Log.Level
	case "log.path":
		return c.Log.Path
	default:
		return ""
	}
}

func (c *Config) GetInt(key string) int {
	switch key {
	case "server.port":
		return c.Server.Port
	case "rpc.port":
		return c.RPC.Port
	case "registry.port":
		return c.Registry.Port
	default:
		return 0
	}
}

func (c *Config) GetBool(key string) bool {
	switch key {
	case "server.frontend":
		return c.Server.Frontend
	case "rpc.enabled":
		return c.RPC.Enabled
	case "registry.enabled":
		return c.Registry.Enabled
	default:
		return false
	}
}

var defaultConfig = &Config{
	Server: ServerConfig{
		Host:     "0.0.0.0",
		Port:     3010,
		Mode:     "standalone",
		Env:      "development",
		Timeout:  30,
		MaxConns: 10000,
	},
	RPC: RPCConfig{
		Enabled:  true,
		Port:     3030,
		MaxConns: 10,
		MinConns: 1,
		Timeout:  5,
	},
	Registry: RegistryConfig{
		Enabled: true,
		Type:    "local",
		Port:    3040,
	},
	Scheduler: SchedulerConfig{
		Enabled:   true,
		Workers:   4,
		QueueSize: 1024,
	},
	Log: LogConfig{
		Level:   "info",
		Path:    "./logs",
		Console: true,
		Format:  "json",
		Rotate: &RotateConfig{
			MaxSize:  100 * 1024 * 1024,
			MaxFiles: 10,
		},
	},
}

func Default() *Config {
	return defaultConfig
}

func SetDefault(cfg *Config) {
	defaultConfig = cfg
}
