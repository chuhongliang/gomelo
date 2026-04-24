package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/chuhongliang/gomelo/loader"

	"github.com/robfig/cron/v3"
)

type CronTask struct {
	ID       int    `json:"id"`
	Time     string `json:"time"`
	Action   string `json:"action"`
	ServerID string `json:"serverId,omitempty"`
}

type ServerCronConfig map[string][]CronTask

type CronConfigs map[string]ServerCronConfig

type CronScheduler struct {
	serverType  string
	serverID    string
	cron        *cron.Cron
	taskEntries map[int]cron.EntryID
	tasks       []CronTask
	ldr         *loader.Loader
	mu          sync.RWMutex
	methods     map[string]*CronMethodInfo
	initialized bool
	stopCtx     context.Context
	stopCancel  context.CancelFunc
}

type CronMethodInfo struct {
	Method reflect.Method
	Cron   any
}

func NewCronScheduler(serverType, serverID string, ldr *loader.Loader) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &CronScheduler{
		serverType:  serverType,
		serverID:    serverID,
		ldr:         ldr,
		cron:        cron.New(cron.WithSeconds()),
		taskEntries: make(map[int]cron.EntryID),
		tasks:       make([]CronTask, 0),
		methods:     make(map[string]*CronMethodInfo),
		stopCtx:     ctx,
		stopCancel:  cancel,
	}
}

func (cs *CronScheduler) LoadConfig(env string, configs CronConfigs) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	envConfig, ok := configs[env]
	if !ok {
		return fmt.Errorf("env %s not found in cron config", env)
	}

	serverTasks, ok := envConfig[cs.serverType]
	if !ok {
		return nil
	}

	for _, task := range serverTasks {
		if task.ServerID != "" && task.ServerID != cs.serverID {
			continue
		}
		cs.tasks = append(cs.tasks, task)
	}

	return nil
}

func (cs *CronScheduler) RegisterMethods() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.ldr == nil {
		return
	}

	crons := cs.ldr.GetAllCrons()
	serverCrons, ok := crons[cs.serverType]
	if !ok {
		return
	}

	for key, cm := range serverCrons {
		parts := splitKey(key)
		if len(parts) != 2 {
			continue
		}

		cs.methods[key] = &CronMethodInfo{
			Method: cm.Method,
			Cron:   cm.Cron,
		}
	}
}

func splitKey(key string) []string {
	return strings.Split(key, ".")
}

func (cs *CronScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.initialized {
		return nil
	}

	cs.cron.Start()

	for _, task := range cs.tasks {
		info, ok := cs.methods[task.Action]
		if !ok {
			log.Printf("Cron action not found: %s.%s", cs.serverType, task.Action)
			continue
		}

		spec := convertToCronV3Spec(task.Time)
		if !isValidCronSpec(spec) {
			log.Printf("Invalid cron spec for %s [%d]: %s", task.Action, task.ID, spec)
			continue
		}

		method := info.Method
		cron := info.Cron
		taskID := task.ID

		entryID, err := cs.cron.AddFunc(spec, func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("cron panic recovered: %v", r)
				}
			}()

			select {
			case <-cs.stopCtx.Done():
				return
			default:
			}
			args := []reflect.Value{
				reflect.ValueOf(cron),
				reflect.ValueOf(cs.stopCtx),
			}
			method.Func.Call(args)
		})
		if err != nil {
			log.Printf("Failed to add cron %s [%d]: %v", task.Action, task.ID, err)
			continue
		}

		cs.taskEntries[taskID] = entryID
		log.Printf("Registered cron: %s [%d] (%s)", task.Action, task.ID, spec)
	}

	cs.initialized = true
	return nil
}

func (cs *CronScheduler) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.stopCancel()

	if cs.cron != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx := cs.cron.Stop()
		select {
		case <-ctx.Done():
		case <-stopCtx.Done():
			log.Printf("cron stop timed out")
		}
	}
	cs.initialized = false
	cs.taskEntries = make(map[int]cron.EntryID)
}

func (cs *CronScheduler) Cancel(id int) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entryID, ok := cs.taskEntries[id]
	if !ok {
		return false
	}

	cs.cron.Remove(entryID)
	delete(cs.taskEntries, id)
	log.Printf("Cancelled cron task [%d]", id)
	return true
}

func (cs *CronScheduler) IsInitialized() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.initialized
}

func convertToCronV3Spec(spec string) string {
	parts := strings.Fields(spec)
	if len(parts) == 5 {
		return "0 " + spec
	}
	return spec
}

func isValidCronSpec(spec string) bool {
	parts := strings.Fields(spec)
	return len(parts) == 5 || len(parts) == 6
}

type CronManager struct {
	schedulers map[string]*CronScheduler
	mu         sync.RWMutex
	configPath string
	env        string
	ldr        *loader.Loader
}

func NewCronManager(configPath, env string, ldr *loader.Loader) *CronManager {
	return &CronManager{
		schedulers: make(map[string]*CronScheduler),
		configPath: configPath,
		env:        env,
		ldr:        ldr,
	}
}

func (cm *CronManager) StartAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.ldr.Load()

	configs, err := cm.loadConfigs()
	if err != nil {
		return fmt.Errorf("load cron configs: %w", err)
	}

	crons := cm.ldr.GetAllCrons()
	for serverType := range crons {
		scheduler := NewCronScheduler(serverType, cm.ldr.GetServerID(), cm.ldr)

		if err := scheduler.LoadConfig(cm.env, configs); err != nil {
			log.Printf("Warning: failed to load cron config for %s: %v", serverType, err)
			continue
		}

		scheduler.RegisterMethods()
		if err := scheduler.Start(); err != nil {
			log.Printf("Warning: failed to start cron scheduler for %s: %v", serverType, err)
			continue
		}

		cm.schedulers[serverType] = scheduler
	}

	return nil
}

func (cm *CronManager) loadConfigs() (CronConfigs, error) {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("read cron config: %w", err)
	}

	var configs CronConfigs
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("parse cron config: %w", err)
	}

	return configs, nil
}

func (cm *CronManager) StopAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, scheduler := range cm.schedulers {
		scheduler.Stop()
	}
	cm.schedulers = make(map[string]*CronScheduler)
}

func (cm *CronManager) GetScheduler(serverType string) *CronScheduler {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.schedulers[serverType]
}